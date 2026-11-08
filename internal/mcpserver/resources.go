package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/glnreddy421/klew/internal/changes"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/redact"
)

// Plain list, get and log reads for questions that do not need a full
// investigation. Results are compact projections, never raw API objects:
// Secret values are never returned, and every free-text field goes through
// clean.

const (
	readTimeout = 20 * time.Second
	// maxManifestLen caps the YAML klew_get_resource returns, in runes.
	maxManifestLen = 24000
	maxAnnotations = 30
	maxDataKeys    = 50
	// maxLogFetchBytes caps what the kubelet sends for one klew_pod_logs call;
	// maxLogOutputLen caps the cleaned lines returned, in runes.
	maxLogFetchBytes = 512 * 1024
	maxLogOutputLen  = 40000
	lastApplied      = "kubectl.kubernetes.io/last-applied-configuration"
)

func (s *Server) registerResourceTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_list_resources",
		Description: "List objects of one resource type in a namespace (or cluster-wide for cluster-scoped types such as nodes), like kubectl get. " +
			"Accepts any type the cluster serves, including custom resources: kind (Deployment), plural (deployments), short name (deploy) or resource.group. " +
			"Each row has name, age, status, owner and the type's key columns (ready, restarts, node, replicas, ports, …); Secret rows show only type and key count. " +
			"Filter with label_selector and field_selector. For why something is failing use klew_investigate instead.",
		Annotations: readOnly("List resources", true),
		InputSchema: schemaFor[listResourcesIn](map[string]func(*jsonschema.Schema){
			"resource_type": pattern(`^[A-Za-z0-9.\-]+$`, 253),
			"limit":         intRange(1, 200, 50),
		}),
	}, s.listResources)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_get_resource",
		Description: "Get one object: summary row, labels, annotations, owners, conditions, and for Pods each container's state, restarts, image, limits and probes. " +
			"ConfigMaps include keys and values; Secrets include key names and sizes only, never values. " +
			"include_manifest adds the YAML without managedFields or last-applied-configuration, with Secret values and credential-named env values masked.",
		Annotations: readOnly("Get resource", true),
		InputSchema: schemaFor[getResourceIn](map[string]func(*jsonschema.Schema){
			"resource_type": pattern(`^[A-Za-z0-9.\-]+$`, 253),
			"name":          pattern(objectNamePattern, 253),
		}),
	}, s.getResource)

	// Raw log text is exactly what -no-log-samples keeps out of results.
	if s.opts.NoLogSamples {
		return
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_pod_logs",
		Description: "Read recent log lines of one pod container, oldest first, with timestamps. Does not follow. " +
			"previous reads the last terminated container, which is where a crash usually shows. " +
			"Lines are masked for common credential shapes (best effort) and capped in length. For log patterns across many pods use klew_investigate.",
		Annotations: readOnly("Pod logs", true),
		InputSchema: schemaFor[podLogsIn](map[string]func(*jsonschema.Schema){
			"pod":           pattern(objectNamePattern, 253),
			"tail_lines":    intRange(1, 1000, 100),
			"since_minutes": intRange(0, 10080, 0),
		}),
	}, s.podLogs)
}

// resolveType looks up typ in the cluster's discovered resources.
func (s *Server) resolveType(client *kube.Client, typ string) (kube.DiscoveredResource, error) {
	resources, err := s.discover(client)
	if err != nil {
		return kube.DiscoveredResource{}, fmt.Errorf("discover API resources: %w", err)
	}
	return kube.ResolveResourceType(resources, typ)
}

// scopeNamespace validates the namespace for r: required for namespaced types,
// ignored (with a note) for cluster-scoped ones.
func scopeNamespace(r kube.DiscoveredResource, in string) (ns, note string, err error) {
	if !r.Namespaced {
		if strings.TrimSpace(in) != "" {
			note = fmt.Sprintf("%s is cluster-scoped; namespace was ignored.", r.Kind)
		}
		return "", note, nil
	}
	if strings.TrimSpace(in) == "" {
		return "", "", fmt.Errorf("namespace is required for %s", r.Kind)
	}
	ns, err = parseNamespace(in)
	return ns, "", err
}

