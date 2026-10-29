package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

type fakeHistory struct {
	recs []model.EventRecord
	cov  model.EventHistoryCoverage
}

func (f *fakeHistory) RecordedEvents(context.Context, string, string, model.EventHistoryQuery) ([]model.EventRecord, model.EventHistoryCoverage, error) {
	return f.recs, f.cov, nil
}

// connect starts the server on an in-memory transport and returns a client session.
func connect(t *testing.T, s *Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.MCP().Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func testServer(cs *fake.Clientset, hist kube.EventHistory) *Server {
	s := &Server{cache: newInvestigationCache(maxCachedInvestigations), runFor: engine.RunFor}
	s.newClient = func(kubeContext, namespace string) (*kube.Client, error) {
		c := &kube.Client{Clientset: cs, Config: &rest.Config{Host: "https://lab"}, Context: "kind-lab", Namespace: namespace}
		if hist != nil {
			c.History = hist
		}
		return c, nil
	}
	return s
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, map[string]any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out map[string]any
	if !res.IsError {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("%s structured content: %v", name, err)
		}
	}
	return res, out
}

func errText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestToolsAreReadOnly(t *testing.T) {
	cs := connect(t, testServer(fake.NewSimpleClientset(), nil))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"klew_list_contexts": true, "klew_investigate": true, "klew_investigation_details": true,
		"klew_event_history": true, "klew_workload_changes": true,
	}
	if len(res.Tools) != len(want) {
		t.Fatalf("got %d tools, want %d", len(res.Tools), len(want))
	}
	for _, tool := range res.Tools {
		if !want[tool.Name] {
			t.Errorf("unexpected tool %q", tool.Name)
		}
		a := tool.Annotations
		if a == nil || !a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint || a.Title == "" {
			t.Errorf("%s: annotations must be read-only, non-destructive and titled: %+v", tool.Name, a)
		}
		if len(tool.Name) > 64 {
			t.Errorf("%s: name longer than 64 characters", tool.Name)
		}
	}
}

func TestInvestigateSummaryAndDetails(t *testing.T) {
	s := testServer(fake.NewSimpleClientset(), nil)
	var gotOpts engine.LiveOptions
	var gotObserve time.Duration
	s.runFor = func(_ context.Context, _ *kube.Client, opts engine.LiveOptions, d time.Duration) (model.InvestigationState, error) {
		gotOpts, gotObserve = opts, d
		pods := []model.PodSummary{{Name: "api-ok", Phase: "Running", Ready: true}}
		for i := range 12 {
			pods = append(pods, model.PodSummary{
				Name: fmt.Sprintf("api-bad-%02d", i), Phase: "Running", RestartCount: int32(i),
				Containers: []model.ContainerStatus{{Name: "api", State: "waiting", Reason: "CrashLoopBackOff", LastReason: "OOMKilled", LastExitCode: 137, LimitsMem: "128Mi"}},
			})
		}
		return model.InvestigationState{
			CollectedAt: model.TimestampFrom(time.Now()), Query: opts.Query,
			KubeContext:    model.KubeContext{Context: "kind-lab"},
			NamespaceScope: model.NamespaceScope{Primary: opts.Namespace},
			Snapshot:       model.EvidenceBundle{Pods: pods},
			Verdict: model.Verdict{
				Status: "degraded", LeadingSignal: "OOMKilled", Confidence: 0.82,
				StrongSignals: []model.Signal{{Label: "OOMKilled", Strength: "strong", Severity: "critical"}},
			},
			CausalChain: []string{"memory limit 128Mi", "OOMKilled", "CrashLoopBackOff"},
		}, nil
	}
	cs := connect(t, s)

	res, _ := call(t, cs, "klew_investigate", map[string]any{"namespace": "shop", "observe_seconds": 500})
	if !res.IsError {
		t.Fatal("observe_seconds above the schema maximum should be rejected")
	}
	res, out := call(t, cs, "klew_investigate", map[string]any{"namespace": "shop", "workload": "api", "observe_seconds": 45})
	if res.IsError {
		t.Fatalf("investigate error: %s", errText(res))
	}
	if gotOpts.Namespace != "shop" || gotOpts.Query != "api" || gotObserve != 45*time.Second {
		t.Fatalf("runFor got %+v, %v", gotOpts, gotObserve)
	}
	if out["leading_signal"] != "OOMKilled" || out["failing_pods_total"] != float64(12) {
		t.Fatalf("summary = %v", out)
	}
	failing := out["failing_pods"].([]any)
	if len(failing) != 10 || failing[0].(map[string]any)["name"] != "api-bad-11" {
		t.Fatalf("failing pods not capped or not ranked by restarts: %v", failing)
	}
	id := out["investigation_id"].(string)

	_, page := call(t, cs, "klew_investigation_details", map[string]any{"investigation_id": id, "section": "pods", "limit": 5, "offset": 10})
	if page["total"] != float64(13) || len(page["items"].([]any)) != 3 || page["more"] != false {
		t.Fatalf("pods page = %v", page)
	}
	if last := page["items"].([]any)[2].(map[string]any); last["name"] != "api-ok" {
		t.Fatalf("healthy pod should rank last, got %v", last["name"])
	}

	res, _ = call(t, cs, "klew_investigation_details", map[string]any{"investigation_id": "inv_000000000000", "section": "pods"})
	if !res.IsError || !strings.Contains(errText(res), "klew_investigate") {
		t.Fatalf("missing id should error with a hint, got %v", errText(res))
	}
	res, _ = call(t, cs, "klew_investigation_details", map[string]any{"investigation_id": id, "section": "secrets"})
	if !res.IsError {
		t.Fatal("unknown section should be rejected")
	}
}

