package kube

import "testing"

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
