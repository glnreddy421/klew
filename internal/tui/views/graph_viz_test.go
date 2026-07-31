package views

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/investigation"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/render"
)

func TestPropagationFlowVerticalNoTruncate(t *testing.T) {
	steps := []render.GraphStep{
		{Kind: "Deployment", Name: "payment-api-with-a-very-long-name", Health: "warning"},
		{Kind: "ReplicaSet", Name: "payment-api-7d8f9c6b4-x", Health: "warning", Relation: "replicaSet"},
		{Kind: "Pod", Name: "payment-api-7d8f9c6b4-x-abcd1", Health: "critical"},
	}
	out := PropagationFlow(steps, 40)
	if strings.Contains(out, "…") {
		t.Fatalf("propagation flow truncated names: %q", out)
	}
	if !strings.Contains(out, "payment-api-with-a-very") || !strings.Contains(out, "long-name") {
		t.Fatalf("missing full deployment name (wrapped): %q", out)
	}
	if !strings.Contains(out, "▼") {
		t.Fatalf("expected vertical connector: %q", out)
	}
}

func TestWrapPlainBreaksLongLines(t *testing.T) {
	lines := wrapPlain("Deployment/payment-api-with-a-very-long-resource-name", 20)
	if len(lines) < 2 {
		t.Fatalf("expected wrap, got %v", lines)
	}
}

func TestWorkloadMapIncludesCRDLayer(t *testing.T) {
	st := model.InvestigationState{
		Scope: &investigation.InvestigationScope{
			RootKind: "Deployment", RootName: "payment-api",
			RelatedCRDs: []investigation.RelatedCRD{{
				Extension: "Istio", Kind: "VirtualService",
				Refs: []investigation.Ref{{Kind: "VirtualService", Name: "payment-api"}},
			}},
		},
		WorkloadGraph: model.WorkloadGraph{
			Nodes: []model.GraphNode{
				{ID: "Deployment/payment-api", Kind: "Deployment", Name: "payment-api", Health: "warning"},
				{ID: "Pod/payment-api-abc", Kind: "Pod", Name: "payment-api-abc", Health: "critical"},
			},
			Edges: []model.GraphEdge{
				{From: "Deployment/payment-api", To: "Pod/payment-api-abc", Relation: "owns"},
			},
			HealthByNode: map[string]string{
				"Deployment/payment-api": "warning",
				"Pod/payment-api-abc":    "critical",
			},
		},
	}
	layout := GraphLayoutFor(st)
	out := WorkloadMap(st, layout, 60)
	if !strings.Contains(out, "Operators / CRD") {
		t.Fatalf("missing CRD layer: %q", out)
	}
	if !strings.Contains(out, "VirtualService/payment-api") {
		t.Fatalf("missing CRD node: %q", out)
	}
	if !strings.Contains(out, "✖") {
		t.Fatalf("missing critical marker: %q", out)
	}
}

func TestGraphViewShowsScrollHintWhenLong(t *testing.T) {
	st := model.InvestigationState{
		Snapshot: model.EvidenceBundle{
			Workloads: []model.WorkloadSummary{{Kind: "Deployment", Name: "app", Replicas: 3, Ready: 1}},
			ReplicaSets: []model.ReplicaSetSummary{
				{Name: "app-old", Replicas: 1, Ready: 0, DeploymentOwner: "app"},
				{Name: "app-new", Replicas: 2, Ready: 1, DeploymentOwner: "app"},
			},
			Pods: []model.PodSummary{
				{Name: "app-a", Ready: false, Phase: "Running", Containers: []model.ContainerStatus{{Name: "app", State: "waiting", Reason: "CrashLoopBackOff"}}},
				{Name: "app-b", Ready: true, Phase: "Running", Containers: []model.ContainerStatus{{Name: "app", Ready: true, State: "running"}}},
				{Name: "app-c", Ready: false, Phase: "Running", Containers: []model.ContainerStatus{{Name: "app", State: "waiting", Reason: "CrashLoopBackOff"}}},
			},
			Services: []model.ServiceSummary{{Name: "app", ReadyEndpoints: 1, TotalEndpoints: 3}},
		},
		WorkloadGraph: model.WorkloadGraph{
			Nodes: []model.GraphNode{
				{ID: "Deployment/app", Kind: "Deployment", Name: "app", Health: "warning"},
				{ID: "Pod/app-a", Kind: "Pod", Name: "app-a", Health: "critical"},
				{ID: "Pod/app-b", Kind: "Pod", Name: "app-b", Health: "healthy"},
				{ID: "Pod/app-c", Kind: "Pod", Name: "app-c", Health: "critical"},
			},
		},
	}
	out := GraphView(st, 80, 0, 12)
	if !strings.Contains(out, "j/k scroll") {
		t.Fatalf("expected scroll hint for long graph: %q", out)
	}
	if GraphLineCount(st, 80) <= 10 {
		t.Fatalf("expected many graph lines")
	}
}
