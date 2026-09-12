package kube

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestApplyCatalogSchedulingStructured(t *testing.T) {
	entity := model.CatalogEntity{Name: "web-abc"}
	fields := map[string]string{}
	spec := map[string]interface{}{
		"nodeSelector": map[string]interface{}{"disktype": "ssd"},
		"tolerations": []interface{}{
			map[string]interface{}{
				"key":               "node.kubernetes.io/not-ready",
				"operator":          "Exists",
				"effect":            "NoExecute",
				"tolerationSeconds": int64(300),
			},
		},
	}
	applyCatalogScheduling(&entity, fields, spec)
	if entity.Scheduling == nil {
		t.Fatal("scheduling missing")
	}
	if len(entity.Scheduling.NodeSelector) != 1 || entity.Scheduling.NodeSelector[0].Key != "disktype" {
		t.Fatalf("node selector = %+v", entity.Scheduling.NodeSelector)
	}
	if len(entity.Scheduling.Tolerations) != 1 || entity.Scheduling.Tolerations[0].Seconds != "300" {
		t.Fatalf("tolerations = %+v", entity.Scheduling.Tolerations)
	}
}

func TestApplyCatalogNodeTaintsStructured(t *testing.T) {
	entity := model.CatalogEntity{Name: "node-a"}
	fields := map[string]string{}
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": []interface{}{
				map[string]interface{}{"key": "dedicated", "value": "gpu", "effect": "NoSchedule"},
			},
		},
	}
	applyCatalogNodeTaints(&entity, fields, obj)
	if entity.Scheduling == nil || len(entity.Scheduling.Taints) != 1 {
		t.Fatalf("taints = %+v", entity.Scheduling)
	}
	if fields["taints"] == "" {
		t.Fatal("expected taints label")
	}
}
