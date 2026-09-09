package kube

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestEnrichNodeCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "desktop-control-plane"}
	enrichNodeCatalogEntity(&entity, map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		"spec": map[string]interface{}{
			"taints": []interface{}{
				map[string]interface{}{"key": "node-role.kubernetes.io/control-plane", "effect": "NoSchedule"},
			},
		},
		"status": map[string]interface{}{
			"nodeInfo": map[string]interface{}{
				"kubeletVersion": "v1.36.1",
			},
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
			"capacity": map[string]interface{}{
				"cpu":               "4",
				"memory":            "8127348Ki",
				"ephemeral-storage": "100Gi",
			},
			"allocatable": map[string]interface{}{
				"cpu":               "4",
				"memory":            "8027348Ki",
				"ephemeral-storage": "100Gi",
			},
		},
	})
	if entity.NodeResources == nil {
		t.Fatal("node resources missing")
	}
	if entity.NodeResources.Roles != "control-plane" {
		t.Fatalf("roles = %q", entity.NodeResources.Roles)
	}
	if entity.NodeResources.KubeletVersion != "v1.36.1" {
		t.Fatalf("version = %q", entity.NodeResources.KubeletVersion)
	}
	if entity.NodeResources.TaintCount == nil || *entity.NodeResources.TaintCount != 1 {
		t.Fatalf("taints = %v", entity.NodeResources.TaintCount)
	}
	if entity.NodeResources.Ready == nil || !*entity.NodeResources.Ready {
		t.Fatalf("ready = %v", entity.NodeResources.Ready)
	}
	if entity.StatusHint != "Ready" {
		t.Fatalf("status = %q", entity.StatusHint)
	}
}
