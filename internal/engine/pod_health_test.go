package engine

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestSucceededJobPodDoesNotCountAsUnready(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{
			{Name: "payment-api-a", Ready: true, Phase: "Running"},
			{Name: "payment-api-b", Ready: true, Phase: "Running"},
			{Name: "payment-api-c", Ready: true, Phase: "Running"},
			{Name: "hello-world-123", Ready: false, Phase: "Succeeded"},
		},
		Workloads: []model.WorkloadSummary{{
			Kind: "Deployment", Name: "payment-api", Replicas: 3, Ready: 3, Available: 3,
		}},
	}
	if !WorkloadNominal(b) {
		t.Fatal("expected workload nominal with completed job pod in scope")
	}
	if IncidentActive(b) {
		t.Fatal("completed job pod must not keep incident active")
	}
	sum := BuildIncidentSummary(model.InvestigationState{Snapshot: b})
	if sum.UnreadyPods != 0 {
		t.Fatalf("unready=%d want 0", sum.UnreadyPods)
	}
	if sum.ReadyPods != 3 {
		t.Fatalf("ready=%d want 3", sum.ReadyPods)
	}
	st := model.NewInvestigationState("payment-api", model.ModeLive)
	st.Snapshot = b
	if statusFromSignals(nil, b) != model.VerdictHealthy {
		t.Fatalf("verdict=%s want healthy", statusFromSignals(nil, b))
	}
}
