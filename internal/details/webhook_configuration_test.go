package details

import "testing"

func TestWebhookConfigurationSection(t *testing.T) {
	obj := map[string]interface{}{
		"webhooks": []interface{}{
			map[string]interface{}{
				"name":          "kyverno-svc.kyverno.svc",
				"failurePolicy": "Fail",
				"sideEffects":   "None",
				"clientConfig": map[string]interface{}{
					"service": map[string]interface{}{
						"name":      "kyverno-svc",
						"namespace": "kyverno",
						"path":      "/celexception/validate",
						"port":      float64(443),
					},
				},
				"rules": []interface{}{
					map[string]interface{}{
						"apiGroups":   []interface{}{"policies.kyverno.io"},
						"apiVersions": []interface{}{"v1alpha1"},
						"operations":  []interface{}{"CREATE", "UPDATE"},
						"resources":   []interface{}{"policyexceptions"},
						"scope":       "*",
					},
				},
			},
		},
	}
	sec := webhookConfigurationSection(obj)
	if sec.Empty() {
		t.Fatal("expected webhooks section")
	}
	if len(sec.Table.Rows) != 1 {
		t.Fatalf("rows = %d", len(sec.Table.Rows))
	}
	row := sec.Table.Rows[0]
	if row[0] != "kyverno-svc.kyverno.svc" {
		t.Fatalf("name = %q", row[0])
	}
	if row[1] != "kyverno/kyverno-svc" {
		t.Fatalf("service = %q", row[1])
	}
	if row[2] != "/celexception/validate" {
		t.Fatalf("path = %q", row[2])
	}
	if row[3] != "Fail" {
		t.Fatalf("failurePolicy = %q", row[3])
	}
	if row[4] != "None" {
		t.Fatalf("sideEffects = %q", row[4])
	}
	if row[5] == "" {
		t.Fatal("expected rules summary")
	}
}

func TestGenericBodySectionsIncludesWebhooks(t *testing.T) {
	obj := map[string]interface{}{
		"webhooks": []interface{}{
			map[string]interface{}{"name": "hook-a", "failurePolicy": "Ignore"},
		},
	}
	sections := genericBodySections("SomeCustomKind", obj)
	if len(sections) != 1 || sections[0].ID != "webhooks" {
		t.Fatalf("sections = %#v", sections)
	}
}
