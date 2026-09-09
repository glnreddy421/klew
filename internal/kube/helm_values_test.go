package kube

import "testing"

func TestHelmUserSuppliedValues(t *testing.T) {
	defaults := map[string]interface{}{
		"replicas": float64(1),
		"nested": map[string]interface{}{
			"enabled": true,
			"count":   float64(3),
		},
	}
	config := map[string]interface{}{
		"replicas": float64(2),
		"nested": map[string]interface{}{
			"enabled": true,
			"count":   float64(5),
		},
		"extra": "value",
	}
	got := helmUserSuppliedValues(config, defaults)
	if got["replicas"] != float64(2) {
		t.Fatalf("replicas = %v", got["replicas"])
	}
	if got["extra"] != "value" {
		t.Fatalf("extra = %v", got["extra"])
	}
	nested, ok := got["nested"].(map[string]interface{})
	if !ok || nested["count"] != float64(5) {
		t.Fatalf("nested = %v", got["nested"])
	}
	if _, ok := nested["enabled"]; ok {
		t.Fatalf("expected unchanged nested.enabled to be omitted")
	}
}
