package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/v2/internal/engine"
	"github.com/glnreddy421/klew/v2/internal/history"
	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/model"
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
	_ = cs.Tracker().Add(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "shop"}})
	s := &Server{
		cache: newInvestigationCache(maxCachedInvestigations), runFor: engine.RunFor,
		investigations: make(chan struct{}, maxConcurrentInvestigations), progressEvery: progressInterval,
	}
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
		"klew_list_resources": true, "klew_get_resource": true, "klew_pod_logs": true,
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
	// reason and object_kind apply to live and recorded events, ignoring case.
	_, out = call(t, sess, "klew_event_history", map[string]any{"namespace": "shop", "reason": "backoff, OOMKilled"})
	if out["total"] != float64(3) || out["recorded_events"] != float64(1) {
		t.Fatalf("reason filter: %v", out)
	}
	_, out = call(t, sess, "klew_event_history", map[string]any{"namespace": "shop", "reason": "Pulled", "object_kind": "pod"})
	if out["total"] != float64(1) {
		t.Fatalf("reason and kind filter: %v", out)
	}
	_, out = call(t, sess, "klew_event_history", map[string]any{"namespace": "shop", "object_kind": "Node"})
	if out["total"] != float64(0) {
		t.Fatalf("kind filter should match nothing: %v", out)
	}
}

// seedHistory records one old Warning under normals newer Normal events.
func seedHistory(t *testing.T, normals int, now time.Time) *history.Reader {
	t.Helper()
	dir := t.TempDir()
	st, err := history.Open(filepath.Join(dir, "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := history.ClusterKey("kind-lab", "https://lab")
	row := func(uid, typ, reason string, at time.Time) history.EventRow {
		return history.EventRow{
			ClusterKey: key, UID: uid, Namespace: "shop", RegardingKind: "Pod", RegardingName: "api-1", Type: typ, Reason: reason,
			Message: reason, Count: 1, FirstSeen: at, LastSeen: at, RecordedAt: at,
		}
	}
	rows := []history.EventRow{row("oom", "Warning", "OOMKilling", now.Add(-2*time.Hour))}
	for i := range normals {
		rows = append(rows, row(fmt.Sprint("n", i), "Normal", "Pulled", now.Add(-time.Duration(i)*time.Second)))
	}
	if err := st.UpsertEvents(ctx, rows); err != nil {
		t.Fatal(err)
	}
	if _, err := st.BeginSession(ctx, key, "", now.Add(-3*time.Hour), false); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	r, err := history.OpenReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestEventHistoryWarningsSurviveTheRecordedCap(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	sess := connect(t, testServer(fake.NewSimpleClientset(), seedHistory(t, maxRecordedEvents, now)))

	_, out := call(t, sess, "klew_event_history", map[string]any{"namespace": "shop", "warnings_only": true})
	if out["total"] != float64(1) || out["truncated"] != false {
		t.Fatalf("warnings_only should find the old Warning: %v", out)
	}

	_, out = call(t, sess, "klew_event_history", map[string]any{"namespace": "shop"})
	if out["truncated"] != true || !strings.Contains(fmt.Sprint(out["note"]), "lower bound") {
		t.Fatalf("hitting the recorded cap must be reported: %v", out)
	}
}

func TestEventHistoryNoteWhenRecordingEndedBeforeWindow(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	sess := connect(t, testServer(fake.NewSimpleClientset(), seedHistory(t, 0, now.Add(-48*time.Hour))))
	_, out := call(t, sess, "klew_event_history", map[string]any{"namespace": "shop", "since_minutes": 60})
	note := fmt.Sprint(out["note"])
	if !strings.Contains(note, "since_minutes") || strings.Contains(note, "has not recorded") {
		t.Fatalf("note = %q", note)
	}
}

func TestPartialInvestigationIsFlagged(t *testing.T) {
	s := testServer(fake.NewSimpleClientset(), nil)
	s.runFor = func(context.Context, *kube.Client, engine.LiveOptions, time.Duration) (model.InvestigationState, error) {
		return model.InvestigationState{CollectedAt: model.TimestampFrom(time.Now())}, context.DeadlineExceeded
	}
	res, out := call(t, connect(t, s), "klew_investigate", map[string]any{"namespace": "shop"})
	if res.IsError || !strings.Contains(fmt.Sprint(out["warnings"]), "incomplete") {
		t.Fatalf("partial run should return a warning, got error=%v %v", res.IsError, out["warnings"])
	}
}

func TestObserveSeconds(t *testing.T) {
	s := testServer(fake.NewSimpleClientset(), nil)
	var got time.Duration
	s.runFor = func(_ context.Context, _ *kube.Client, _ engine.LiveOptions, d time.Duration) (model.InvestigationState, error) {
		got = d
		return model.InvestigationState{CollectedAt: model.TimestampFrom(time.Now())}, nil
	}
	cs := connect(t, s)
	call(t, cs, "klew_investigate", map[string]any{"namespace": "shop"})
	if got != defaultObserveSeconds*time.Second {
		t.Fatalf("unset observed %v, want the default", got)
	}
	call(t, cs, "klew_investigate", map[string]any{"namespace": "shop", "observe_seconds": 0})
	if got != 0 {
		t.Fatalf("observe_seconds 0 observed %v, want snapshot only", got)
	}
}

func TestInvestigateReportsProgress(t *testing.T) {
	s := testServer(fake.NewSimpleClientset(), nil)
	s.progressEvery = 5 * time.Millisecond
	s.runFor = func(context.Context, *kube.Client, engine.LiveOptions, time.Duration) (model.InvestigationState, error) {
		time.Sleep(60 * time.Millisecond)
		return model.InvestigationState{CollectedAt: model.TimestampFrom(time.Now())}, nil
	}
	var mu sync.Mutex
	var seen []*mcp.ProgressNotificationParams
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.MCP().Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			defer mu.Unlock()
			seen = append(seen, r.Params)
		},
	}).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	params := &mcp.CallToolParams{Name: "klew_investigate", Arguments: map[string]any{"namespace": "shop"}}
	params.SetProgressToken("tok")
	if _, err := cs.CallTool(ctx, params); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("no progress notifications")
	}
	for i, p := range seen {
		if p.ProgressToken != "tok" || (i > 0 && p.Progress <= seen[i-1].Progress) {
			t.Fatalf("progress[%d] = %+v, must carry the token and increase", i, p)
		}
	}
}

