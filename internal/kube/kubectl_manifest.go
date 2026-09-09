package kube

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

const kubectlManifestTimeout = 20 * time.Second

// ResourceManifest is read-only kubectl get -o yaml output for a single object.
type ResourceManifest struct {
	Command string `json:"command"`
	YAML    string `json:"yaml"`
	Error   string `json:"error,omitempty"`
}

// ResourceManifestRequest selects the object to fetch.
type ResourceManifestRequest struct {
	KubeconfigPath string
	Context        string
	ClusterVersion string
	ResourceID     string
	Kind           string
	Name           string
	Namespace      string
	ClusterScoped  bool
}

// KubectlResourceArg formats the resource type argument for kubectl get.
func KubectlResourceArg(group, resource string) string {
	resource = strings.TrimSpace(resource)
	group = strings.TrimSpace(group)
	if resource == "" {
		return ""
	}
	if group == "" {
		return resource
	}
	return resource + "." + group
}

// FormatKubectlGetCommand builds a copy-pasteable kubectl get command (no secrets).
func FormatKubectlGetCommand(req ResourceManifestRequest, resourceType string) string {
	parts := []string{"kubectl"}
	if ctx := strings.TrimSpace(req.Context); ctx != "" {
		parts = append(parts, "--context", shellQuote(ctx))
	}
	if ns := strings.TrimSpace(req.Namespace); ns != "" && !req.ClusterScoped {
		parts = append(parts, "-n", shellQuote(ns))
	}
	parts = append(parts, "get", resourceType, shellQuote(strings.TrimSpace(req.Name)), "-o", "yaml")
	return strings.Join(parts, " ")
}

// GetResourceManifest runs kubectl get -o yaml for the requested object.
func GetResourceManifest(ctx context.Context, req ResourceManifestRequest) (ResourceManifest, error) {
	empty := ResourceManifest{}
	if strings.TrimSpace(req.Name) == "" {
		return empty, fmt.Errorf("resource name is required")
	}

	kubectl, _ := ResolveKubectlForCluster(strings.TrimSpace(req.ClusterVersion))
	if kubectl == "" {
		return empty, fmt.Errorf("kubectl is not available")
	}

	manifestReq := req
	if storageName, storageErr := helmReleaseManifestName(ctx, req); storageErr == nil && storageName != "" {
		manifestReq.Name = storageName
	}

	group, resource, clusterScoped, err := resolveManifestResource(ctx, req)
	if err != nil {
		return empty, err
	}
	resourceType := KubectlResourceArg(group, resource)
	if resourceType == "" {
		return empty, fmt.Errorf("could not resolve resource type for %q", req.Kind)
	}

	manifestReq.ClusterScoped = manifestReq.ClusterScoped || clusterScoped
	command := FormatKubectlGetCommand(manifestReq, resourceType)

	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, kubectlManifestTimeout)
	defer cancel()

	args := kubectlManifestArgs(manifestReq, resourceType)
	cmd := exec.CommandContext(reqCtx, kubectl, args...)
	cmd.Env = os.Environ()
	if kcfg := strings.TrimSpace(req.KubeconfigPath); kcfg != "" &&
		!strings.Contains(kcfg, string(os.PathListSeparator)) {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kcfg)
	}

	out, runErr := cmd.CombinedOutput()
	yaml := strings.TrimSpace(string(out))
	result := ResourceManifest{
		Command: command,
		YAML:    yaml,
	}
	if runErr != nil {
		msg := strings.TrimSpace(yaml)
		if msg == "" {
			msg = runErr.Error()
		}
		result.Error = msg
		return result, nil
	}
	return result, nil
}

func kubectlManifestArgs(req ResourceManifestRequest, resourceType string) []string {
	args := []string{"get", resourceType, strings.TrimSpace(req.Name), "-o", "yaml"}
	if ctxName := strings.TrimSpace(req.Context); ctxName != "" {
		args = append([]string{"--context", ctxName}, args...)
	}
	if kcfg := strings.TrimSpace(req.KubeconfigPath); kcfg != "" &&
		!strings.Contains(kcfg, string(os.PathListSeparator)) {
		args = append([]string{"--kubeconfig", kcfg}, args...)
	}
	if ns := strings.TrimSpace(req.Namespace); ns != "" && !req.ClusterScoped {
		args = append(args, "-n", ns)
	}
	return args
}

