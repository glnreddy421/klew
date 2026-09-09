package kube

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
)

const (
	helmOwnerLabel           = "owner"
	helmOwnerValue           = "helm"
	helmReleaseNameLabel     = "name"
	helmReleaseVersionLabel  = "version"
	helmReleaseSecretType    = "helm.sh/release.v1"
	helmReleaseDataKey       = "release"
	helmReleaseNamePrefix    = "sh.helm.release.v1."
	helmMetaReleaseName      = "meta.helm.sh/release-name"
	helmMetaReleaseNamespace = "meta.helm.sh/release-namespace"
)

// HelmReleaseRecord is a decoded Helm 3 release stored in a Secret or ConfigMap.
type HelmReleaseRecord struct {
	Name          string
	Namespace     string
	Revision      int
	Status        string
	Chart         string
	ChartVersion  string
	AppVersion    string
	Updated       time.Time
	Description   string
	Notes         string
	StorageKind   string
	StorageName   string
	StorageUID    string
	ResourceVer   string
	FirstDeployed time.Time
	ConfigYAML    string
	UserValuesYAML string
}

type helmReleasePayload struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Version   int                    `json:"version"`
	Config    map[string]interface{} `json:"config"`
	Info      struct {
		Status          string    `json:"status"`
		Description     string    `json:"description"`
		Notes           string    `json:"notes"`
		FirstDeployed   time.Time `json:"first_deployed"`
		LastDeployed    time.Time `json:"last_deployed"`
		Deleted         time.Time `json:"deleted"`
	} `json:"info"`
	Chart struct {
		Metadata struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			AppVersion string `json:"appVersion"`
		} `json:"metadata"`
		Values map[string]interface{} `json:"values"`
	} `json:"chart"`
}

type helmStorageObject struct {
	kind      string
	namespace string
	name      string
	uid       string
	rv        string
	created   time.Time
	labels    map[string]string
	data      []byte
}

