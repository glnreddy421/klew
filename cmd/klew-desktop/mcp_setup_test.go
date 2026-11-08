package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindStandalone(t *testing.T) {
	notOnPath := func(string) (string, error) { return "", errors.New("not found") }

	dir := t.TempDir()
	if got := findStandalone(notOnPath, []string{dir}, ""); got != "" {
		t.Fatalf("empty dir: got %q", got)
	}

	bin := filepath.Join(dir, "klew")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findStandalone(notOnPath, []string{dir}, ""); got != "" {
		t.Fatalf("non-executable file should not count: got %q", got)
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findStandalone(notOnPath, nil, dir); got != bin {
		t.Fatalf("go bin: got %q, want %q", got, bin)
	}

	onPath := func(string) (string, error) { return "/somewhere/klew", nil }
	if got := findStandalone(onPath, []string{dir}, ""); got != "/somewhere/klew" {
		t.Fatalf("PATH wins: got %q", got)
	}
}
