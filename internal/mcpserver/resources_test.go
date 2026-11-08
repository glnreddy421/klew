package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/glnreddy421/klew/v2/internal/kube"
)

var testResources = []kube.DiscoveredResource{
	{Version: "v1", APIVersion: "v1", Resource: "pods", Kind: "Pod", Namespaced: true, ShortNames: []string{"po"}, Verbs: []string{"get", "list", "watch"}},
	{Version: "v1", APIVersion: "v1", Resource: "secrets", Kind: "Secret", Namespaced: true, Verbs: []string{"get", "list"}},
	{Version: "v1", APIVersion: "v1", Resource: "configmaps", Kind: "ConfigMap", Namespaced: true, ShortNames: []string{"cm"}, Verbs: []string{"get", "list"}},
	{Version: "v1", APIVersion: "v1", Resource: "nodes", Kind: "Node", Verbs: []string{"get", "list"}},
}

// resourceServer serves objs through both the typed and the dynamic fake.
func resourceServer(t *testing.T, opts Options, objs ...runtime.Object) *Server {
	t.Helper()
	cs := fake.NewSimpleClientset(objs...)
	_ = cs.Tracker().Add(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "shop"}})
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, objs...)
	s := testServer(cs, nil)
	s.opts = opts
	s.shape = shaper{noLogSamples: opts.NoLogSamples}
	s.discover = func(*kube.Client) ([]kube.DiscoveredResource, error) { return testResources, nil }
	s.newClient = func(_, namespace string) (*kube.Client, error) {
		return &kube.Client{Clientset: cs, Dynamic: dyn, Config: &rest.Config{Host: "https://lab"}, Context: "kind-lab", Namespace: namespace}, nil
	}
	return s
}

func testPod(name, phase string, labels map[string]string, containers ...string) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "shop", Labels: labels},
		Status:     corev1.PodStatus{Phase: corev1.PodPhase(phase)},
	}
	for _, c := range containers {
		p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: c, Image: "app:1"})
		p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{Name: c, Ready: phase == "Running"})
	}
	return p
}

func resultJSON(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestListResources(t *testing.T) {
	s := resourceServer(t, Options{},
		testPod("web-1", "Running", map[string]string{"app": "web"}, "app"),
		testPod("web-2", "Failed", map[string]string{"app": "web"}, "app"),
		testPod("db-1", "Running", map[string]string{"app": "db"}, "db"),
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	)
	cs := connect(t, s)

	res, out := call(t, cs, "klew_list_resources", map[string]any{"namespace": "shop", "resource_type": "po"})
	if res.IsError {
		t.Fatal(errText(res))
	}
	if out["kind"] != "Pod" || out["returned"].(float64) != 3 {
		t.Fatalf("want 3 pods, got %v", out)
	}
	// Failed pods stay listed, unlike the browse catalog.
	if !strings.Contains(resultJSON(t, res), `"web-2"`) {
		t.Error("failed pod missing from list")
	}
	items := out["items"].([]any)
	first := items[0].(map[string]any)
	if fields, _ := first["fields"].(map[string]any); fields["ready"] == nil {
		t.Errorf("pod row has no ready column: %v", first)
	}

	_, out = call(t, cs, "klew_list_resources", map[string]any{"namespace": "shop", "resource_type": "pods", "label_selector": "app=web"})
	if out["returned"].(float64) != 2 {
		t.Errorf("label_selector app=web: want 2, got %v", out["returned"])
	}

	_, out = call(t, cs, "klew_list_resources", map[string]any{"resource_type": "nodes", "namespace": "shop"})
	if out["returned"].(float64) != 1 || !strings.Contains(out["note"].(string), "cluster-scoped") {
		t.Errorf("nodes: want 1 row and a cluster-scoped note, got %v", out)
	}

	res, _ = call(t, cs, "klew_list_resources", map[string]any{"resource_type": "pods"})
	if !res.IsError || !strings.Contains(errText(res), "namespace is required") {
		t.Errorf("namespaced type without namespace: got %q", errText(res))
	}
	res, _ = call(t, cs, "klew_list_resources", map[string]any{"namespace": "nope", "resource_type": "pods"})
	if !res.IsError || !strings.Contains(errText(res), "not found") {
		t.Errorf("missing namespace: got %q", errText(res))
	}
	res, _ = call(t, cs, "klew_list_resources", map[string]any{"namespace": "shop", "resource_type": "widgets"})
	if !res.IsError || !strings.Contains(errText(res), "no resource type") {
		t.Errorf("unknown type: got %q", errText(res))
	}
}

func TestGetSecretNeverReturnsValues(t *testing.T) {
	const value = "hunter2-very-secret"
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "shop", Annotations: map[string]string{
			lastApplied: `{"stringData":{"password":"` + value + `"}}`,
			"team":      "payments",
		}},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"password": []byte(value), "user": []byte("app")},
	}
	cs := connect(t, resourceServer(t, Options{}, sec))
	res, out := call(t, cs, "klew_get_resource", map[string]any{"namespace": "shop", "resource_type": "Secret", "name": "db", "include_manifest": true})
	if res.IsError {
		t.Fatal(errText(res))
	}
	raw := resultJSON(t, res)
	for _, leak := range []string{value, "aHVudGVyMi12ZXJ5LXNlY3JldA==", lastApplied} {
		if strings.Contains(raw, leak) {
			t.Errorf("result contains %q", leak)
		}
	}
	data := out["data"].([]any)
	if len(data) != 2 || data[0].(map[string]any)["key"] != "password" || data[0].(map[string]any)["size_bytes"].(float64) != float64(len(value)) {
		t.Errorf("want key names and sizes, got %v", data)
	}
	if _, ok := data[0].(map[string]any)["value"]; ok {
		t.Error("secret key carries a value")
	}
	if !strings.Contains(out["manifest"].(string), "‹redacted› (19 bytes)") {
		t.Errorf("manifest should show masked sizes:\n%s", out["manifest"])
	}
	if out["annotations"].(map[string]any)["team"] != "payments" {
		t.Errorf("ordinary annotations should stay: %v", out["annotations"])
	}
}

