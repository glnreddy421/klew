package details

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestPodsForSelectorSnapshot(t *testing.T) {
	req := &Request{
		Snapshot: model.EvidenceBundle{
			Pods: []model.PodSummary{
				{Name: "web-abc", Phase: "Running", Ready: true, Node: "node-a", Labels: map[string]string{"app": "web"}},
				{Name: "other", Phase: "Running", Ready: true, Labels: map[string]string{"app": "other"}},
			},
		},
	}
	rows := podsForSelectorSnapshot(req, map[string]string{"app": "web"})
	if len(rows) != 1 || rows[0][0] != "web-abc" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestPodsRelationshipSectionFromSnapshot(t *testing.T) {
	req := &Request{
		Snapshot: model.EvidenceBundle{
			Pods: []model.PodSummary{
				{Name: "web-abc", Phase: "Running", Ready: true, Labels: map[string]string{"app": "web"}},
			},
		},
	}
	sec := podsRelationshipSection(t.Context(), req, map[string]string{"app": "web"}, nil)
	if sec.Empty() || sec.Table == nil || len(sec.Table.Rows) != 1 {
		t.Fatalf("section = %+v", sec)
	}
}

func TestAppendPodSummaryFields(t *testing.T) {
	got := appendPodSummaryFields([]string{"Replicas", "1/1"}, [][]string{{"web-abc", "Running", "True", "node-a"}})
	if len(got) != 4 || got[2] != "Pod" || got[3] != "web-abc" {
		t.Fatalf("summary = %v", got)
	}
}