// readError turns API errors into messages an assistant can act on.
func readError(err error, verb string, r kube.DiscoveredResource, ns, name string) error {
	what := r.Resource
	if r.Group != "" {
		what += "." + r.Group
	}
	where := ""
	if ns != "" {
		where = " in " + ns
	}
	switch {
	case apierrors.IsForbidden(err):
		return fmt.Errorf("no permission to %s %s%s with this context: %s", verb, what, where, clean(err.Error()))
	case apierrors.IsNotFound(err) && name != "":
		return fmt.Errorf("%s %q not found%s; list them with klew_list_resources", r.Kind, name, where)
	case apierrors.IsBadRequest(err):
		return fmt.Errorf("%s %s: %s", verb, what, clean(err.Error()))
	}
	return fmt.Errorf("%s %s%s: %w", verb, what, where, err)
}

// ---- klew_list_resources

type listResourcesIn struct {
	KubeContext   string `json:"kube_context,omitempty" jsonschema:"Kube context name from klew_list_contexts. Omit to use the server default (kubeconfig current-context)."`
	Namespace     string `json:"namespace,omitempty" jsonschema:"One namespace name. Required for namespaced types; omit for cluster-scoped types such as nodes or namespaces."`
	ResourceType  string `json:"resource_type" jsonschema:"Resource type: kind (Deployment), plural (deployments), short name (deploy, po, svc) or resource.group for custom resources (certificates.cert-manager.io)."`
	LabelSelector string `json:"label_selector,omitempty" jsonschema:"Kubernetes label selector, e.g. app=web,tier!=cache."`
	FieldSelector string `json:"field_selector,omitempty" jsonschema:"Kubernetes field selector, e.g. status.phase=Failed or spec.nodeName=node-1 (supported fields depend on the type)."`
	Limit         int    `json:"limit,omitempty" jsonschema:"Max objects to return."`
}

// resourceRow is one object as a compact table row.
type resourceRow struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
	Status    string `json:"status,omitempty"`
	Owner     string `json:"owner,omitempty"`
	// Fields are the type's key columns (ready, restarts, node, ports, …).
	Fields map[string]string `json:"fields,omitempty"`
}

type listResourcesOut struct {
	Context    string        `json:"context"`
	Namespace  string        `json:"namespace,omitempty"`
	Kind       string        `json:"kind"`
	APIVersion string        `json:"api_version"`
	Returned   int           `json:"returned"`
	Items      []resourceRow `json:"items"`
	// More is true when objects exist beyond Items; narrow with selectors.
	More      bool   `json:"more"`
	Remaining *int64 `json:"remaining_estimate,omitempty"`
	Note      string `json:"note,omitempty"`
}

