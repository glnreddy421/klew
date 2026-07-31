package engine

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestAnalyzeOOMRootCauseStartupAllocation(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{{
			Name: "payment-api-abc",
			Containers: []model.ContainerStatus{{
				Name: "app", LastReason: "OOMKilled", LastExitCode: 137, LimitsMem: "128Mi",
				Command: []string{"sh", "-c"},
				Args:    []string{"dd if=/dev/zero of=/tmp/leak bs=1M count=512"},
			}},
		}},
		PreviousLogs: []model.LogRecord{{
			PodName: "payment-api-abc", ContainerName: "app",
			Lines: []string{"klew-lab scenario 04-oomkill: allocating memory until OOM"},
		}},
	}
	rc := analyzeOOMRootCause(b, nil)
	if rc == nil {
		t.Fatal("expected root cause analysis")
	}
	if rc.Category != "startup_allocation" {
		t.Fatalf("category=%q want startup_allocation", rc.Category)
	}
	if !strings.Contains(rc.Label, "startup") {
		t.Fatalf("label=%q", rc.Label)
	}
	if len(rc.Bullets) == 0 {
		t.Fatal("expected correlation bullets")
	}
}

func TestCorrelateOOMLeadsOverBackOff(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{{
			Name: "payment-api-abc",
			Containers: []model.ContainerStatus{{
				Name: "app", LastReason: "OOMKilled", LastExitCode: 137, LimitsMem: "128Mi",
				Args: []string{"allocating memory until OOM"},
			}},
		}},
	}
	events := []model.EvidenceEvent{
		{SourceType: model.SourceK8sEvent, Reason: "BackOff", Message: "back-off restarting", Severity: model.SeverityHigh, Count: 50},
		{SourceType: model.SourceLog, Reason: "Memory allocation", Raw: "allocating memory until OOM", Severity: model.SeverityCritical},
	}
	signals := SignalAggregator{Window: 0}.Aggregate(events, events[0].Timestamp)
	corr := CorrelationEngine{}.Correlate(b, signals, events)
	if corr.LeadingSignal != "OOMKilled" {
		t.Fatalf("leading=%q want OOMKilled", corr.LeadingSignal)
	}
	if !strings.Contains(corr.HypothesisLabel, "startup") && !strings.Contains(corr.HypothesisLabel, "Memory limit exceeded") {
		t.Fatalf("hypothesis=%q", corr.HypothesisLabel)
	}
}

func TestClassifyLogReasonOOMContext(t *testing.T) {
	if got := ClassifyLogReason("klew-lab scenario 04-oomkill: allocating memory until OOM"); got != "Memory allocation" {
		t.Fatalf("got %q", got)
	}
	if got := ClassifyLogReason("fatal: out of memory allocating 64MB"); got != "Memory allocation failures" {
		t.Fatalf("got %q", got)
	}
}
