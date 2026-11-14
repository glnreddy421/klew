package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func writeExtension(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const certExtension = `apiVersion: klew.dev/v1alpha1
kind: Collector
metadata: {name: certificates}
spec: {group: cert-manager.io, version: v1, resource: certificates}
---
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: certificate-not-ready}
spec:
  resource: certificates
  when: "object.status.conditions.exists(c, c.type == 'Ready' && c.status != 'True')"
  severity: high
  state: CertificateNotReady
  facts:
    - "'Ready=False: ' + object.status.conditions.filter(c, c.type == 'Ready')[0].message"
`

func TestLoadExtensionsAddsCollectorsAndRules(t *testing.T) {
	dir := t.TempDir()
	writeExtension(t, dir, "certs.yaml", certExtension)
	writeExtension(t, dir, "notes.txt", "not an extension")
	reg := LoadExtensions(dir)
	if len(reg.Warnings) != 0 {
		t.Fatalf("warnings = %v", reg.Warnings)
	}
	if len(reg.Collectors) != 1 || reg.Collectors[0].Name() != "certificates" {
		t.Fatalf("collectors = %v", reg.Collectors)
	}
	if got := reg.Rules[len(reg.Rules)-1].Name(); got != "certificate-not-ready" {
		t.Fatalf("last rule = %q", got)
	}
	if n := len(reg.Rules) - len(DefaultRules()); n != 1 {
		t.Fatalf("added %d rules, want 1", n)
	}
}

func TestLoadExtensionsMissingDirIsDefault(t *testing.T) {
	reg := LoadExtensions(filepath.Join(t.TempDir(), "absent"))
	if len(reg.Warnings) != 0 || len(reg.Collectors) != 0 || len(reg.Rules) != len(DefaultRules()) {
		t.Fatalf("registry = %+v", reg)
	}
}

func TestLoadExtensionsRejectsBadDocuments(t *testing.T) {
	cases := map[string]string{
		"secrets":     "apiVersion: klew.dev/v1alpha1\nkind: Collector\nmetadata: {name: s}\nspec: {version: v1, resource: secrets}\n",
		"subresource": "apiVersion: klew.dev/v1alpha1\nkind: Collector\nmetadata: {name: l}\nspec: {version: v1, resource: pods/log}\n",
		"bad cel":     "apiVersion: klew.dev/v1alpha1\nkind: Rule\nmetadata: {name: r}\nspec: {resource: pods, when: 'object.(', state: X}\n",
		"not bool":    "apiVersion: klew.dev/v1alpha1\nkind: Rule\nmetadata: {name: r}\nspec: {resource: pods, when: '1 + 1', state: X}\n",
		"no state":    "apiVersion: klew.dev/v1alpha1\nkind: Rule\nmetadata: {name: r}\nspec: {resource: pods, when: 'true'}\n",
		"severity":    "apiVersion: klew.dev/v1alpha1\nkind: Rule\nmetadata: {name: r}\nspec: {resource: pods, when: 'true', state: X, severity: urgent}\n",
		"unknown key": "apiVersion: klew.dev/v1alpha1\nkind: Rule\nmetadata: {name: r}\nspec: {resource: pods, when: 'true', state: X, run: 'kubectl delete'}\n",
		"version":     "apiVersion: v1\nkind: Rule\nmetadata: {name: r}\nspec: {resource: pods, when: 'true', state: X}\n",
		"kind":        "apiVersion: klew.dev/v1alpha1\nkind: Action\nmetadata: {name: r}\nspec: {}\n",
		"builtin":     "apiVersion: klew.dev/v1alpha1\nkind: Rule\nmetadata: {name: pod-state}\nspec: {resource: pods, when: 'true', state: X}\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeExtension(t, dir, "x.yaml", body)
			reg := LoadExtensions(dir)
			if len(reg.Warnings) != 1 || len(reg.Collectors) != 0 || len(reg.Rules) != len(DefaultRules()) {
				t.Fatalf("registry = %d collectors, %d rules, warnings %v; want the document rejected",
					len(reg.Collectors), len(reg.Rules), reg.Warnings)
			}
		})
	}
}

