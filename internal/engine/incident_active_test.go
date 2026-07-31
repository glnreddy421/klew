package engine

import (
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestRecoveredPodsClearCriticalVerdict(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{{
			Name: "payment-api-abc", Ready: true, Phase: "Running", RestartCount: 5,
			Containers: []model.ContainerStatus{{
				Name: "app", Ready: true, State: "running",
				LastReason: "OOMKilled", LastExitCode: 137, RestartCount: 5,
			}},
		}},
		Workloads: []model.WorkloadSummary{{
			Kind: "Deployment", Name: "payment-api", Replicas: 1, Ready: 1, Available: 1,
		}},
	}
	if IncidentActive(b) {
		t.Fatal("expected recovered pod to not be incident active")
	}
	if !WorkloadNominal(b) {
		t.Fatal("expected recovered workload to be nominal")
	}
	if oomPodCount(b) != 0 {
		t.Fatalf("expected no active OOM pods, got %d", oomPodCount(b))
	}

	st := model.NewInvestigationState("payment-api", model.ModeLive)
	st.Snapshot = b
	store := NewReducer(&st)
	store.ApplyEvent(model.EvidenceEvent{
		Timestamp: time.Now(), SourceType: model.SourceK8sEvent,
		Severity: model.SeverityCritical, Reason: "OOMKilled", Message: "exceeded memory limit",
	})
	out := store.State()
	if out.Verdict.Status != model.VerdictHealthy {
		t.Fatalf("expected healthy verdict after recovery, got %q leading=%q", out.Verdict.Status, out.Verdict.LeadingSignal)
	}
}

func TestReadyPodIgnoresStaleContainerBackoff(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{{
			Name: "payment-api-abc", Ready: true, Phase: "Running", RestartCount: 0,
			Containers: []model.ContainerStatus{{
				Name: "app", Ready: false, State: "waiting", Reason: "CrashLoopBackOff",
			}},
		}},
		Workloads: []model.WorkloadSummary{{
			Kind: "Deployment", Name: "payment-api", Replicas: 1, Ready: 1, Available: 1,
		}},
		Services: []model.ServiceSummary{{
			Name: "payment-api", ReadyEndpoints: 1, TotalEndpoints: 1,
		}},
	}
	if IncidentActive(b) {
		t.Fatal("ready pod should not be incident active")
	}
	if !WorkloadNominal(b) {
		t.Fatal("ready pod should be nominal")
	}
	corr := CorrelationEngine{}.Correlate(b, []model.Signal{{
		Label: "BackOff", Severity: model.SeverityHigh, Score: 40, Count: 10,
	}}, nil)
	if corr.LeadingSignal != "" {
		t.Fatalf("leading=%q want empty when recovered", corr.LeadingSignal)
	}
	if corr.HypothesisLabel != "Workload operating normally" {
		t.Fatalf("hypothesis=%q", corr.HypothesisLabel)
	}
}

func TestActiveCrashLoopStillCritical(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{{
			Name: "payment-api-abc", Ready: false, Phase: "Running",
			Containers: []model.ContainerStatus{{
				Name: "app", Ready: false, State: "waiting", Reason: "CrashLoopBackOff",
			}},
		}},
	}
	if !IncidentActive(b) {
		t.Fatal("expected crash loop pod to be incident active")
	}
}
