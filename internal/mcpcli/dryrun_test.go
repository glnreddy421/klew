package mcpcli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/engine"
)

const testExtensions = `apiVersion: klew.dev/v1alpha1
kind: Collector
metadata: {name: certificates}
spec: {group: cert-manager.io, resource: certificates}
---
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: cert-not-ready}
spec:
  resource: certificates
  when: "object.status.conditions.exists(c, c.type == 'Ready' && c.status != 'True')"
  state: CertNotReady
  facts: ["object.metadata.name + ' is not ready'"]
---
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: typo}
spec: {resource: certifcates, when: "true", state: Typo}
`

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckExtensionsReportsProblems(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "e.yaml", testExtensions)
	var out bytes.Buffer
	if checkExtensions(&out, []string{dir}, engine.LoadExtensions(dir)) {
		t.Fatal("a skipped rule should make the check fail")
	}
	for _, want := range []string{"Collectors loaded: certificates", "Rules loaded:      cert-not-ready", `Did you mean "certificates"?`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestDryRunAgainstSavedObjects(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "e.yaml", testExtensions)
	objects := writeFile(t, t.TempDir(), "certs.yaml", `apiVersion: v1
kind: List
items:
- kind: Certificate
  metadata: {name: web-tls, namespace: shop}
  status: {conditions: [{type: Ready, status: "False"}]}
- kind: Certificate
  metadata: {name: api-tls, namespace: shop}
  status: {conditions: [{type: Ready, status: "True"}]}
- kind: Certificate
  metadata: {name: fresh, namespace: shop}
`)
	var out bytes.Buffer
	err := dryRun(&out, engine.LoadExtensions(dir), dryRunOptions{Rule: "cert-not-ready", ObjectFile: objects})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"MATCH    Certificate/web-tls", "fact: web-tls is not ready",
		"no match Certificate/api-tls", "ERROR    Certificate/fresh",
		"1 of 3 matched, 1 could not be evaluated", "warning: rule cert-not-ready: 1 objects could not be evaluated",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if err := dryRun(&out, engine.LoadExtensions(dir), dryRunOptions{Rule: "cert-not-ready"}); err == nil {
		t.Error("a dry run needs -object or -namespace")
	}
}

func TestDryRunAllNeedsANamespace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "e.yaml", testExtensions)
	reg := engine.LoadExtensions(dir)
	var out bytes.Buffer
	if _, err := dryRunAll(&out, reg, dryRunOptions{}); err == nil || !strings.Contains(err.Error(), "needs -namespace") {
		t.Errorf("no namespace: %v", err)
	}
	if _, err := dryRunAll(&out, reg, dryRunOptions{ObjectFile: "x.yaml", Namespace: "ns"}); err == nil || !strings.Contains(err.Error(), "-dry-run RULE -object FILE") {
		t.Errorf("object file: %v", err)
	}
	if _, err := dryRunAll(&out, engine.LoadExtensions(t.TempDir()), dryRunOptions{Namespace: "ns"}); err == nil || !strings.Contains(err.Error(), "no user rules") {
		t.Errorf("no rules: %v", err)
	}
}
