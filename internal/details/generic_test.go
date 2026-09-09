package details

import "testing"

func TestFlattenToMapSortedAndNested(t *testing.T) {
	kv := flattenToMap("", map[string]interface{}{
		"z": "last",
		"nested": map[string]interface{}{
			"b": "two",
			"a": true,
		},
		"items": []interface{}{"x", "y"},
	})
	if kv["z"] != "last" {
		t.Fatalf("z = %q", kv["z"])
	}
	if kv["nested.a"] != "true" {
		t.Fatalf("nested.a = %q", kv["nested.a"])
	}
	if kv["items"] != "x, y" {
		t.Fatalf("items = %q", kv["items"])
	}
}

func TestFlattenSliceObjects(t *testing.T) {
	out := map[string]string{}
	flattenSlice("rules", []interface{}{
		map[string]interface{}{"verbs": []interface{}{"get", "list"}},
	}, out)
	if out["rules[0].verbs"] != "get, list" {
		t.Fatalf("rules[0].verbs = %q", out["rules[0].verbs"])
	}
}