func TestEventHistoryMergesAndFilters(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ev := func(uid, obj, typ, reason string, ago time.Duration) *corev1.Event {
		ts := metav1.NewTime(now.Add(-ago))
		return &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: uid, Namespace: "shop", UID: types.UID(uid)},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: obj, Namespace: "shop"},
			Type:           typ, Reason: reason, Message: reason, FirstTimestamp: ts, LastTimestamp: ts,
		}
	}
	cs := fake.NewSimpleClientset(
		ev("u1", "api-1", "Warning", "BackOff", 2*time.Minute),
		ev("u2", "api-2", "Normal", "Pulled", 1*time.Minute),
		ev("u3", "web-1", "Warning", "BackOff", 1*time.Minute),
	)
	hist := &fakeHistory{
		recs: []model.EventRecord{{
			UID: "old", Type: "Warning", Reason: "OOMKilled", Message: "expired from API", Recorded: true,
			Timestamp: model.TimestampFrom(now.Add(-3 * time.Hour)), LastSeen: model.TimestampFrom(now.Add(-3 * time.Hour)),
			InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "api-0", Namespace: "shop"},
		}},
		cov: model.EventHistoryCoverage{Since: model.TimestampFrom(now.Add(-24 * time.Hour))},
	}
	sess := connect(t, testServer(cs, hist))

	res, out := call(t, sess, "klew_event_history", map[string]any{"namespace": "shop", "object_name": "api", "warnings_only": true})
	if res.IsError {
		t.Fatalf("event_history error: %s", errText(res))
	}
	evs := out["events"].([]any)
	if len(evs) != 2 || out["recorded_events"] != float64(1) || out["live_events"] != float64(1) || out["coverage"] == nil {
		t.Fatalf("event history = %v", out)
	}
	if first := evs[0].(map[string]any); first["reason"] != "BackOff" {
		t.Fatalf("want newest first, got %v", first)
	}
	if last := evs[1].(map[string]any); last["recorded"] != true {
		t.Fatalf("recorded event missing: %v", last)
	}

	_, out = call(t, sess, "klew_event_history", map[string]any{"namespace": "shop", "since_minutes": 60})
	if out["total"] != float64(3) {
		t.Fatalf("since_minutes should drop the 3h-old recorded event: %v", out)
	}
}

func TestEventHistoryForbiddenWithholdsRecorded(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "events", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "events"}, "", fmt.Errorf("denied"))
	})
	hist := &fakeHistory{recs: []model.EventRecord{{UID: "x", Reason: "OOMKilled", Recorded: true}}}
	res, _ := call(t, connect(t, testServer(cs, hist)), "klew_event_history", map[string]any{"namespace": "shop"})
	if !res.IsError || !strings.Contains(errText(res), "no permission") {
		t.Fatalf("want permission error, got %q", errText(res))
	}
}

func TestWorkloadChangesRejectsUnknownKind(t *testing.T) {
	sess := connect(t, testServer(fake.NewSimpleClientset(), nil))
	res, _ := call(t, sess, "klew_workload_changes", map[string]any{"namespace": "shop", "kind": "Secret", "name": "db"})
	if !res.IsError {
		t.Fatal("unsupported kind should be rejected")
	}
}

