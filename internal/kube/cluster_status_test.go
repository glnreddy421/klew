package kube

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectClusterStatusNoContext(t *testing.T) {
	st := CollectClusterStatus(context.Background(), "", "")
	if st.Available {
		t.Fatal("expected unavailable without context")
	}
	if st.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestCachedClusterVersionUsesCacheOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	// Unroutable server: any network call would hang or fail, the cache path must not touch it.
	cfg := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://10.255.255.1:6443
contexts:
- name: alpha
  context:
    cluster: c
    user: u
users:
- name: u
  user:
    token: x
current-context: alpha
`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if v := CachedClusterVersion(path, "alpha"); v != "" {
		t.Fatalf("expected empty version without cache, got %q", v)
	}
	client, err := NewFromFlags(path, "alpha", "")
	if err != nil {
		t.Fatal(err)
	}
	globalClusterStatusCache.set(path, client, ClusterStatus{KubernetesVersion: "v1.30.2"})
	t.Cleanup(func() { InvalidateClusterStatus(path, client) })
	if v := CachedClusterVersion(path, "alpha"); v != "v1.30.2" {
		t.Fatalf("expected cached version, got %q", v)
	}
	if v := CachedClusterVersion(path, ""); v != "" {
		t.Fatalf("expected empty version without context, got %q", v)
	}
}

func TestCollectClusterStatusInvalidKubeconfig(t *testing.T) {
	st := CollectClusterStatus(context.Background(), "/nonexistent/kubeconfig", "alpha")
	if st.Available {
		t.Fatal("expected unavailable for invalid kubeconfig")
	}
	if st.Error == "" {
		t.Fatal("expected error message")
	}
}
