package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/glnreddy421/klew/v2/internal/engine"
	"github.com/glnreddy421/klew/v2/internal/model"
)

func TestListExtensionsReportsLoadedAndSkipped(t *testing.T) {
	dir := t.TempDir()
	examples, err := os.ReadFile(filepath.Join("..", "..", "extensions-examples", "04-cert-manager.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "certs.yaml"), examples, 0o600); err != nil {
		t.Fatal(err)
	}
	broken := "apiVersion: klew.dev/v1alpha1\nkind: Rule\nmetadata:\n  name: broken\nspec:\n  resource: pods\n  when: \"object.status ==\"\n  state: Broken\n"
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	engine.SetRegistry(engine.LoadExtensions(dir))
	t.Cleanup(func() { engine.SetRegistry(nil) })

	s := testServer(fake.NewSimpleClientset(), nil)
	s.opts.ExtensionFolders = []string{dir, filepath.Join(dir, "missing")}
	_, out := call(t, connect(t, s), "klew_list_extensions", nil)

	folders := out["folders"].([]any)
	if len(folders) != 2 || folders[0].(map[string]any)["exists"] != true || folders[1].(map[string]any)["exists"] != false {
		t.Errorf("folders: %v", folders)
	}
	collectors := out["collectors"].([]any)
	if len(collectors) != 1 {
		t.Fatalf("collectors: %v", collectors)
	}
	if c := collectors[0].(map[string]any); c["name"] != "certificates" || c["group"] != "cert-manager.io" || c["file"] != "certs.yaml" {
		t.Errorf("collector: %v", c)
	}
	rules := map[string]map[string]any{}
	for _, r := range out["rules"].([]any) {
		r := r.(map[string]any)
		rules[r["name"].(string)] = r
	}
	if r := rules["certificate-not-ready"]; r == nil || r["state"] != "CertificateNotReady" || r["severity"] != "high" || r["file"] != "certs.yaml" {
		t.Errorf("rules: %v", rules)
	}
	if _, ok := rules["broken"]; ok {
		t.Error("a rule that failed to compile is listed as loaded")
	}
	skipped, _ := out["skipped"].([]any)
	if len(skipped) != 1 || skipped[0].(map[string]any)["file"] != "broken.yaml" || skipped[0].(map[string]any)["reason"] == "" {
		t.Errorf("skipped: %v", skipped)
	}
	if out["note"] != nil {
		t.Errorf("note with extensions loaded: %v", out["note"])
	}
}

func TestListExtensionsWithNoneLoadedSaysWhereToPutThem(t *testing.T) {
	engine.SetRegistry(engine.DefaultRegistry())
	t.Cleanup(func() { engine.SetRegistry(nil) })
	_, out := call(t, connect(t, testServer(fake.NewSimpleClientset(), nil)), "klew_list_extensions", nil)
	if len(out["rules"].([]any)) != 0 || len(out["collectors"].([]any)) != 0 {
		t.Errorf("built-in rules or collectors listed as the user's: %v", out)
	}
	if out["note"] == nil {
		t.Error("no note telling the user where to put extensions")
	}
}

func TestNoLogSamplesDropsLogExcerptsFromProblems(t *testing.T) {
	p := model.Problem{Object: "Deployment/notifier", State: "ErrorsWhileReady", Facts: []string{
		"container notifier is Ready but 20 of its last 40 log lines are errors",
		model.LogExcerptPrefix + "ERROR failed to deliver audit record",
	}}
	if got := (shaper{}).problem(p).Facts; len(got) != 2 {
		t.Errorf("with log samples on, facts = %q", got)
	}
	got := shaper{noLogSamples: true}.problem(p).Facts
	if len(got) != 1 || strings.HasPrefix(got[0], model.LogExcerptPrefix) {
		t.Errorf("with -no-log-samples, facts = %q, want only the count", got)
	}
}