func TestShapeChangesCapsAndCleans(t *testing.T) {
	long := strings.Repeat("x", maxTextLen*3)
	diffs := make([]model.FieldDiff, maxDiffsPerRevision+5)
	for i := range diffs {
		diffs[i] = model.FieldDiff{Path: "spec.args", Before: long, After: "password=hunter2-prod"}
	}
	wc := model.WorkloadChanges{Changes: []model.WorkloadChange{{Revision: 2, Actor: "jane@example.com", Diffs: diffs}, {Revision: 1}}, HelmNote: long}

	out := shapeChanges(wc, 5, false)
	got := out.Changes[0]
	if len(got.Diffs) != maxDiffsPerRevision || got.DiffsTotal != len(diffs) {
		t.Fatalf("diffs = %d of %d", len(got.Diffs), got.DiffsTotal)
	}
	if b := got.Diffs[0].Before; len([]rune(b)) > maxTextLen+1 {
		t.Fatalf("before not truncated: %d runes", len([]rune(b)))
	}
	if strings.Contains(got.Diffs[0].After, "hunter2-prod") || strings.Contains(got.Actor, "jane@example.com") {
		t.Fatalf("not redacted: %+v", got.Diffs[0])
	}
	if len([]rune(out.HelmNote)) > maxTextLen+1 {
		t.Fatal("helm note not truncated")
	}
	if wc.Changes[0].Diffs[0].Before != long {
		t.Fatal("shaping changed the source diffs")
	}

	out = shapeChanges(wc, 1, true)
	if len(out.Changes) != 1 || out.TotalRevisions != 2 || out.Changes[0].Diffs != nil || out.Changes[0].DiffsTotal != len(diffs) {
		t.Fatalf("summary only = %+v", out.Changes[0])
	}
}

func TestWorkloadChangesResultMatchesItsSchema(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "shop"}})
	res, out := call(t, connect(t, testServer(cs, nil)), "klew_workload_changes", map[string]any{"namespace": "shop", "kind": "Deployment", "name": "api"})
	if res.IsError {
		t.Fatalf("workload_changes: %s", errText(res))
	}
	if _, ok := out["changes"].([]any); !ok {
		t.Fatalf("changes should be a list: %v", out)
	}
}

