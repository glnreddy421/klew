package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func warnEvent(reason, pod string, at time.Time, count int32) model.EventRecord {
	return model.EventRecord{
		Type: "Warning", Reason: reason, Count: count,
		Timestamp: model.TimestampFrom(at), FirstSeen: model.TimestampFrom(at),
		InvolvedObject: model.ObjectRef{Kind: "Pod", Name: pod, Namespace: "klew-lab"},
	}
}

func healthyBundle() model.EvidenceBundle {
	return model.EvidenceBundle{
		Workloads: []model.WorkloadSummary{{Kind: "Deployment", Name: "payment-api", Replicas: 1, Ready: 1, Available: 1}},
		Pods:      []model.PodSummary{{Name: "payment-api-84bd-j4tsh", Ready: true, Phase: "Running"}},
	}
}

func TestBuildIncidentsGroupsByProximity(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 0, 0, 0, time.UTC)
	b := healthyBundle()
	b.Events = []model.EventRecord{
		warnEvent("FailedCreate", "payment-api-old-1", now.Add(-3*time.Hour), 2),
		warnEvent("BackOff", "payment-api-old-1", now.Add(-3*time.Hour+5*time.Minute), 4),
		// 40 minutes later: a separate incident.
		warnEvent("OOMKilling", "payment-api-old-2", now.Add(-2*time.Hour), 1),
		{Type: "Normal", Reason: "Pulled", Timestamp: model.TimestampFrom(now.Add(-time.Hour))},
	}
	got := BuildIncidents(b, now)
	if len(got) != 2 {
		t.Fatalf("want 2 incidents, got %+v", got)
	}
	first := got[0]
	if first.Count != 6 || len(first.Objects) != 1 {
		t.Fatalf("first incident = %+v", first)
	}
	// FailedCreate and BackOff: the more severe reason leads.
	if first.LeadingReason != "FailedCreate" && first.LeadingReason != "BackOff" {
		t.Fatalf("leading reason = %q", first.LeadingReason)
	}
	if !first.Recovered || !got[1].Recovered {
		t.Fatalf("old incidents on replaced pods of a healthy workload should be recovered: %+v", got)
	}
}

func TestIncidentOngoingWithinQuietPeriod(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 0, 0, 0, time.UTC)
	b := healthyBundle()
	b.Events = []model.EventRecord{warnEvent("BackOff", "payment-api-84bd-j4tsh", now.Add(-4*time.Minute), 1)}
	got := BuildIncidents(b, now)
	if len(got) != 1 || got[0].Recovered {
		t.Fatalf("a Warning 4 minutes ago is ongoing even with healthy pods: %+v", got)
	}
	// Quiet for the full period → recovered.
	got = BuildIncidents(b, now.Add(IncidentQuietPeriod))
	if !got[0].Recovered {
		t.Fatal("quiet period passed with healthy objects → recovered")
	}
}

func TestIncidentOngoingWhileObjectUnhealthy(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 0, 0, 0, time.UTC)
	b := healthyBundle()
	b.Pods[0].Ready = false
	b.Events = []model.EventRecord{warnEvent("BackOff", "payment-api-84bd-j4tsh", now.Add(-2*time.Hour), 1)}
	if got := BuildIncidents(b, now); got[0].Recovered {
		t.Fatal("old Warning on a pod that is still not ready must stay ongoing")
	}
}

func TestHealthyNowLabels(t *testing.T) {
	rec := model.Incident{Recovered: true}
	ongoing := model.Incident{}
	cases := []struct {
		in   []model.Incident
		win  time.Duration
		want string
	}{
		{nil, 6 * time.Hour, ""},
		{[]model.Incident{rec}, 6 * time.Hour, "Healthy now · 1 incident in the last 6h (recovered)"},
		{[]model.Incident{rec, rec}, 6 * time.Hour, "Healthy now · 2 incidents in the last 6h (recovered)"},
		{[]model.Incident{ongoing}, 15 * time.Minute, "Healthy now · 1 ongoing incident in the last 15m"},
		{[]model.Incident{rec, ongoing}, 6 * time.Hour, "Healthy now · 1 ongoing, 1 recovered incident in the last 6h"},
	}
	for _, c := range cases {
		if got := HealthyNowLabel(c.in, c.win); got != c.want {
			t.Errorf("HealthyNowLabel(%d incidents) = %q, want %q", len(c.in), got, c.want)
		}
	}
}

// Healthy now + past Warnings → "Healthy now · N incidents (recovered)";
// no Warnings → label unchanged; unhealthy now → the existing verdict.
func TestReducerHeadlineHealthyNowVsUnhealthy(t *testing.T) {
	now := time.Now().UTC()
	b := healthyBundle()
	b.Events = []model.EventRecord{warnEvent("FailedCreate", "payment-api-old-1", now.Add(-3*time.Hour), 2)}
	b.EventHistory = &model.EventHistoryCoverage{Since: model.TimestampFrom(now.Add(-6 * time.Hour)), RecordedEvents: 1}

	st := BootstrapState(b, model.NamespaceScope{Primary: "klew-lab"}, "payment-api", model.ModeLive)
	r := NewReducer(st)
	r.recompute()
	if got := r.State().HypothesisLabel; !strings.HasPrefix(got, "Healthy now · 1 incident in the last 6h (recovered)") {
		t.Fatalf("healthy now with a past incident: %q", got)
	}

	clean := healthyBundle()
	r = NewReducer(BootstrapState(clean, model.NamespaceScope{Primary: "klew-lab"}, "payment-api", model.ModeLive))
	r.recompute()
	if got := r.State().HypothesisLabel; got != HealthyLabel {
		t.Fatalf("no warnings: %q", got)
	}

	broken := healthyBundle()
	broken.Pods[0].Ready = false
	broken.Pods[0].Containers = []model.ContainerStatus{{Name: "app", State: "waiting", Reason: "CrashLoopBackOff", RestartCount: 5}}
	broken.Workloads[0].Ready, broken.Workloads[0].Available = 0, 0
	broken.Events = b.Events
	r = NewReducer(BootstrapState(broken, model.NamespaceScope{Primary: "klew-lab"}, "payment-api", model.ModeLive))
	r.recompute()
	if got := r.State().HypothesisLabel; strings.HasPrefix(got, "Healthy now") || got == HealthyLabel {
		t.Fatalf("unhealthy now must keep the failure verdict, got %q", got)
	}
}

func TestCausalChainSkipsSystemEvidence(t *testing.T) {
	now := time.Now().UTC()
	tl := []model.TimelineEvent{
		{Timestamp: model.TimestampFrom(now.Add(-2 * time.Minute)), Type: "k8s_event", Severity: model.SeverityHigh, Reason: "BackOff"},
		{Timestamp: model.TimestampFrom(now.Add(-time.Minute)), Type: string(model.SourceSystem), Severity: model.SeverityWarning, Reason: "metrics_source"},
		{Timestamp: model.TimestampFrom(now), Type: "k8s_event", Severity: model.SeverityCritical, Reason: "OOMKilling"},
	}
	for _, step := range buildCausalChain(tl) {
		if strings.Contains(step, "metrics_source") {
			t.Fatalf("system evidence leaked into the chain: %v", buildCausalChain(tl))
		}
	}
}