// ListHelmReleaseEntities lists Helm releases from cluster storage (Secrets / ConfigMaps).
func ListHelmReleaseEntities(ctx context.Context, client *Client, scope CatalogEntityScope) (model.CatalogEntityList, error) {
	records, access, errMsg, err := collectHelmReleases(ctx, client, scope)
	if err != nil {
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}, err
	}
	if access == model.ResourceAccessForbidden {
		return model.CatalogEntityList{AccessState: model.ResourceAccessForbidden}, nil
	}
	if access == model.ResourceAccessError && errMsg != "" {
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: errMsg}, nil
	}
	if access == model.ResourceAccessUnavailable {
		return model.CatalogEntityList{AccessState: model.ResourceAccessUnavailable}, nil
	}

	entities := make([]model.CatalogEntity, 0, len(records))
	for _, rec := range records {
		entities = append(entities, helmReleaseToCatalogEntity(rec))
	}
	sort.Slice(entities, func(i, j int) bool {
		a, b := entities[i], entities[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	return model.CatalogEntityList{
		Entities:    entities,
		AccessState: model.ResourceAccessAllowed,
	}, nil
}

// GetHelmRelease loads the latest revision of a Helm release by name and namespace.
func GetHelmRelease(ctx context.Context, client *Client, namespace, name string) (*HelmReleaseRecord, error) {
	if client == nil || client.Clientset == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	ns := strings.TrimSpace(namespace)
	releaseName := strings.TrimSpace(name)
	if ns == "" || releaseName == "" {
		return nil, fmt.Errorf("namespace and release name are required")
	}
	records, _, _, err := collectHelmReleases(ctx, client, CatalogEntityScope{Namespace: ns})
	if err != nil {
		return nil, err
	}
	for _, rec := range records {
		if rec.Name == releaseName && rec.Namespace == ns {
			return &rec, nil
		}
	}
	return nil, fmt.Errorf("helm release %q not found in namespace %q", releaseName, ns)
}

func collectHelmReleases(ctx context.Context, client *Client, scope CatalogEntityScope) ([]HelmReleaseRecord, model.ResourceAccessState, string, error) {
	if client == nil || client.Clientset == nil {
		return nil, model.ResourceAccessUnknown, "", fmt.Errorf("kubernetes client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	objects, access, errMsg, err := listHelmStorageObjects(ctx, client, scope)
	if err != nil {
		return nil, model.ResourceAccessError, errMsg, err
	}
	if access != model.ResourceAccessAllowed {
		return nil, access, errMsg, nil
	}

	latest := map[string]HelmReleaseRecord{}
	for _, obj := range objects {
		rec, parseErr := helmRecordFromStorage(obj)
		if parseErr != nil {
			continue
		}
		if scope.Namespace != "" && rec.Namespace != scope.Namespace {
			continue
		}
		if len(scope.Namespaces) > 0 && !containsString(scope.Namespaces, rec.Namespace) {
			continue
		}
		key := rec.Namespace + "/" + rec.Name
		prev, ok := latest[key]
		if !ok || rec.Revision > prev.Revision {
			latest[key] = rec
		}
	}

	out := make([]HelmReleaseRecord, 0, len(latest))
	for _, rec := range latest {
		out = append(out, rec)
	}
	return out, model.ResourceAccessAllowed, "", nil
}

func listHelmStorageObjects(ctx context.Context, client *Client, scope CatalogEntityScope) ([]helmStorageObject, model.ResourceAccessState, string, error) {
	labelSelector := helmOwnerLabel + "=" + helmOwnerValue
	var out []helmStorageObject
	allowed := 0
	forbidden := 0
	var lastErr string

	appendSecrets := func(list *corev1.SecretList, err error) {
		if err != nil {
			if apierrors.IsForbidden(err) {
				forbidden++
				return
			}
			lastErr = err.Error()
			return
		}
		allowed++
		for i := range list.Items {
			sec := &list.Items[i]
			if !isHelmReleaseSecret(sec) {
				continue
			}
			raw := sec.Data[helmReleaseDataKey]
			if len(raw) == 0 {
				continue
			}
			out = append(out, helmStorageObject{
				kind:      "Secret",
				namespace: sec.Namespace,
				name:      sec.Name,
				uid:       string(sec.UID),
				rv:        sec.ResourceVersion,
				created:   sec.CreationTimestamp.Time,
				labels:    sec.Labels,
				data:      raw,
			})
		}
	}

	appendConfigMaps := func(list *corev1.ConfigMapList, err error) {
		if err != nil {
			if apierrors.IsForbidden(err) {
				forbidden++
				return
			}
			lastErr = err.Error()
			return
		}
		allowed++
		for i := range list.Items {
			cm := &list.Items[i]
			if !isHelmReleaseConfigMap(cm) {
				continue
			}
			raw := []byte(cm.BinaryData[helmReleaseDataKey])
			if len(raw) == 0 {
				raw = []byte(cm.Data[helmReleaseDataKey])
			}
			if len(raw) == 0 {
				continue
			}
			out = append(out, helmStorageObject{
				kind:      "ConfigMap",
				namespace: cm.Namespace,
				name:      cm.Name,
				uid:       string(cm.UID),
				rv:        cm.ResourceVersion,
				created:   cm.CreationTimestamp.Time,
				labels:    cm.Labels,
				data:      raw,
			})
		}
	}

	listInNamespace := func(ns string) {
		secList, secErr := client.Clientset.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		appendSecrets(secList, secErr)

		cmList, cmErr := client.Clientset.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		appendConfigMaps(cmList, cmErr)
	}

	switch {
	case scope.ClusterScoped, scope.AllNamespaces:
		secList, secErr := client.Clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		appendSecrets(secList, secErr)

		cmList, cmErr := client.Clientset.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		appendConfigMaps(cmList, cmErr)
	case len(scope.Namespaces) > 1:
		for _, ns := range scope.Namespaces {
			listInNamespace(ns)
		}
	default:
		ns := scope.Namespace
		listInNamespace(ns)
	}

	if allowed == 0 && forbidden > 0 {
		return nil, model.ResourceAccessForbidden, "", nil
	}
	if allowed == 0 && lastErr != "" {
		return nil, model.ResourceAccessError, lastErr, nil
	}
	if allowed == 0 {
		return nil, model.ResourceAccessUnavailable, "", nil
	}
	return out, model.ResourceAccessAllowed, "", nil
}

func isHelmReleaseSecret(sec *corev1.Secret) bool {
	if sec == nil {
		return false
	}
	if sec.Type == helmReleaseSecretType {
		return true
	}
	if sec.Labels != nil && sec.Labels[helmOwnerLabel] == helmOwnerValue {
		return strings.HasPrefix(sec.Name, helmReleaseNamePrefix) || sec.Data[helmReleaseDataKey] != nil
	}
	return false
}

func isHelmReleaseConfigMap(cm *corev1.ConfigMap) bool {
	if cm == nil {
		return false
	}
	if cm.Labels != nil && cm.Labels[helmOwnerLabel] == helmOwnerValue {
		return cm.Data[helmReleaseDataKey] != "" || cm.BinaryData[helmReleaseDataKey] != nil
	}
	return strings.HasPrefix(cm.Name, helmReleaseNamePrefix)
}

func helmRecordFromStorage(obj helmStorageObject) (HelmReleaseRecord, error) {
	payload, err := decodeHelmReleasePayload(obj.data)
	if err != nil {
		return HelmReleaseRecord{}, err
	}

	rec := HelmReleaseRecord{
		Name:          strings.TrimSpace(payload.Name),
		Namespace:     strings.TrimSpace(payload.Namespace),
		Revision:      payload.Version,
		Status:        strings.TrimSpace(payload.Info.Status),
		Chart:         strings.TrimSpace(payload.Chart.Metadata.Name),
		ChartVersion:  strings.TrimSpace(payload.Chart.Metadata.Version),
		AppVersion:    strings.TrimSpace(payload.Chart.Metadata.AppVersion),
		Description:   strings.TrimSpace(payload.Info.Description),
		Notes:         strings.TrimSpace(payload.Info.Notes),
		StorageKind:   obj.kind,
		StorageName:   obj.name,
		StorageUID:    obj.uid,
		ResourceVer:   obj.rv,
		FirstDeployed: payload.Info.FirstDeployed,
	}
	if rec.Namespace == "" {
		rec.Namespace = obj.namespace
	}
	if rec.Name == "" {
		rec.Name = helmReleaseNameFromLabels(obj.labels, obj.name)
	}
	if rec.Revision == 0 {
		rec.Revision = helmRevisionFromLabels(obj.labels, obj.name)
	}
	if !payload.Info.LastDeployed.IsZero() {
		rec.Updated = payload.Info.LastDeployed
	} else if !obj.created.IsZero() {
		rec.Updated = obj.created
	}
	if rec.Status == "" {
		rec.Status = "unknown"
	}
	rec.ConfigYAML = marshalHelmValuesYAML(payload.Config)
	userValues := helmUserSuppliedValues(payload.Config, payload.Chart.Values)
	rec.UserValuesYAML = marshalHelmValuesYAML(userValues)
	if strings.TrimSpace(rec.UserValuesYAML) == "# (empty)\n" {
		rec.UserValuesYAML = "# No user-supplied overrides\n"
	}
	return rec, nil
}

func helmReleaseToCatalogEntity(rec HelmReleaseRecord) model.CatalogEntity {
	updated := ""
	if !rec.Updated.IsZero() {
		updated = rec.Updated.UTC().Format(time.RFC3339)
	}
	firstDeployed := ""
	if !rec.FirstDeployed.IsZero() {
		firstDeployed = rec.FirstDeployed.UTC().Format(time.RFC3339)
	}
	return model.CatalogEntity{
		ResourceID:        VirtualHelmReleasesResourceID,
		Name:              rec.Name,
		Namespace:         rec.Namespace,
		UID:               rec.StorageUID,
		ResourceVersion:   rec.ResourceVer,
		Kind:              "HelmRelease",
		APIVersion:        "klew/virtual/v1",
		CreationTimestamp: firstDeployed,
		StatusHint:        rec.Status,
		HelmStorageKind:   rec.StorageKind,
		HelmStorageName:   rec.StorageName,
		TableFields: map[string]string{
			"chart":        rec.Chart,
			"chartVersion": rec.ChartVersion,
			"appVersion":   rec.AppVersion,
			"revision":     strconv.Itoa(rec.Revision),
			"status":       rec.Status,
			"updated":      updated,
			"storage":      rec.StorageKind,
			"description":  rec.Description,
		},
	}
}

func helmReleaseNameFromLabels(labels map[string]string, objectName string) string {
	if labels != nil {
		if name := strings.TrimSpace(labels[helmReleaseNameLabel]); name != "" {
			return name
		}
	}
	if strings.HasPrefix(objectName, helmReleaseNamePrefix) {
		rest := strings.TrimPrefix(objectName, helmReleaseNamePrefix)
		if idx := strings.LastIndex(rest, ".v"); idx > 0 {
			return rest[:idx]
		}
	}
	return objectName
}

func helmRevisionFromLabels(labels map[string]string, objectName string) int {
	if labels != nil {
		if v := strings.TrimSpace(labels[helmReleaseVersionLabel]); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}
	if strings.HasPrefix(objectName, helmReleaseNamePrefix) {
		rest := strings.TrimPrefix(objectName, helmReleaseNamePrefix)
		if idx := strings.LastIndex(rest, ".v"); idx >= 0 && idx+2 < len(rest) {
			if n, err := strconv.Atoi(rest[idx+2:]); err == nil {
				return n
			}
		}
	}
	return 0
}

func decodeHelmReleasePayload(raw []byte) (*helmReleasePayload, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty helm release payload")
	}

	decoded := raw
	if !looksLikeJSON(decoded) {
		b64, err := base64.StdEncoding.DecodeString(string(raw))
		if err != nil {
			return nil, fmt.Errorf("decode helm release base64: %w", err)
		}
		decoded = b64
	}
	if !looksLikeJSON(decoded) {
		gz, err := gzip.NewReader(bytes.NewReader(decoded))
		if err != nil {
			return nil, fmt.Errorf("decode helm release gzip: %w", err)
		}
		defer gz.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(gz); err != nil {
			return nil, fmt.Errorf("read helm release gzip: %w", err)
		}
		decoded = buf.Bytes()
	}

	var payload helmReleasePayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, fmt.Errorf("parse helm release json: %w", err)
	}
	return &payload, nil
}

func looksLikeJSON(b []byte) bool {
	trimmed := bytes.TrimSpace(b)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
