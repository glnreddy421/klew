package render

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestPropagationSteps(t *testing.T) {
	g := model.WorkloadGraph{
		PropagationPath: []string{
			"Deployment/pay -> ReplicaSet/pay-abc (replicaSet)",
			"ReplicaSet/pay-abc -> Pod/pay-abc-1 (ownerReference)",
		},
		HealthByNode: map[string]string{
			"Deployment/pay":     "warning",
			"ReplicaSet/pay-abc": "warning",
			"Pod/pay-abc-1":      "critical",
		},
	}
	steps := PropagationSteps(g)
	if len(steps) != 3 {
		t.Fatalf("steps=%d want 3", len(steps))
	}
	if steps[0].ID != "Deployment/pay" || steps[2].Health != "critical" {
		t.Fatalf("unexpected steps: %+v", steps)
	}
}

func TestLayoutGraph(t *testing.T) {
	g := model.WorkloadGraph{
		Nodes: []model.GraphNode{
			{ID: "Deployment/a", Kind: "Deployment", Name: "a", Health: "healthy"},
			{ID: "Pod/b", Kind: "Pod", Name: "b", Health: "critical"},
		},
		Edges: []model.GraphEdge{{From: "Deployment/a", To: "Pod/b", Relation: "owns"}},
	}
	layout := LayoutGraph(g)
	if len(layout.Nodes) != 2 {
		t.Fatalf("nodes=%d", len(layout.Nodes))
	}
	if layout.Width <= 0 || layout.Height <= 0 {
		t.Fatalf("bad dimensions %.0fx%.0f", layout.Width, layout.Height)
	}
}