func (s *Server) listResources(ctx context.Context, _ *mcp.CallToolRequest, in listResourcesIn) (*mcp.CallToolResult, listResourcesOut, error) {
	client, err := s.newClient(in.KubeContext, "")
	if err != nil {
		return nil, listResourcesOut{}, err
	}
	r, err := s.resolveType(client, in.ResourceType)
	if err != nil {
		return nil, listResourcesOut{}, err
	}
	if !hasVerb(r, "list") {
		return nil, listResourcesOut{}, fmt.Errorf("%s cannot be listed", r.Kind)
	}
	ns, note, err := scopeNamespace(r, in.Namespace)
	if err != nil {
		return nil, listResourcesOut{}, err
	}
	if ns != "" && namespaceMissing(ctx, client, ns) {
		return nil, listResourcesOut{}, errNamespaceNotFound(ns, client.Context)
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	limit := clamp(in.Limit, 200, 50)
	list, err := kube.ListResources(ctx, client, r, kube.ResourceListOptions{
		Namespace: ns, LabelSelector: strings.TrimSpace(in.LabelSelector), FieldSelector: strings.TrimSpace(in.FieldSelector), Limit: int64(limit),
	})
	if err != nil {
		return nil, listResourcesOut{}, readError(err, "list", r, ns, "")
	}
	out := listResourcesOut{
		Context: client.Context, Namespace: ns, Kind: r.Kind, APIVersion: r.APIVersion,
		Items: mapSlice(list.Entities, listRow), More: list.More, Remaining: list.Remaining, Note: note,
	}
	out.Returned = len(out.Items)
	if out.More {
		out.Note = strings.TrimSpace(out.Note + " More objects exist; narrow with label_selector or field_selector, or raise limit.")
	}
	return nil, out, nil
}

func hasVerb(r kube.DiscoveredResource, verb string) bool {
	if len(r.Verbs) == 0 {
		return true // discovery without verbs: let the API decide
	}
	for _, v := range r.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// schedulingFields repeat the pod template's placement rules on every row;
// list rows leave them out, klew_get_resource keeps them.
var schedulingFields = []string{"affinity", "nodeSelector", "tolerations", "tolerationsCount"}

func listRow(e model.CatalogEntity) resourceRow {
	row := briefRow(e)
	for _, k := range schedulingFields {
		delete(row.Fields, k)
	}
	if len(row.Fields) == 0 {
		row.Fields = nil
	}
	return row
}

func briefRow(e model.CatalogEntity) resourceRow {
	row := resourceRow{Name: e.Name, Status: clean(e.StatusHint)}
	if t, err := time.Parse(time.RFC3339, e.CreationTimestamp); err == nil {
		row.CreatedAt = string(model.TimestampFrom(t))
	}
	if e.OwnerKind != "" || e.OwnerName != "" {
		row.Owner = refString(model.ObjectRef{Kind: e.OwnerKind, Name: e.OwnerName})
	}
	fields := map[string]string{}
	for k, v := range e.TableFields {
		if k != "status" && v != "" {
			fields[k] = clean(v)
		}
	}
	if e.Kind == "Pod" {
		ready, total := 0, 0
		for _, c := range e.Containers {
			if !c.Init {
				total++
				if c.Ready {
					ready++
				}
			}
		}
		if total > 0 {
			fields["ready"] = fmt.Sprintf("%d/%d", ready, total)
		}
		if e.CPUUsageMilli != nil {
			fields["cpu_usage"] = fmt.Sprintf("%dm", *e.CPUUsageMilli)
		}
		if e.MemUsageMi != nil {
			fields["memory_usage"] = fmt.Sprintf("%dMi", *e.MemUsageMi)
		}
	}
	if len(fields) > 0 {
		row.Fields = fields
	}
	return row
}

// ---- klew_get_resource

type getResourceIn struct {
	KubeContext     string `json:"kube_context,omitempty" jsonschema:"Kube context name from klew_list_contexts. Omit to use the server default (kubeconfig current-context)."`
	Namespace       string `json:"namespace,omitempty" jsonschema:"The object's namespace. Required for namespaced types; omit for cluster-scoped types."`
	ResourceType    string `json:"resource_type" jsonschema:"Resource type: kind (Deployment), plural (deployments), short name (deploy, po, svc) or resource.group for custom resources."`
	Name            string `json:"name" jsonschema:"Object name."`
	IncludeManifest bool   `json:"include_manifest,omitempty" jsonschema:"Also return the object as YAML (managedFields and last-applied-configuration removed, secrets masked). Costs more context; use it when the summary is not enough."`
}

type dataKey struct {
	Key       string `json:"key"`
	SizeBytes int    `json:"size_bytes"`
	Value     string `json:"value,omitempty"`
}

type resourceGetOut struct {
	Context     string                   `json:"context"`
	Kind        string                   `json:"kind"`
	APIVersion  string                   `json:"api_version"`
	Name        string                   `json:"name"`
	Namespace   string                   `json:"namespace,omitempty"`
	Summary     resourceRow              `json:"summary"`
	Labels      map[string]string        `json:"labels,omitempty"`
	Annotations map[string]string        `json:"annotations,omitempty"`
	Owners      []string                 `json:"owners,omitempty"`
	Conditions  []model.CatalogCondition `json:"conditions,omitempty"`
	Pod         *podBrief                `json:"pod,omitempty"`
	// Data lists ConfigMap and Secret keys. Secret entries never carry a value.
	Data              []dataKey `json:"data,omitempty"`
	DataTotal         int       `json:"data_total,omitempty"`
	Manifest          string    `json:"manifest,omitempty"`
	ManifestTruncated bool      `json:"manifest_truncated,omitempty"`
	Note              string    `json:"note,omitempty"`
}

func (s *Server) getResource(ctx context.Context, _ *mcp.CallToolRequest, in getResourceIn) (*mcp.CallToolResult, resourceGetOut, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, resourceGetOut{}, fmt.Errorf("name is required")
	}
	client, err := s.newClient(in.KubeContext, "")
	if err != nil {
		return nil, resourceGetOut{}, err
	}
	r, err := s.resolveType(client, in.ResourceType)
	if err != nil {
		return nil, resourceGetOut{}, err
	}
	ns, note, err := scopeNamespace(r, in.Namespace)
	if err != nil {
		return nil, resourceGetOut{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	obj, entity, err := kube.GetResource(ctx, client, r, ns, name)
	if err != nil {
		if apierrors.IsNotFound(err) && ns != "" && namespaceMissing(ctx, client, ns) {
			return nil, resourceGetOut{}, errNamespaceNotFound(ns, client.Context)
		}
		return nil, resourceGetOut{}, readError(err, "get", r, ns, name)
	}
	out := resourceGetOut{
		Context: client.Context, Kind: r.Kind, APIVersion: r.APIVersion, Name: obj.GetName(), Namespace: obj.GetNamespace(),
		Summary: briefRow(entity), Labels: cleanMap(obj.GetLabels(), 0), Annotations: briefAnnotations(obj.GetAnnotations()),
		Conditions: mapSlice(entity.Conditions, func(c model.CatalogCondition) model.CatalogCondition {
			c.Reason, c.Message = clean(c.Reason), clean(c.Message)
			return c
		}),
		Note: note,
	}
	for _, o := range obj.GetOwnerReferences() {
		out.Owners = append(out.Owners, refString(model.ObjectRef{Kind: o.Kind, Name: o.Name}))
	}
	core := r.Group == ""
	switch {
	case core && r.Resource == "pods":
		var pod corev1.Pod
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pod); err == nil {
			p := s.shape.pod(kube.SummarizePod(pod))
			out.Pod = &p
		}
	case core && r.Resource == "configmaps":
		out.Data, out.DataTotal = configMapKeys(obj.Object)
	case core && r.Resource == "secrets":
		out.Data, out.DataTotal = secretKeys(obj.Object)
	}
	if in.IncludeManifest {
		out.Manifest, out.ManifestTruncated = s.shape.manifest(obj.Object, core && r.Resource == "secrets")
	}
	return nil, out, nil
}

// cleanMap cleans every value; max > 0 keeps only the first max keys by name.
func cleanMap(in map[string]string, max int) map[string]string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if max > 0 {
		keys = capSlice(keys, max)
	}
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = clean(changes.RedactValue(k, in[k]))
	}
	return out
}

