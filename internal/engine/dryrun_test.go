package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func certObj(name, status string) map[string]any {
	return map[string]any{
		"kind": "Certificate", "metadata": map[string]any{"name": name, "namespace": "ns"},
		"status": map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": status, "message": "msg " + name}}},
	}
}

func certBundle(objs ...map[string]any) model.EvidenceBundle {
	return model.EvidenceBundle{Namespace: "ns", Resources: map[string][]map[string]any{"certificates": objs}}
}

func TestRuleWithUnknownResourceIsSkippedWithSuggestion(t *testing.T) {
	dir := t.TempDir()
	writeExtension(t, dir, "a.yaml", `apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: typo}
spec: {resource: certifcates, when: "true", state: Typo}
---
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: nothing-like-it}
spec: {resource: zzzzzzzzzz, when: "true", state: Nothing}
---
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: uses-later-file}
spec: {resource: certificates, when: "true", state: Fine}
`)
	// The collector is in another file, loaded after the rule.
	writeExtension(t, dir, "b.yaml", `apiVersion: klew.dev/v1alpha1
kind: Collector
metadata: {name: certificates}
spec: {group: cert-manager.io, resource: certificates}
`)
	reg := LoadExtensions(dir)
	if got := reg.UserRuleNames(); len(got) != 1 || got[0] != "uses-later-file" {
		t.Fatalf("user rules = %v, want only uses-later-file", got)
	}
	warns := strings.Join(reg.Warnings, "\n")
	for _, want := range []string{
		`rule typo: resource "certifcates" is not a collector or built-in resource, so the rule is skipped. Did you mean "certificates"?`,
		`rule nothing-like-it: resource "zzzzzzzzzz" is not a collector or built-in resource, so the rule is skipped`,
	} {
		if !strings.Contains(warns, want) {
			t.Errorf("warnings %q lack %q", warns, want)
		}
	}
}

func TestRuleOverBuiltinResourceCollectorIsValid(t *testing.T) {
	dir := t.TempDir()
	writeExtension(t, dir, "r.yaml", `apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: pvc-pending}
spec: {resource: persistentvolumeclaims, when: "true", state: ClaimPending}
`)
	if reg := LoadExtensions(dir); len(reg.UserRuleNames()) != 1 || len(reg.Warnings) != 0 {
		t.Fatalf("rules %v warnings %v", reg.UserRuleNames(), reg.Warnings)
	}
}

func TestWhenThatIsNotBooleanIsReported(t *testing.T) {
	// object.status.conditions[0].status is a string; dyn hides that until it runs.
	rule, err := NewCELRule("wrong-type", RuleSpec{Resource: "certificates", State: "X", When: "object.status.conditions[0].status"})
	if err != nil {
		t.Fatal(err)
	}
	res := rule.Evaluate(certBundle(certObj("a", "False")))
	if len(res.Findings) != 0 || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "not true or false") {
		t.Fatalf("findings %v warnings %v", res.Findings, res.Warnings)
	}
}

func TestFailingFactIsReported(t *testing.T) {
	rule, err := NewCELRule("fact-fails", RuleSpec{Resource: "certificates", State: "X", When: "true",
		Facts: []string{"object.status.conditions[0].message", "object.spec.issuerRef.name"}})
	if err != nil {
		t.Fatal(err)
	}
	res := rule.Evaluate(certBundle(certObj("a", "False")))
	if len(res.Findings) != 1 || len(res.Findings[0].Facts) != 1 || res.Findings[0].Facts[0] != "msg a" {
		t.Fatalf("findings = %+v", res.Findings)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "1 facts could not be evaluated") {
		t.Fatalf("warnings = %v", res.Warnings)
	}
}

func TestFindingCapIsReported(t *testing.T) {
	rule, err := NewCELRule("all", RuleSpec{Resource: "certificates", State: "X", When: "true"})
	if err != nil {
		t.Fatal(err)
	}
	var objs []map[string]any
	for i := range maxRuleFindings + 5 {
		objs = append(objs, certObj(fmt.Sprintf("c%d", i), "False"))
	}
	res := rule.Evaluate(certBundle(objs...))
	if len(res.Findings) != maxRuleFindings {
		t.Fatalf("findings = %d", len(res.Findings))
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "stopped at 200 findings; 5 more objects were not checked") {
		t.Fatalf("warnings = %v", res.Warnings)
	}
	exact := rule.Evaluate(certBundle(objs[:maxRuleFindings]...))
	if len(exact.Warnings) != 0 {
		t.Fatalf("no objects were left unchecked, but warned %v", exact.Warnings)
	}
}

func TestDryRunReportsEveryObject(t *testing.T) {
	dir := t.TempDir()
	writeExtension(t, dir, "c.yaml", certExtension)
	reg := LoadExtensions(dir)
	run, err := reg.DryRunRule("certificate-not-ready", certBundle(
		certObj("good", "True"), certObj("bad", "False"),
		map[string]any{"kind": "Certificate", "metadata": map[string]any{"name": "new"}},
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Checks) != 3 || run.Matched() != 1 {
		t.Fatalf("checks = %+v", run.Checks)
	}
	byName := map[string]RuleCheck{}
	for _, c := range run.Checks {
		byName[c.Object.Name] = c
	}
	if c := byName["bad"]; !c.Matched || len(c.Facts) != 1 || c.Facts[0] != "Ready=False: msg bad" {
		t.Errorf("bad = %+v", c)
	}
	if c := byName["good"]; c.Matched || c.Error != "" {
		t.Errorf("good = %+v", c)
	}
	if c := byName["new"]; c.Error == "" {
		t.Errorf("new has no status and should fail to evaluate: %+v", c)
	}
	if len(run.Warnings) != 1 {
		t.Errorf("dry run should carry the investigation's warnings, got %v", run.Warnings)
	}
	if _, err := reg.DryRunRule("nope", certBundle()); err == nil || !strings.Contains(err.Error(), "loaded rules: certificate-not-ready") {
		t.Errorf("unknown rule error = %v", err)
	}
}

func TestParseObjectsAndSummaryGuard(t *testing.T) {
	list := `apiVersion: v1
kind: List
items:
- {kind: Certificate, metadata: {name: a}}
- {kind: Certificate, metadata: {name: b}}
---
{"kind": "Certificate", "metadata": {"name": "c"}}
`
	objs, err := ParseObjects([]byte(list))
	if err != nil || len(objs) != 3 {
		t.Fatalf("objects %v err %v", objs, err)
	}
	if _, err := ParseObjects([]byte("\n---\n")); err == nil {
		t.Error("empty input should fail")
	}
	if _, err := BundleFromObjects("pods", "ns", objs); err == nil || !strings.Contains(err.Error(), "namespace instead") {
		t.Errorf("pods summaries should be refused for saved objects: %v", err)
	}
}
