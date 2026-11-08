package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensionSourcesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "extension-sources.json")
	src, err := ReadExtensionSources(path)
	if err != nil || len(src.Folders) != 0 {
		t.Fatalf("missing file should mean no folders: %v %v", src, err)
	}
	own, linked := t.TempDir(), t.TempDir()
	if err := src.LinkFolder(linked, own); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{linked, own, "relative/path", filepath.Join(linked, "nope")} {
		if err := src.LinkFolder(bad, own); err == nil {
			t.Errorf("LinkFolder(%q) should fail", bad)
		}
	}
	if err := WriteExtensionSources(path, src); err != nil {
		t.Fatal(err)
	}
	back, err := ReadExtensionSources(path)
	if err != nil || len(back.Folders) != 1 || back.Folders[0] != linked {
		t.Fatalf("read back %v %v", back, err)
	}
	if err := back.UnlinkFolder(linked); err != nil || len(back.Folders) != 0 {
		t.Fatalf("unlink: %v %v", back, err)
	}
	if err := back.UnlinkFolder(linked); err == nil {
		t.Error("unlinking twice should fail")
	}
}

func TestLoadExtensionDirsReadsLinkedFoldersInPlace(t *testing.T) {
	own, linked := t.TempDir(), t.TempDir()
	writeExtension(t, own, "c.yaml", `apiVersion: klew.dev/v1alpha1
kind: Collector
metadata: {name: certificates}
spec: {group: cert-manager.io, resource: certificates}
`)
	writeExtension(t, linked, "r.yaml", `apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: cert-rule}
spec: {resource: certificates, when: "true", state: X}
---
apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: bad}
spec: {resource: certificates, when: "object.", state: X}
`)
	missing := filepath.Join(t.TempDir(), "gone")
	reg := LoadExtensionDirs(own, []string{linked, missing})
	if got := reg.UserRuleNames(); len(got) != 1 || got[0] != "cert-rule" {
		t.Fatalf("rules = %v", got)
	}
	status := map[string]ExtensionEntry{}
	for _, e := range reg.Entries {
		status[e.Name] = e
	}
	if e := status["certificates"]; e.Status != ExtensionLoaded || e.Folder != own {
		t.Errorf("collector entry %+v", e)
	}
	if e := status["cert-rule"]; e.Status != ExtensionLoaded || e.Folder != linked {
		t.Errorf("rule entry %+v", e)
	}
	if e := status["bad"]; e.Status != ExtensionSkipped || e.Reason == "" || e.Document != 2 {
		t.Errorf("bad entry %+v", e)
	}
	if !strings.Contains(strings.Join(reg.Warnings, "\n"), "linked folder "+missing+" was not found") {
		t.Errorf("warnings %v", reg.Warnings)
	}
	// An edit in the linked folder applies on the next load.
	if err := os.Remove(filepath.Join(linked, "r.yaml")); err != nil {
		t.Fatal(err)
	}
	if got := LoadExtensionDirs(own, []string{linked}).UserRuleNames(); len(got) != 0 {
		t.Errorf("after removing the file, rules = %v", got)
	}
}

func TestSkippedResourceRuleIsMarkedInEntries(t *testing.T) {
	dir := t.TempDir()
	writeExtension(t, dir, "r.yaml", `apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: typo}
spec: {resource: certifcates, when: "true", state: X}
`)
	reg := LoadExtensions(dir)
	if len(reg.Entries) != 1 || reg.Entries[0].Status != ExtensionSkipped || !strings.Contains(reg.Entries[0].Reason, "not a collector") {
		t.Fatalf("entries %+v", reg.Entries)
	}
}
