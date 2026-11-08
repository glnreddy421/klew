package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodRule = `apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: pvc-pending}
spec: {resource: persistentvolumeclaims, when: "true", state: ClaimPending}
`

func TestAddExtensionFileCopiesOnlyLoadableFiles(t *testing.T) {
	srcDir, own := t.TempDir(), filepath.Join(t.TempDir(), "extensions")
	good := filepath.Join(srcDir, "pvc.yaml")
	if err := os.WriteFile(good, []byte(goodRule), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := addExtensionFile(good, own); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(own, "pvc.yaml")); err != nil {
		t.Fatalf("not copied: %v", err)
	}
	if err := addExtensionFile(good, own); err != nil {
		t.Errorf("adding the same file again should be a no-op: %v", err)
	}
	other := filepath.Join(t.TempDir(), "pvc.yaml")
	if err := os.WriteFile(other, []byte(strings.Replace(goodRule, "pvc-pending", "other", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := addExtensionFile(other, own); err == nil || !strings.Contains(err.Error(), "already has a file named pvc.yaml") {
		t.Errorf("different file with the same name: %v", err)
	}
	bad := filepath.Join(srcDir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata: {name: x}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := addExtensionFile(bad, own); err == nil || !strings.Contains(err.Error(), "nothing in bad.yaml loads") {
		t.Errorf("unloadable file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(own, "bad.yaml")); err == nil {
		t.Error("an unloadable file must not be copied")
	}
}

func TestInFolderOnlyAllowsDirectChildren(t *testing.T) {
	own := "/data/Klew/extensions"
	for path, want := range map[string]bool{
		"/data/Klew/extensions/a.yaml":         true,
		"/data/Klew/extensions/../secret.yaml": false,
		"/data/Klew/extensions/sub/a.yaml":     false,
		"/data/Klew/extensions":                false,
		"/home/me/rules/a.yaml":                false,
		"":                                     false,
	} {
		if got := inFolder(path, own); got != want {
			t.Errorf("inFolder(%q) = %v, want %v", path, got, want)
		}
	}
}
