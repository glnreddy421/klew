package engine

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestCrashLoopNextChecksAreActionable(t *testing.T) {
	b := model.EvidenceBundle{
		Namespace: "klew-lab",
		Workloads: []model.WorkloadSummary{{
			Kind: "Deployment", Name: "payment-api", Selector: "app=payment-api",
		}},
		Pods: []model.PodSummary{{
			Name: "payment-api-abc12",
			Containers: []model.ContainerStatus{{
				Name: "app", Reason: "CrashLoopBackOff", LastReason: "Error",
				RestartCount: 7, Ready: false,
			}},
		}},
	}
	signals := ScoreSignals(b)
	corr := CorrelationEngine{}.Correlate(b, signals, nil)
	if corr.HypothesisLabel != "Container crash loop" {
		t.Fatalf("hypothesis=%q want Container crash loop", corr.HypothesisLabel)
	}
	if len(corr.NextChecks) == 0 {
		t.Fatal("expected next steps for crashloop")
	}
	for _, c := range corr.NextChecks {
		if strings.HasPrefix(c, "kubectl") {
			t.Fatalf("next steps should not be kubectl commands: %q", c)
		}
	}
	foundTermination := false
	foundLogs := false
	for _, c := range corr.NextChecks {
		if strings.Contains(c, "payment-api-abc12") && strings.Contains(c, "termination") {
			foundTermination = true
		}
		if strings.Contains(c, "payment-api-abc12") && strings.Contains(c, "logs") {
			foundLogs = true
		}
	}
	if !foundTermination {
		t.Fatalf("expected termination step referencing failing pod, got %v", corr.NextChecks)
	}
	if !foundLogs {
		t.Fatalf("expected log review step referencing failing pod, got %v", corr.NextChecks)
	}
	if len(corr.FixActions) == 0 {
		t.Fatal("expected fix actions for crashloop")
	}
	foundUndo := false
	for _, c := range corr.FixActions {
		if strings.Contains(c, "rollout undo") {
			foundUndo = true
		}
	}
	if !foundUndo {
		t.Fatalf("expected rollout undo fix, got %v", corr.FixActions)
	}
}

func TestOOMNextChecksWithoutMetrics(t *testing.T) {
	b := model.EvidenceBundle{
		Namespace: "klew-lab",
		Metrics:   model.MetricsSummary{Available: false},
		Pods: []model.PodSummary{{
			Name: "payment-api-xyz",
			Containers: []model.ContainerStatus{{
				Name: "app", LastReason: "OOMKilled", LastExitCode: 137,
				LimitsMem: "128Mi", RestartCount: 3,
			}},
		}},
	}
	steps := investigationStepsFor("OOMKilled", b)
	if len(steps) == 0 {
		t.Fatal("expected OOM investigation steps")
	}
	found := false
	for _, s := range steps {
		if strings.Contains(s, "128Mi") && strings.Contains(s, "OOMKilled") {
			found = true
		}
		if strings.HasPrefix(s, "kubectl") {
			t.Fatalf("unexpected kubectl in steps: %q", s)
		}
	}
	if !found {
		t.Fatalf("expected limit-aware OOM step, got %v", steps)
	}
}

func TestFailingPodPrefersCrashLoop(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{
			{Name: "healthy-a1", Containers: []model.ContainerStatus{{Name: "app", Ready: true}}},
			{Name: "broken-b2", Containers: []model.ContainerStatus{{
				Name: "app", Reason: "CrashLoopBackOff", RestartCount: 4,
			}}},
		},
	}
	pod, _ := failingPod(b)
	if pod != "broken-b2" {
		t.Fatalf("failingPod=%q want broken-b2", pod)
	}
}