// leakyState is an investigation whose log-derived fields all carry a secret.
func leakyState() model.InvestigationState {
	const line = "auth failed password=hunter2-prod for jane@example.com"
	logTpl := model.LogTemplate{
		Template: "auth failed password=<*> for <*>", Count: 12, Severity: "error", Pods: []string{"api-1"},
		Samples: []model.LogSample{{Message: line, Pod: "api-1"}},
	}
	return model.InvestigationState{
		CollectedAt:    model.TimestampFrom(time.Now()),
		NamespaceScope: model.NamespaceScope{Primary: "shop"},
		Verdict: model.Verdict{
			Status: "degraded",
			StrongSignals: []model.Signal{
				{Label: "log signal", Source: "LOG", Strength: "strong", Evidence: line},
				{Label: "OOMKilled", Source: "OBJECT", Strength: "strong", Evidence: "container api/api reason=OOMKilled"},
			},
		},
		CausalChain: []string{"OOMKilled", "auth failed password=hunter2-prod +2m"},
		Timeline: []model.TimelineEvent{
			{Type: "log", Message: line},
			{Type: "k8s_event", Reason: "OOMKilled", Message: "container exceeded memory"},
		},
		LiveEvidence: []model.EvidenceEvent{{SourceType: model.SourceLog, Pod: "api-1", Message: line}},
		LogPatterns: &model.LogPatterns{
			Templates:     []model.LogTemplate{logTpl},
			EvidenceBoard: &model.EvidenceBoardPayload{Cards: []model.EvidenceCard{{RootEvent: model.LogTemplate{Template: "[OOMKilled] <*>"}, TriggeredLogs: []model.LogTemplate{logTpl}}}},
		},
	}
}

// plainJSON encodes v without HTML escaping, so "<*>" stays readable.
func plainJSON(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return b.String()
}

// allOutput runs an investigation and fetches every detail section, returning
// all the JSON the client received.
func allOutput(t *testing.T, s *Server) string {
	t.Helper()
	s.runFor = func(context.Context, *kube.Client, engine.LiveOptions, time.Duration) (model.InvestigationState, error) {
		return leakyState(), nil
	}
	cs := connect(t, s)
	res, out := call(t, cs, "klew_investigate", map[string]any{"namespace": "shop"})
	if res.IsError {
		t.Fatalf("investigate: %s", errText(res))
	}
	all := plainJSON(out)
	for _, sec := range detailSections {
		res, page := call(t, cs, "klew_investigation_details", map[string]any{"investigation_id": out["investigation_id"], "section": sec, "limit": 200})
		if res.IsError {
			t.Fatalf("%s: %s", sec, errText(res))
		}
		all += plainJSON(page)
	}
	return all
}

func TestResultsAreRedacted(t *testing.T) {
	all := allOutput(t, testServer(fake.NewSimpleClientset(), nil))
	for _, secret := range []string{"hunter2-prod", "jane@example.com"} {
		if strings.Contains(all, secret) {
			t.Errorf("%q reached the client", secret)
		}
	}
	if !strings.Contains(all, "auth failed password=‹redacted›") {
		t.Error("redacted log sample missing; samples should still be returned by default")
	}
}

func TestNoLogSamplesDropsLogText(t *testing.T) {
	s := testServer(fake.NewSimpleClientset(), nil)
	s.shape = shaper{noLogSamples: true}
	all := allOutput(t, s)
	if strings.Contains(all, "auth failed for") || strings.Contains(all, "auth failed password=‹redacted› for") {
		t.Errorf("raw log text reached the client with -no-log-samples:\n%s", all)
	}
	for _, want := range []string{
		"auth failed password=<*> for <*>",   // the template itself is kept
		"container api/api reason=OOMKilled", // non-log signal evidence is kept
		`"causal_chain":["OOMKilled"]`,       // log-worded chain step dropped
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestNamespaceMustBeAValidName(t *testing.T) {
	cs := connect(t, testServer(fake.NewSimpleClientset(), nil))
	for _, ns := range []string{"Prod", "shop; rm -rf /", "-bad", strings.Repeat("a", 64)} {
		res, _ := call(t, cs, "klew_event_history", map[string]any{"namespace": ns})
		if !res.IsError {
			t.Errorf("namespace %q accepted", ns)
		}
	}
}
