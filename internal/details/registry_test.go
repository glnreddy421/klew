package details

import (
	"strings"
	"testing"
)

func TestNormalizeKind(t *testing.T) {
	cases := map[string]string{
		"pvc":    "PersistentVolumeClaim",
		"HPA":    "HorizontalPodAutoscaler",
		"deploy": "Deployment",
		"Pod":    "Pod",
	}
	for in, want := range cases {
		if got := normalizeKind(in); got != want {
			t.Fatalf("normalizeKind(%q)=%q want %q", in, got, want)
		}
	}
}

func TestPruneEmptySections(t *testing.T) {
	in := []Section{
		{ID: "a", Title: "A", Fields: []Field{{Key: "k", Value: "v"}}},
		{ID: "b", Title: "B"},
		{ID: "c", Title: "C", Table: &Table{Columns: []string{"x"}, Rows: [][]string{{"1"}}}},
	}
	out := prune(in)
	if len(out) != 2 {
		t.Fatalf("prune len=%d want 2", len(out))
	}
	if out[0].ID != "a" || out[1].ID != "c" {
		t.Fatalf("unexpected prune order: %#v", out)
	}
}

func TestRegistryHasCoreKinds(t *testing.T) {
	kinds := []string{
		"Pod", "Deployment", "Service", "Ingress", "RoleBinding",
		"PersistentVolumeClaim", "HorizontalPodAutoscaler", "NetworkPolicy",
	}
	for _, k := range kinds {
		key := strings.ToLower(normalizeKind(k))
		regMu.RLock()
		_, ok := registry[key]
		regMu.RUnlock()
		if !ok {
			t.Fatalf("missing provider for %s", k)
		}
	}
}