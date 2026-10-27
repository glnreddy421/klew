package kube

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
)

func minimalKubeconfigYAML(contextName, namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: %s
contexts:
- name: %s
  context:
    cluster: c
    user: u
    namespace: %s
clusters:
- name: c
  cluster:
    server: https://127.0.0.1:6443
users:
- name: u
  user: {}
`, contextName, contextName, namespace)
}

func TestPrepareTerminalShellKubeFallbackOnWriteFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "config")
	if err := os.WriteFile(src, []byte(minimalKubeconfigYAML("alpha", "team-a")), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := writeTerminalKubeconfigEncoded
	writeTerminalKubeconfigEncoded = func([]byte) (string, func(), error) {
		return "", nil, fmt.Errorf("disk full")
	}
	t.Cleanup(func() { writeTerminalKubeconfigEncoded = prev })

	sess, err := PrepareTerminalShellKube(src, "alpha", "prod-ns")
	if err != nil {
		t.Fatal(err)
	}
	if sess.KubeconfigPath != src {
		t.Fatalf("kubeconfig path = %q", sess.KubeconfigPath)
	}
	if sess.Isolated || sess.Cleanup != nil || sess.Notice == "" {
		t.Fatal("fallback session must not be isolated and must explain itself")
	}
	// Without a session copy the namespace override cannot apply; report the file's.
	if sess.Context != "alpha" || sess.Namespace != "team-a" {
		t.Fatalf("context/ns = %q / %q", sess.Context, sess.Namespace)
	}
}

func TestPrepareTerminalShellKubeRefusesFallbackToOtherContext(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "config")
	yaml := strings.Replace(minimalKubeconfigYAML("alpha", "team-a"), "clusters:", `- name: beta
  context:
    cluster: c
    user: u
clusters:`, 1)
	if err := os.WriteFile(src, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := writeTerminalKubeconfigEncoded
	writeTerminalKubeconfigEncoded = func([]byte) (string, func(), error) {
		return "", nil, fmt.Errorf("disk full")
	}
	t.Cleanup(func() { writeTerminalKubeconfigEncoded = prev })

	_, err := PrepareTerminalShellKube(src, "beta", "")
	if err == nil || errors.Is(err, ErrKubeconfigUnavailable) {
		t.Fatalf("wrong-cluster fallback must be a hard error, got %v", err)
	}
}

func TestPrepareTerminalShellKubeMissingCurrentContext(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "config")
	yaml := strings.Replace(minimalKubeconfigYAML("alpha", "team-a"), "current-context: alpha", "current-context: ghost", 1)
	if err := os.WriteFile(src, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := PrepareTerminalShellKube(src, "", "")
	if !errors.Is(err, ErrKubeconfigUnavailable) {
		t.Fatalf("dangling current-context: err = %v, want ErrKubeconfigUnavailable", err)
	}
}

func TestSweepStaleTerminalKubeconfigs(t *testing.T) {
	dir := t.TempDir()
	self := os.Getpid()
	files := map[string]bool{ // name → should survive
		fmt.Sprintf("kubeconfig-%d-111", self): true,
		"kubeconfig-424242-222":                true, // alive per stub
		"kubeconfig-515151-333":                false,
		"kubeconfig-legacy":                    true,
		"unrelated":                            true,
	}
	for name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sweepStaleTerminalKubeconfigsIn(dir, func(pid int) bool { return pid == 424242 })
	for name, keep := range files {
		_, err := os.Stat(filepath.Join(dir, name))
		if keep && err != nil {
			t.Fatalf("%s was removed", name)
		}
		if !keep && err == nil {
			t.Fatalf("%s was not removed", name)
		}
	}
}

func TestPrepareTerminalKubeconfigDoesNotMutateSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "config")
	contents := []byte(`apiVersion: v1
kind: Config
current-context: alpha
contexts:
- name: alpha
  context:
    cluster: c
    user: u
    namespace: team-a
- name: beta
  context:
    cluster: c
    user: u
clusters:
- name: c
  cluster:
    server: https://127.0.0.1:6443
users:
- name: u
  user: {}
`)
	if err := os.WriteFile(src, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	sess, err := PrepareTerminalShellKube(src, "beta", "prod-ns")
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cleanup()
	sessionPath := sess.KubeconfigPath

	after, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, contents) {
		t.Fatal("source kubeconfig was modified")
	}

	loaded, err := clientcmd.LoadFromFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CurrentContext != "beta" {
		t.Fatalf("session current-context = %q", loaded.CurrentContext)
	}
	if loaded.Contexts["beta"].Namespace != "prod-ns" {
		t.Fatalf("session namespace = %q", loaded.Contexts["beta"].Namespace)
	}
}

func TestPrepareTerminalShellKubeMissingKubeconfig(t *testing.T) {
	_, err := PrepareTerminalShellKube(filepath.Join(t.TempDir(), "absent"), "", "")
	if !errors.Is(err, ErrKubeconfigUnavailable) {
		t.Fatalf("missing kubeconfig: err = %v, want ErrKubeconfigUnavailable", err)
	}
}

func TestPrepareTerminalShellKubeUsesContextNamespaceByDefault(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "config")
	if err := os.WriteFile(src, []byte(minimalKubeconfigYAML("alpha", "team-a")), 0o600); err != nil {
		t.Fatal(err)
	}
	sess, err := PrepareTerminalShellKube(src, "alpha", "")
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cleanup()
	if sess.Namespace != "team-a" {
		t.Fatalf("namespace = %q, want the context's team-a", sess.Namespace)
	}
}

func TestEnsurePrivateDir(t *testing.T) {
	base := t.TempDir()

	dir := filepath.Join(base, "sessions")
	if err := ensurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}

	loose := filepath.Join(base, "loose")
	if err := os.Mkdir(loose, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(loose, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDir(loose); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(loose)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Fatalf("loose dir perm = %o, want 700", perm)
		}
	}

	link := filepath.Join(base, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := ensurePrivateDir(link); err == nil {
		t.Fatal("a symlinked session dir must be rejected")
	}
}
