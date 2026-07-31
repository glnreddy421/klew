package engine

import (
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestServiceSelectsPod(t *testing.T) {
	if !serviceSelectsPod("app=payment,tier=web", map[string]string{"app": "payment", "tier": "web"}) {
		t.Fatal("expected selector match")
	}
	if serviceSelectsPod("app=payment", map[string]string{"app": "other"}) {
		t.Fatal("expected no match")
	}
}

func TestSortTimeline(t *testing.T) {
	now := time.Now()
	events := []model.TimelineEvent{
		{Timestamp: now.Add(2 * time.Minute), Severity: model.SeverityInfo},
		{Timestamp: now, Severity: model.SeverityCritical},
	}
	sorted := SortTimeline(events)
	if sorted[0].Timestamp.After(sorted[1].Timestamp) {
		t.Fatal("expected chronological order")
	}
}

func TestScoreSignalsOOM(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{{
			Name: "p1",
			Containers: []model.ContainerStatus{{
				Name: "app", LastReason: "OOMKilled", RestartCount: 5,
			}},
		}},
	}
	sigs := ScoreSignals(b)
	found := false
	for _, s := range sigs {
		if s.ID == "oom_killed" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected OOMKilled signal")
	}
}

func TestGenerateVerdictCritical(t *testing.T) {
	b := model.EvidenceBundle{Namespace: "prod", Workloads: []model.WorkloadSummary{{Kind: "Deployment", Name: "payment"}}}
	signals := []model.Signal{{
		ID: "crashloop", Label: "CrashLoopBackOff", Strength: "strong", Severity: model.SeverityCritical, Score: 90,
	}}
	v := GenerateVerdict(b, nil, signals)
	if v.Status != model.VerdictCritical {
		t.Fatalf("expected critical got %s", v.Status)
	}
}

func TestDetectTrigger(t *testing.T) {
	now := time.Now()
	timeline := []model.TimelineEvent{
		{Timestamp: now, Severity: model.SeverityInfo, SourceKind: "Pod", SourceName: "a", Message: "started"},
		{Timestamp: now.Add(time.Minute), Severity: model.SeverityCritical, Reason: "OOMKilled", SourceKind: "Pod", SourceName: "b", Message: "oom", Confidence: 0.9},
	}
	trigger, conf := detectTrigger(timeline)
	if trigger == "" || conf < 0.5 {
		t.Fatalf("expected trigger, got %q conf=%v", trigger, conf)
	}
}