func TestGetConfigMapMasksCredentialKeys(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: "shop"},
		Data:       map[string]string{"LOG_LEVEL": "debug", "DB_PASSWORD": "letmein"},
	}
	cs := connect(t, resourceServer(t, Options{}, cm))
	res, out := call(t, cs, "klew_get_resource", map[string]any{"namespace": "shop", "resource_type": "cm", "name": "settings", "include_manifest": true})
	if res.IsError {
		t.Fatal(errText(res))
	}
	if strings.Contains(resultJSON(t, res), "letmein") {
		t.Error("credential-named ConfigMap value leaked")
	}
	if !strings.Contains(resultJSON(t, res), "debug") {
		t.Error("ordinary ConfigMap value should be returned")
	}
	if out["data_total"].(float64) != 2 {
		t.Errorf("data_total: %v", out["data_total"])
	}
}

func TestGetPodMasksEnvAndHonoursNoLogSamples(t *testing.T) {
	pod := testPod("api-1", "Running", nil, "api")
	pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "API_TOKEN", Value: "plain-token-value"}, {Name: "REGION", Value: "eu-west-1"}}
	pod.Status.ContainerStatuses[0].LastTerminationState = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
		ExitCode: 1, Reason: "Error", Message: "panic: connection refused to db",
	}}
	pod.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "kubectl"}}

	for _, noLog := range []bool{false, true} {
		cs := connect(t, resourceServer(t, Options{NoLogSamples: noLog}, pod))
		res, out := call(t, cs, "klew_get_resource", map[string]any{"namespace": "shop", "resource_type": "Pod", "name": "api-1", "include_manifest": true})
		if res.IsError {
			t.Fatal(errText(res))
		}
		m := out["manifest"].(string)
		if strings.Contains(m, "plain-token-value") || !strings.Contains(m, "eu-west-1") {
			t.Errorf("env masking wrong:\n%s", m)
		}
		if strings.Contains(m, "managedFields") {
			t.Error("managedFields not removed")
		}
		if got := strings.Contains(m, "panic: connection refused"); got == noLog {
			t.Errorf("noLogSamples=%v: termination message present=%v", noLog, got)
		}
		if out["pod"] == nil {
			t.Error("pod brief missing")
		}
	}
}

func TestPodLogs(t *testing.T) {
	s := resourceServer(t, Options{}, testPod("api-1", "Running", nil, "api"), testPod("multi", "Running", nil, "app", "sidecar"))
	cs := connect(t, s)
	res, out := call(t, cs, "klew_pod_logs", map[string]any{"namespace": "shop", "pod": "api-1"})
	if res.IsError {
		t.Fatal(errText(res))
	}
	if out["container"] != "api" || out["returned"].(float64) != 1 {
		t.Errorf("want one line from container api, got %v", out)
	}

	res, _ = call(t, cs, "klew_pod_logs", map[string]any{"namespace": "shop", "pod": "multi"})
	if !res.IsError || !strings.Contains(errText(res), "app, sidecar") {
		t.Errorf("multi-container pod without container: got %q", errText(res))
	}
	res, _ = call(t, cs, "klew_pod_logs", map[string]any{"namespace": "shop", "pod": "multi", "container": "nope"})
	if !res.IsError || !strings.Contains(errText(res), "no container") {
		t.Errorf("unknown container: got %q", errText(res))
	}
	res, _ = call(t, cs, "klew_pod_logs", map[string]any{"namespace": "shop", "pod": "ghost"})
	if !res.IsError || !strings.Contains(errText(res), "not found") {
		t.Errorf("missing pod: got %q", errText(res))
	}
}

func TestPodLogsAbsentWithoutLogSamples(t *testing.T) {
	cs := connect(t, resourceServer(t, Options{NoLogSamples: true}))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "klew_pod_logs" {
			t.Fatal("klew_pod_logs registered with -no-log-samples")
		}
	}
}

func TestBriefLogLinesKeepsNewest(t *testing.T) {
	var lines []kube.PodLogLine
	for range 200 {
		lines = append(lines, kube.PodLogLine{Text: strings.Repeat("x", 300)})
	}
	lines = append(lines, kube.PodLogLine{Text: "newest password=abc123"})
	out, truncated := briefLogLines(lines)
	if !truncated || len(out) == 0 || len(out) >= len(lines) {
		t.Fatalf("want a truncated tail, got %d lines truncated=%v", len(out), truncated)
	}
	last := out[len(out)-1].Text
	if !strings.HasPrefix(last, "newest") || strings.Contains(last, "abc123") {
		t.Errorf("newest line should be kept and redacted: %q", last)
	}
}

func TestPrompts(t *testing.T) {
	cs := connect(t, resourceServer(t, Options{}))
	ctx := context.Background()
	list, err := cs.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Prompts) != 2 {
		t.Fatalf("want 2 prompts, got %d", len(list.Prompts))
	}
	got, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "troubleshoot_workload", Arguments: map[string]string{
		"namespace": "shop", "workload": "Deployment/checkout", "kube_context": "kind-lab",
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := got.Messages[0].Content.(*mcp.TextContent).Text
	for _, want := range []string{`"Deployment/checkout"`, `"shop"`, `kube_context "kind-lab"`, "do not run them"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q:\n%s", want, text)
		}
	}
	if _, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "troubleshoot_namespace"}); err == nil {
		t.Error("missing namespace: want error")
	}
}
