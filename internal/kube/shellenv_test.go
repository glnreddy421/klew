package kube

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseNullTerminatedEnv(t *testing.T) {
	raw := []byte("PATH=/bin:/usr/bin\x00KUBECONFIG=/a:/b\x00EMPTY=\x00")
	env := parseNullTerminatedEnv(raw)
	if env["PATH"] != "/bin:/usr/bin" {
		t.Fatalf("PATH = %q", env["PATH"])
	}
	if env["KUBECONFIG"] != "/a:/b" {
		t.Fatalf("KUBECONFIG = %q", env["KUBECONFIG"])
	}
	if _, ok := env["EMPTY"]; !ok {
		t.Fatal("expected EMPTY key")
	}
}

func TestMergeLoginShellVar(t *testing.T) {
	for _, key := range []string{"PATH", "KUBECONFIG", "AWS_PROFILE", "AWS_REGION", "GOOGLE_APPLICATION_CREDENTIALS", "USE_GKE_GCLOUD_AUTH_PLUGIN"} {
		if !mergeLoginShellVar(key) {
			t.Fatalf("%q should merge from login shell", key)
		}
	}
	if mergeLoginShellVar("OLDPWD") {
		t.Fatal("OLDPWD should not merge")
	}
}

func TestLoginShellEnvironmentTimesOut(t *testing.T) {
	// The background sleep keeps stdout open after the shell is killed.
	shell := filepath.Join(t.TempDir(), "slow-shell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nsleep 30 &\nwait\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", shell)
	start := time.Now()
	if _, err := loginShellEnvironment(100 * time.Millisecond); err == nil {
		t.Fatal("want an error from a shell that never finishes")
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("took %v", d)
	}
}