func TestCELRuleOnCustomObjects(t *testing.T) {
	rule, err := NewCELRule("certificate-not-ready", RuleSpec{
		Resource: "certificates", Severity: model.SeverityHigh, State: "CertificateNotReady",
		When:  "object.status.conditions.exists(c, c.type == 'Ready' && c.status != 'True')",
		Facts: []string{"object.status.conditions[0].message"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cert := func(name, status, msg string) map[string]any {
		return map[string]any{
			"kind": "Certificate", "metadata": map[string]any{"name": name, "namespace": "ns"},
			"status": map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": status, "message": msg}}},
		}
	}
	b := model.EvidenceBundle{Resources: map[string][]map[string]any{"certificates": {
		cert("good", "True", "issued"), cert("expired", "False", "certificate expired"),
		{"kind": "Certificate", "metadata": map[string]any{"name": "new"}},
	}}}
	res := rule.Evaluate(b)
	if len(res.Findings) != 1 {
		t.Fatalf("findings = %+v", res.Findings)
	}
	f := res.Findings[0]
	if f.Object != (model.ObjectRef{Kind: "Certificate", Name: "expired", Namespace: "ns"}) || f.State != "CertificateNotReady" ||
		len(f.Facts) != 1 || f.Facts[0] != "certificate expired" {
		t.Fatalf("finding = %+v", f)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "1 objects could not be evaluated") {
		t.Fatalf("warnings = %v, want one for the object without status", res.Warnings)
	}
}

func TestCELRuleOnBuiltinPodsFeedsProblems(t *testing.T) {
	rule, err := NewCELRule("too-many-restarts", RuleSpec{
		Resource: "pods", State: "RestartingOften", Severity: model.SeverityHigh,
		When:  "object.restartCount > 5",
		Facts: []string{"'restarts: ' + string(object.restartCount)"},
	})
	if err != nil {
		t.Fatal(err)
	}
	pod := ownedPod("cache-1", "StatefulSet", "cache", true, model.ContainerStatus{Name: "c", State: "running", RestartCount: 7})
	b := model.EvidenceBundle{Pods: []model.PodSummary{pod}}
	reg := &Registry{Rules: append(DefaultRules(), rule)}
	res := reg.Evaluate(b)
	problems := BuildProblems(b, res.Findings)
	if len(problems) != 1 || problems[0].Object != "StatefulSet/cache" || problems[0].State != "RestartingOften" ||
		problems[0].Rules[0] != "too-many-restarts" || problems[0].Facts[0] != "restarts: 7" {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestActiveRegistryDrivesScoreSignals(t *testing.T) {
	t.Cleanup(func() { SetRegistry(nil) })
	SetRegistry(&Registry{Rules: []Rule{signalRule("only", func(model.EvidenceBundle) []model.Signal {
		return []model.Signal{{ID: "x", Label: "X"}}
	})}})
	if got := ScoreSignals(model.EvidenceBundle{}); len(got) != 1 || got[0].ID != "x" {
		t.Fatalf("ScoreSignals = %v", got)
	}
}

func TestCELRuleCanCheckConfigMapKeys(t *testing.T) {
	rule, err := NewCELRule("needs-db-host", RuleSpec{Resource: "configmaps", State: "MissingKey", Severity: model.SeverityHigh,
		When: "object.metadata.name == 'app' && !('DB_HOST' in object.keys)", Facts: []string{"'keys: ' + object.keys.join(', ')"}})
	if err != nil {
		t.Fatal(err)
	}
	b := model.EvidenceBundle{Resources: map[string][]map[string]any{"configmaps": {
		{"kind": "ConfigMap", "metadata": map[string]any{"name": "app"}, "keys": []string{"DB_PORT", "DB_USER"}},
	}}}
	res := rule.Evaluate(b)
	if len(res.Findings) != 1 || res.Findings[0].Facts[0] != "keys: DB_PORT, DB_USER" || len(res.Warnings) != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestSnapshotCollectorsWithoutLogs(t *testing.T) {
	has := func(opts SnapshotOptions) bool {
		for _, c := range snapshotCollectors(opts) {
			if c.Name() == "logs" {
				return true
			}
		}
		return false
	}
	if !has(SnapshotOptions{}) || has(SnapshotOptions{NoLogs: true}) {
		t.Fatal("NoLogs should drop exactly the logs collector")
	}
	if len(snapshotCollectors(SnapshotOptions{NoLogs: true})) != len(snapshotCollectors(SnapshotOptions{}))-1 {
		t.Fatal("NoLogs dropped more than the logs collector")
	}
}

// A cluster-wide collector sees every namespace; a rule compares objects to
// the investigation namespace through ns. CEL reserves "namespace", so a rule
// that uses it is refused with a pointer to ns.
func TestCELRuleSeesInvestigationNamespace(t *testing.T) {
	rule, err := NewCELRule("external-dependents", RuleSpec{
		Resource: "all-services", Severity: model.SeverityInfo, State: "DependsOnThisNamespace",
		When:  "object.metadata.namespace != ns && has(object.spec.externalName) && object.spec.externalName.contains('.' + ns + '.svc')",
		Facts: []string{"object.metadata.namespace + '/' + object.metadata.name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := func(ns, name, external string) map[string]any {
		spec := map[string]any{"type": "ClusterIP"}
		if external != "" {
			spec = map[string]any{"type": "ExternalName", "externalName": external}
		}
		return map[string]any{"kind": "Service", "metadata": map[string]any{"name": name, "namespace": ns}, "spec": spec}
	}
	b := model.EvidenceBundle{Namespace: "holdout", Resources: map[string][]map[string]any{"all-services": {
		svc("shop", "billing-proxy", "billing.holdout.svc.cluster.local"),
		svc("shop", "web", ""),
		svc("holdout", "alias", "billing.holdout.svc.cluster.local"),
		svc("shop", "other", "api.payments.svc.cluster.local"),
	}}}
	res := rule.Evaluate(b)
	if len(res.Findings) != 1 || res.Findings[0].Object.Name != "billing-proxy" || res.Findings[0].Facts[0] != "shop/billing-proxy" {
		t.Fatalf("findings = %+v, warnings = %v", res.Findings, res.Warnings)
	}

	_, err = NewCELRule("old-name", RuleSpec{Resource: "pods", State: "Test", When: "namespace == 'holdout'"})
	if err == nil || !strings.Contains(err.Error(), "use ns for the investigation namespace") {
		t.Fatalf("err = %v, want a pointer to ns", err)
	}
}
