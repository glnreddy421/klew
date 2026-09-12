package kube

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestCatalogTableSchedulingPod(t *testing.T) {
	entity := model.CatalogEntity{Name: "web-abc"}
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"nodeSelector": map[string]interface{}{"disktype": "ssd"},
			"tolerations": []interface{}{
				map[string]interface{}{
					"key":    "dedicated",
					"value":  "gpu",
					"effect": "NoSchedule",
				},
			},
			"affinity": map[string]interface{}{
				"podAntiAffinity": map[string]interface{}{
					"requiredDuringSchedulingIgnoredDuringExecution": []interface{}{
						map[string]interface{}{"topologyKey": "kubernetes.io/hostname"},
					},
				},
			},
		},
	}
	enrichCatalogTableFields(&entity, obj, "pods")

	if entity.TableFields["nodeSelector"] != "disktype=ssd" {
		t.Fatalf("nodeSelector = %q", entity.TableFields["nodeSelector"])
	}
	if entity.TableFields["tolerations"] != "dedicated=gpu:NoSchedule" {
		t.Fatalf("tolerations = %q", entity.TableFields["tolerations"])
	}
	if entity.TableFields["tolerationsCount"] != "1" {
		t.Fatalf("tolerationsCount = %q", entity.TableFields["tolerationsCount"])
	}
	if entity.TableFields["affinity"] == "" {
		t.Fatalf("affinity missing")
	}
	if entity.Scheduling == nil || len(entity.Scheduling.Tolerations) != 1 {
		t.Fatalf("structured scheduling = %+v", entity.Scheduling)
	}
}