// briefAnnotations drops last-applied-configuration: it repeats the whole
// object as the client sent it, Secret values included.
func briefAnnotations(in map[string]string) map[string]string {
	rest := make(map[string]string, len(in))
	for k, v := range in {
		if k != lastApplied {
			rest[k] = v
		}
	}
	return cleanMap(rest, maxAnnotations)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// configMapKeys lists data keys with values (masked by key name and shape)
// and binaryData keys with sizes only.
func configMapKeys(obj map[string]any) ([]dataKey, int) {
	var out []dataKey
	data, _ := obj["data"].(map[string]any)
	for _, k := range sortedKeys(data) {
		v, _ := data[k].(string)
		out = append(out, dataKey{Key: k, SizeBytes: len(v), Value: clean(changes.RedactValue(k, v))})
	}
	bin, _ := obj["binaryData"].(map[string]any)
	for _, k := range sortedKeys(bin) {
		v, _ := bin[k].(string)
		out = append(out, dataKey{Key: k, SizeBytes: base64.StdEncoding.DecodedLen(len(v))})
	}
	return capSlice(out, maxDataKeys), len(out)
}

// secretKeys lists key names and decoded sizes. Values are never read out.
func secretKeys(obj map[string]any) ([]dataKey, int) {
	var out []dataKey
	data, _ := obj["data"].(map[string]any)
	for _, k := range sortedKeys(data) {
		out = append(out, dataKey{Key: k, SizeBytes: secretSize(data[k])})
	}
	return capSlice(out, maxDataKeys), len(out)
}

func secretSize(v any) int {
	s, _ := v.(string)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return len(b)
	}
	return base64.StdEncoding.DecodedLen(len(s))
}

