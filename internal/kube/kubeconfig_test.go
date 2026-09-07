package kube

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestLoadKubeConfigSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `apiVersion: v1
kind: Config
current-context: alpha
contexts:
- name: alpha
  context:
    cluster: c1
    user: u1
    namespace: team-a
- name: beta
  context:
    cluster: c2
    user: u2
clusters:
- name: c1
  cluster:
    server: https://127.0.0.1:6443
- name: c2
  cluster:
    server: https://127.0.0.1:6444
users:
- name: u1
- name: u2
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := LoadKubeConfigSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.CurrentContext != "alpha" {
		t.Fatalf("current context = %q", st.CurrentContext)
	}
	if len(st.Contexts) != 2 {
		t.Fatalf("contexts = %d", len(st.Contexts))
	}
	if st.SelectedNamespace != "team-a" {
		t.Fatalf("namespace = %q", st.SelectedNamespace)
	}
}

func TestPickNamespace(t *testing.T) {
	if got := pickNamespace([]string{"kube-system", "default", "klew-lab"}, "klew-lab"); got != "klew-lab" {
		t.Fatalf("preferred present: %q", got)
	}
	if got := pickNamespace([]string{"kube-system", "default"}, "klew-lab"); got != "default" {
		t.Fatalf("preferred missing falls back to default: %q", got)
	}
}

func TestIsNamespaceListRestricted(t *testing.T) {
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "denied", errors.New("denied"))
	if !isNamespaceListRestricted(forbidden) {
		t.Fatal("expected forbidden to be restricted")
	}
	if isNamespaceListRestricted(errors.New("connection refused")) {
		t.Fatal("expected connection error to be hard failure")
	}
}

func TestConfigureLoadingRulesMultiPath(t *testing.T) {
	multi := "/a/config" + string(os.PathListSeparator) + "/b/config"
	rules := configureLoadingRules(multi)
	if rules.ExplicitPath != "" {
		t.Fatalf("multi-path kubeconfig must not set ExplicitPath, got %q", rules.ExplicitPath)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	rules = configureLoadingRules(path)
	if rules.ExplicitPath != path {
		t.Fatalf("single path ExplicitPath = %q", rules.ExplicitPath)
	}
}