func resolveManifestResource(ctx context.Context, req ResourceManifestRequest) (group, resource string, clusterScoped bool, err error) {
	if id := strings.TrimSpace(req.ResourceID); id != "" {
		switch id {
		case VirtualHelmReleasesResourceID:
			return resolveHelmReleaseManifestResource(ctx, req)
		default:
			if IsVirtualCatalogResource(id) {
				return "", "", false, fmt.Errorf("virtual resource %q has no kubectl manifest", id)
			}
		}
		g, _, r, parseErr := ParseResourceID(id)
		if parseErr != nil {
			return "", "", false, parseErr
		}
		return g, r, req.ClusterScoped, nil
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "HelmRelease" {
		return resolveHelmReleaseManifestResource(ctx, req)
	}
	if kind == "" {
		return "", "", false, fmt.Errorf("kind or resourceId is required")
	}

	client, err := NewFromFlags(req.KubeconfigPath, req.Context, req.Namespace)
	if err != nil {
		return "", "", false, err
	}
	if client == nil || client.Clientset == nil {
		return "", "", false, fmt.Errorf("kubernetes client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	g, r, namespaced, resolveErr := resolveResourceTypeForKind(ctx, client, kind)
	if resolveErr != nil {
		return "", "", false, resolveErr
	}
	return g, r, !namespaced || req.ClusterScoped, nil
}

func helmReleaseManifestName(ctx context.Context, req ResourceManifestRequest) (string, error) {
	kind := strings.TrimSpace(req.Kind)
	if kind != "HelmRelease" && strings.TrimSpace(req.ResourceID) != VirtualHelmReleasesResourceID {
		return "", nil
	}
	client, err := NewFromFlags(req.KubeconfigPath, req.Context, req.Namespace)
	if err != nil {
		return "", err
	}
	if client == nil || client.Clientset == nil {
		return "", fmt.Errorf("kubernetes client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rec, err := GetHelmRelease(ctx, client, req.Namespace, req.Name)
	if err != nil {
		return "", err
	}
	return rec.StorageName, nil
}

func resolveHelmReleaseManifestResource(ctx context.Context, req ResourceManifestRequest) (group, resource string, clusterScoped bool, err error) {
	client, err := NewFromFlags(req.KubeconfigPath, req.Context, req.Namespace)
	if err != nil {
		return "", "", false, err
	}
	if client == nil || client.Clientset == nil {
		return "", "", false, fmt.Errorf("kubernetes client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rec, err := GetHelmRelease(ctx, client, req.Namespace, req.Name)
	if err != nil {
		return "", "", false, err
	}
	switch rec.StorageKind {
	case "ConfigMap":
		return "", "configmaps", false, nil
	default:
		return "", "secrets", false, nil
	}
}

type kindResourceMatch struct {
	group      string
	resource   string
	namespaced bool
}

func resolveResourceTypeForKind(ctx context.Context, client *Client, kind string) (group, resource string, namespaced bool, err error) {
	lists, err := client.Clientset.Discovery().ServerPreferredResources()
	if err != nil {
		return "", "", false, err
	}
	var matches []kindResourceMatch
	for _, list := range lists {
		if list == nil {
			continue
		}
		gv, parseErr := schema.ParseGroupVersion(list.GroupVersion)
		if parseErr != nil {
			continue
		}
		for _, r := range list.APIResources {
			if r.Kind != kind || isSubresource(r.Name) {
				continue
			}
			matches = append(matches, kindResourceMatch{
				group:      gv.Group,
				resource:   r.Name,
				namespaced: r.Namespaced,
			})
		}
	}
	if len(matches) == 0 {
		return "", "", false, fmt.Errorf("no API resource found for kind %q", kind)
	}
	if len(matches) > 1 {
		if picked, ok := pickPreferredKindMatch(matches); ok {
			return picked.group, picked.resource, picked.namespaced, nil
		}
	}
	return matches[0].group, matches[0].resource, matches[0].namespaced, nil
}

func pickPreferredKindMatch(matches []kindResourceMatch) (kindResourceMatch, bool) {
	// Prefer stable non-deprecated groups when multiple kinds share a name (e.g. Policy).
	preferredGroups := []string{
		"apps",
		"batch",
		"networking.k8s.io",
		"rbac.authorization.k8s.io",
		"kyverno.io",
		"policies.kyverno.io",
	}
	for _, want := range preferredGroups {
		for _, m := range matches {
			if m.group == want {
				return m, true
			}
		}
	}
	return kindResourceMatch{}, false
}

func shellQuote(s string) string {
	if s == "" {
		return `''`
	}
	if strings.ContainsAny(s, " \t\"'$\\") {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	return s
}