// manifest renders obj as YAML for the model: managedFields and
// last-applied-configuration removed, Secret data replaced by sizes, env
// values masked by name and shape, and with log samples off, terminated
// container messages (the application's last words) dropped. The text then
// goes through redact and is capped at maxManifestLen.
func (sh shaper) manifest(obj map[string]any, secret bool) (string, bool) {
	o := runtime.DeepCopyJSON(obj)
	if meta, ok := o["metadata"].(map[string]any); ok {
		delete(meta, "managedFields")
		if ann, ok := meta["annotations"].(map[string]any); ok {
			delete(ann, lastApplied)
		}
	}
	if secret {
		for _, field := range []string{"data", "stringData"} {
			m, _ := o[field].(map[string]any)
			for k, v := range m {
				size := secretSize(v)
				if field == "stringData" {
					s, _ := v.(string)
					size = len(s)
				}
				m[k] = fmt.Sprintf("%s (%d bytes)", redact.Marker, size)
			}
		}
	} else if data, ok := o["data"].(map[string]any); ok {
		for k, v := range data {
			if s, ok := v.(string); ok {
				data[k] = changes.RedactValue(k, s)
			}
		}
	}
	sh.maskManifest(o)
	b, err := yaml.Marshal(o)
	if err != nil {
		return "", false
	}
	text := []rune(redact.String(string(b)))
	if len(text) <= maxManifestLen {
		return string(text), false
	}
	return string(text[:maxManifestLen]) + "\n# …truncated", true
}

// maskManifest walks the object: {name, value} pairs (env vars, and any list
// shaped like them) have the value masked by name and shape, and with log
// samples off every terminated.message is removed.
func (sh shaper) maskManifest(v any) {
	switch t := v.(type) {
	case map[string]any:
		if name, ok := t["name"].(string); ok {
			if val, ok := t["value"].(string); ok {
				t["value"] = changes.RedactValue(name, val)
			}
		}
		if sh.noLogSamples {
			if term, ok := t["terminated"].(map[string]any); ok {
				delete(term, "message")
			}
		}
		for _, child := range t {
			sh.maskManifest(child)
		}
	case []any:
		for _, child := range t {
			sh.maskManifest(child)
		}
	}
}

// ---- klew_pod_logs

type podLogsIn struct {
	KubeContext  string `json:"kube_context,omitempty" jsonschema:"Kube context name from klew_list_contexts. Omit to use the server default (kubeconfig current-context)."`
	Namespace    string `json:"namespace" jsonschema:"The pod's namespace."`
	Pod          string `json:"pod" jsonschema:"Pod name (from klew_list_resources or klew_investigate failing_pods)."`
	Container    string `json:"container,omitempty" jsonschema:"Container name. Optional when the pod has one container or a default-container annotation."`
	Previous     bool   `json:"previous,omitempty" jsonschema:"Read the previous, terminated instance of the container (after a crash or restart)."`
	TailLines    int    `json:"tail_lines,omitempty" jsonschema:"Number of most recent lines to read."`
	SinceMinutes int    `json:"since_minutes,omitempty" jsonschema:"Only lines from the last N minutes; 0 for no time limit."`
}

type logLine struct {
	Time string `json:"time,omitempty"`
	Text string `json:"text"`
}

type podLogsOut struct {
	Context   string    `json:"context"`
	Namespace string    `json:"namespace"`
	Pod       string    `json:"pod"`
	Container string    `json:"container"`
	Previous  bool      `json:"previous,omitempty"`
	Returned  int       `json:"returned"`
	Lines     []logLine `json:"lines"`
	// Truncated is true when older lines were left out to fit the size cap.
	Truncated bool   `json:"truncated"`
	Note      string `json:"note,omitempty"`
}

