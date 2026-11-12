package kube

import (
	"os"
	"path/filepath"
	"testing"
)

const twoContextKubeconfig = `apiVersion: v1
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
    namespace: team-b
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

func writeKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(twoContextKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewFromFlagsUsesExplicitContext(t *testing.T) {
	c, err := NewFromFlags(writeKubeconfig(t), "beta", "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Context != "beta" || c.Cluster != "c2" || c.User != "u2" {
		t.Fatalf("got context=%q cluster=%q user=%q, want beta/c2/u2", c.Context, c.Cluster, c.User)
	}
	if c.ContextNamespace != "team-b" || c.Namespace != "team-b" {
		t.Fatalf("got namespace=%q contextNamespace=%q, want team-b", c.Namespace, c.ContextNamespace)
	}
	if c.Config.Host != "https://127.0.0.1:6444" {
		t.Fatalf("got host %q, want the beta cluster", c.Config.Host)
	}
}

func TestNewFromFlagsDefaultsToCurrentContext(t *testing.T) {
	c, err := NewFromFlags(writeKubeconfig(t), "", "ns1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Context != "alpha" || c.Cluster != "c1" || c.Namespace != "ns1" || c.ContextNamespace != "team-a" {
		t.Fatalf("got %q/%q ns=%q ctxns=%q, want alpha/c1 ns1 team-a", c.Context, c.Cluster, c.Namespace, c.ContextNamespace)
	}
}

func TestNewFromFlagsUnknownContext(t *testing.T) {
	if _, err := NewFromFlags(writeKubeconfig(t), "missing", ""); err == nil {
		t.Fatal("want an error for a context that is not in the kubeconfig")
	}
}
