package kube

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestWorkloadMissingPods(t *testing.T) {
	b := model.EvidenceBundle{
		Workloads: []model.WorkloadSummary{{
			Name: "api", Replicas: 3, Ready: 0,
		}},
		ReplicaSets: []model.ReplicaSetSummary{{
			Name: "api-abc", DeploymentOwner: "api", Replicas: 3, Ready: 0,
		}},
		Pods: nil,
	}
	if !WorkloadMissingPods(b) {
		t.Fatal("expected missing pods")
	}
	b.Pods = []model.PodSummary{{
		Name: "api-abc-xyz",
		OwnerRefs: []model.ObjectRef{{Kind: "ReplicaSet", Name: "api-abc"}},
	}}
	if WorkloadMissingPods(b) {
		t.Fatal("expected pods present")
	}
	b.Workloads[0].Ready = 3
	if WorkloadMissingPods(b) {
		t.Fatal("ready replicas should not trigger")
	}
}