func (s *Server) podLogs(ctx context.Context, _ *mcp.CallToolRequest, in podLogsIn) (*mcp.CallToolResult, podLogsOut, error) {
	ns, err := parseNamespace(in.Namespace)
	if err != nil {
		return nil, podLogsOut{}, err
	}
	name := strings.TrimSpace(in.Pod)
	if name == "" {
		return nil, podLogsOut{}, fmt.Errorf("pod is required")
	}
	client, err := s.newClient(in.KubeContext, ns)
	if err != nil {
		return nil, podLogsOut{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	pod, err := client.Clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		switch {
		case apierrors.IsNotFound(err) && namespaceMissing(ctx, client, ns):
			return nil, podLogsOut{}, errNamespaceNotFound(ns, client.Context)
		case apierrors.IsNotFound(err):
			return nil, podLogsOut{}, fmt.Errorf("pod %q not found in %s; list pods with klew_list_resources", name, ns)
		case apierrors.IsForbidden(err):
			return nil, podLogsOut{}, fmt.Errorf("no permission to get pods in %s with this context: %s", ns, clean(err.Error()))
		}
		return nil, podLogsOut{}, err
	}
	container, err := pickContainer(pod, strings.TrimSpace(in.Container))
	if err != nil {
		return nil, podLogsOut{}, err
	}

	lines, cut, err := kube.ReadPodLogs(ctx, client, ns, name, kube.PodLogOptions{
		Container: container, Previous: in.Previous, TailLines: int64(clamp(in.TailLines, 1000, 100)),
		SinceSeconds: int64(max(in.SinceMinutes, 0)) * 60, LimitBytes: maxLogFetchBytes,
	})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return nil, podLogsOut{}, fmt.Errorf("no permission to read pod logs in %s with this context: %s", ns, clean(err.Error()))
		}
		if apierrors.IsBadRequest(err) {
			// e.g. no previous instance, or the container has not started.
			return nil, podLogsOut{}, fmt.Errorf("logs for %s/%s unavailable: %s", name, container, clean(err.Error()))
		}
		return nil, podLogsOut{}, fmt.Errorf("read logs for %s/%s: %w", name, container, err)
	}
	out := podLogsOut{Context: client.Context, Namespace: ns, Pod: name, Container: container, Previous: in.Previous}
	out.Lines, out.Truncated = briefLogLines(lines)
	out.Truncated = out.Truncated || cut
	out.Returned = len(out.Lines)
	if out.Returned == 0 {
		out.Note = "No log lines in this window."
	}
	if out.Truncated {
		out.Note = "Older lines were left out to fit the size cap; lower tail_lines or set since_minutes to see a narrower window."
	}
	return nil, out, nil
}

// pickContainer resolves which container to read: the named one, the only
// one, or the pod's default-container annotation.
func pickContainer(pod *corev1.Pod, want string) (string, error) {
	var names []string
	for _, c := range pod.Spec.InitContainers {
		names = append(names, c.Name)
	}
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	for _, c := range pod.Spec.EphemeralContainers {
		names = append(names, c.Name)
	}
	if want != "" {
		for _, n := range names {
			if n == want {
				return want, nil
			}
		}
		return "", fmt.Errorf("pod %s has no container %q; containers: %s", pod.Name, want, strings.Join(names, ", "))
	}
	if len(pod.Spec.Containers) == 1 {
		return pod.Spec.Containers[0].Name, nil
	}
	if def := pod.Annotations["kubectl.kubernetes.io/default-container"]; def != "" {
		return def, nil
	}
	return "", fmt.Errorf("pod %s has several containers; pass container, one of: %s", pod.Name, strings.Join(names, ", "))
}

// briefLogLines cleans every line and keeps the newest that fit maxLogOutputLen.
func briefLogLines(lines []kube.PodLogLine) ([]logLine, bool) {
	out := make([]logLine, len(lines))
	budget := maxLogOutputLen
	start := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		text := clean(lines[i].Text)
		budget -= len([]rune(text))
		if budget < 0 {
			break
		}
		out[i] = logLine{Text: text}
		if !lines[i].Time.IsZero() {
			out[i].Time = lines[i].Time.UTC().Format(time.RFC3339Nano)
		}
		start = i
	}
	return out[start:], start > 0
}
