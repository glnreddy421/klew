package kube

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveKubectlCustom(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "kubectl")
	if err := os.WriteFile(custom, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	SetKubectlOptions(true, custom, true)
	path, source := ResolveKubectl()
	if path != custom || source != "custom" {
		t.Fatalf("ResolveKubectl() = (%q, %q), want custom", path, source)
	}
	SetKubectlOptions(true, "", true)
}

func TestResolveKubectlSystemWhenBundledMissing(t *testing.T) {
	SetKubectlOptions(false, "", true)
	path, source := ResolveKubectl()
	if path == "" {
		t.Skip("kubectl not on PATH in test environment")
	}
	if source != "system" {
		t.Fatalf("source = %q, want system", source)
	}
	SetKubectlOptions(true, "", true)
}