func TestBriefWarningsDedupesAndCaps(t *testing.T) {
	if got := briefWarnings([]string{"a", "b"}, []string{"a"}); len(got) != 2 {
		t.Fatalf("not de-duplicated: %v", got)
	}
	var many []string
	for i := range maxWarnings + 7 {
		many = append(many, fmt.Sprint("w", i))
	}
	got := briefWarnings(many)
	if len(got) != maxWarnings+1 || !strings.Contains(got[maxWarnings], "7 more") {
		t.Fatalf("got %d: %v", len(got), got[len(got)-1])
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
		Snapshot: model.EvidenceBundle{Pods: []model.PodSummary{{Name: "api-1", RestartCount: 1, Containers: []model.ContainerStatus{
			{Name: "api", State: "terminated", Message: "panic: card 4242 declined at checkout"},
			{Name: "sidecar", State: "waiting", Message: "back-off 5m0s restarting failed container"},
		}}}},
		CausalChain: []string{"OOMKilled", "auth failed password=hunter2-prod +2m", "retrying as admin:s3cret +3m"},
		Timeline: []model.TimelineEvent{
			{Type: "log", Message: line},
			{Type: "log", Message: "  retrying   as\tadmin:s3cret now"},
			{Type: "k8s_event", Reason: "OOMKilled", Message: "container exceeded memory"},
		},
		LiveEvidence: []model.EvidenceEvent{{SourceType: model.SourceLog, Pod: "api-1", Message: line}},
		LogPatterns: &model.LogPatterns{
			Templates: []model.LogTemplate{
				logTpl,
				{Template: "fatal: cannot reach vault for tenant acme-corp", Count: 40, Severity: "error"},
				{Template: "user <NUM> rejected for tenant globex", Count: 1, Severity: "error"},
				{Template: "retry <*> for tenant initech", Count: 1, Severity: "error"},
			},
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
	for _, leak := range []string{"auth failed for", "auth failed password=‹redacted› for", "retrying as", "card 4242", "acme-corp", "globex", "initech"} {
		if strings.Contains(all, leak) {
			t.Errorf("raw log text %q reached the client with -no-log-samples:\n%s", leak, all)
		}
	}
	for _, want := range []string{
		"auth failed password=<*> for <*>",   // the template itself is kept
		"container api/api reason=OOMKilled", // non-log signal evidence is kept
		`"causal_chain":["OOMKilled"]`,       // log-worded chain steps dropped
		"back-off 5m0s restarting",           // kubelet waiting message is kept
		withheldTemplate,                     // non-generalised template masked, count kept
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestUnreadableHistoryDoesNotStopTheServer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "events.db"), []byte(strings.Repeat("garbage", 2000)), 0o600); err != nil {
		t.Fatal(err)
	}
	// klew_list_contexts reads only the kubeconfig; give it one so the test
	// does not depend on the machine's.
	kubeconfig := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(kubeconfig, []byte(`apiVersion: v1
kind: Config
current-context: test
clusters: [{name: test, cluster: {server: "https://127.0.0.1:6443"}}]
users: [{name: test, user: {}}]
contexts: [{name: test, context: {cluster: test, user: test}}]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)
	s := New(Options{HistoryDir: dir})
	defer s.Close()
	if s.history != nil {
		t.Fatal("unreadable history should be disabled")
	}
	_, out := call(t, connect(t, s), "klew_list_contexts", nil)
	if out == nil {
		t.Fatal("server should still serve tools")
	}
}

func TestHistoryCreatedAfterStartIsPickedUp(t *testing.T) {
	dir := t.TempDir()
	s := New(Options{HistoryDir: dir})
	defer s.Close()
	if s.recordedHistory() != nil {
		t.Fatal("no database yet")
	}
	st, err := history.Open(filepath.Join(dir, "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	if s.recordedHistory() == nil {
		t.Fatal("history should open once it exists")
	}
}

func TestInvestigationCacheFreesExpiredEntries(t *testing.T) {
	c := newInvestigationCache(maxCachedInvestigations)
	now := time.Now()
	c.now = func() time.Time { return now }
	old := c.put(model.InvestigationState{})
	now = now.Add(investigationTTL / 2)
	live := c.put(model.InvestigationState{})
	now = now.Add(investigationTTL/2 + time.Second)

	if _, ok := c.get(old); ok {
		t.Fatal("expired investigation returned")
	}
	if _, ok := c.get(live); !ok {
		t.Fatal("live investigation dropped")
	}
	if len(c.items) != 1 || len(c.order) != 1 {
		t.Fatalf("items = %d, order = %d; the expired entry should be freed", len(c.items), len(c.order))
	}

	now = now.Add(investigationTTL * 2)
	c.put(model.InvestigationState{})
	if len(c.items) != 1 {
		t.Fatalf("items = %d; put should free expired entries", len(c.items))
	}
}

func TestInvestigationsAreCapped(t *testing.T) {
	s := testServer(fake.NewSimpleClientset(), nil)
	var mu sync.Mutex
	running, peak := 0, 0
	s.runFor = func(context.Context, *kube.Client, engine.LiveOptions, time.Duration) (model.InvestigationState, error) {
		mu.Lock()
		running++
		peak = max(peak, running)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
		return model.InvestigationState{CollectedAt: model.TimestampFrom(time.Now())}, nil
	}
	cs := connect(t, s)
	var wg sync.WaitGroup
	for range maxConcurrentInvestigations * 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "klew_investigate", Arguments: map[string]any{"namespace": "shop"}})
		}()
	}
	wg.Wait()
	if peak > maxConcurrentInvestigations {
		t.Fatalf("%d investigations ran at once, cap is %d", peak, maxConcurrentInvestigations)
	}
}

func TestNamespaceMustBeAValidName(t *testing.T) {
	cs := connect(t, testServer(fake.NewSimpleClientset(), nil))
	for _, ns := range []string{"Prod", "shop; rm -rf /", "-bad", "shop,web", "shop web", "*", strings.Repeat("a", 64)} {
		for _, args := range []struct {
			tool  string
			extra map[string]any
		}{
			{"klew_event_history", nil},
			{"klew_investigate", nil},
			{"klew_workload_changes", map[string]any{"kind": "Deployment", "name": "api"}},
		} {
			in := map[string]any{"namespace": ns}
			for k, v := range args.extra {
				in[k] = v
			}
			res, _ := call(t, cs, args.tool, in)
			if !res.IsError || !strings.Contains(errText(res), "call once for each") {
				t.Errorf("%s namespace %q: error = %v %q", args.tool, ns, res.IsError, errText(res))
			}
		}
	}
}

func TestMissingNamespaceIsReported(t *testing.T) {
	s := testServer(fake.NewSimpleClientset(), nil)
	s.runFor = func(context.Context, *kube.Client, engine.LiveOptions, time.Duration) (model.InvestigationState, error) {
		t.Error("investigated a namespace that does not exist")
		return model.InvestigationState{}, nil
	}
	cs := connect(t, s)
	res, _ := call(t, cs, "klew_investigate", map[string]any{"namespace": "all"})
	if !res.IsError || !strings.Contains(errText(res), `namespace "all" not found`) {
		t.Fatalf("investigate: %q", errText(res))
	}
	res, _ = call(t, cs, "klew_workload_changes", map[string]any{"namespace": "all", "kind": "Deployment", "name": "api"})
	if !res.IsError || !strings.Contains(errText(res), `namespace "all" not found`) {
		t.Fatalf("workload_changes: %q", errText(res))
	}
	res, out := call(t, cs, "klew_event_history", map[string]any{"namespace": "all"})
	if res.IsError || !strings.Contains(fmt.Sprint(out["note"]), "does not exist") {
		t.Fatalf("event_history should note it, got error=%v %v", res.IsError, out)
	}
}

// Clusters may deny get on namespaces while allowing everything a tool needs.
func TestNamespaceCheckIgnoresForbidden(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("get", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "shop", fmt.Errorf("denied"))
	})
	s := testServer(cs, nil)
	s.runFor = func(context.Context, *kube.Client, engine.LiveOptions, time.Duration) (model.InvestigationState, error) {
		return model.InvestigationState{CollectedAt: model.TimestampFrom(time.Now())}, nil
	}
	if res, _ := call(t, connect(t, s), "klew_investigate", map[string]any{"namespace": "shop", "observe_seconds": 0}); res.IsError {
		t.Fatalf("a forbidden namespace get must not stop the tool: %s", errText(res))
	}
}
