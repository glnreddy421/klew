package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
)

// ExtensionsView is what Settings → Extensions shows: Klew's own folder, the
// folders the user linked, and every collector and rule with its status.
type ExtensionsView struct {
	OwnFolder  string                  `json:"ownFolder"`
	Linked     []LinkedFolder          `json:"linked"`
	Entries    []engine.ExtensionEntry `json:"entries"`
	Problems   []string                `json:"problems"`
	Collectors int                     `json:"collectors"`
	Rules      int                     `json:"rules"`
}

// LinkedFolder is a folder read in place; Found is false when it is missing.
type LinkedFolder struct {
	Path  string `json:"path"`
	Found bool   `json:"found"`
}

// ExtensionTestResult is every user rule dry-run against one namespace.
type ExtensionTestResult struct {
	Namespace string          `json:"namespace"`
	Context   string          `json:"context"`
	Runs      []engine.DryRun `json:"runs"`
	// Warnings are collection warnings, such as permissions Klew lacked.
	Warnings []string `json:"warnings"`
}

// ExtensionsStatus describes the extensions in use now.
func (a *App) ExtensionsStatus() ExtensionsView {
	return extensionsView(engine.ActiveRegistry())
}

// ReloadExtensions reads every extensions folder again. New investigations
// and the next refresh of a running one use the result; no restart needed.
func (a *App) ReloadExtensions() ExtensionsView {
	reg := engine.LoadConfiguredExtensions()
	engine.SetRegistry(reg)
	return extensionsView(reg)
}

// AddExtensionFile copies a YAML file the user picks into Klew's own
// extensions folder, after checking that at least one document in it loads.
func (a *App) AddExtensionFile() (ExtensionsView, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Add a collector or rule file",
		Filters: []runtime.FileFilter{{DisplayName: "Klew extensions (*.yaml, *.yml)", Pattern: "*.yaml;*.yml"}},
	})
	if err != nil || path == "" {
		return a.ExtensionsStatus(), err
	}
	if err := addExtensionFile(path, engine.DefaultExtensionsDir()); err != nil {
		return a.ExtensionsStatus(), err
	}
	return a.ReloadExtensions(), nil
}

// RemoveExtensionFile deletes a file from Klew's own extensions folder. Files
// in linked folders belong to the user and are never deleted; unlink instead.
func (a *App) RemoveExtensionFile(path string) (ExtensionsView, error) {
	if !inFolder(path, engine.DefaultExtensionsDir()) {
		return a.ExtensionsStatus(), errors.New("only files Klew copied into its own extensions folder can be removed here; unlink the folder instead")
	}
	if err := os.Remove(path); err != nil {
		return a.ExtensionsStatus(), err
	}
	return a.ReloadExtensions(), nil
}

// LinkExtensionFolder asks for a folder and reads it in place from now on,
// in the desktop app and the klew command alike.
func (a *App) LinkExtensionFolder() (ExtensionsView, error) {
	folder, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Link a folder of collectors and rules",
	})
	if err != nil || folder == "" {
		return a.ExtensionsStatus(), err
	}
	if err := editSources(func(s *engine.ExtensionSources) error {
		return s.LinkFolder(folder, engine.DefaultExtensionsDir())
	}); err != nil {
		return a.ExtensionsStatus(), err
	}
	return a.ReloadExtensions(), nil
}

// UnlinkExtensionFolder stops reading a linked folder. Its files are kept.
func (a *App) UnlinkExtensionFolder(folder string) (ExtensionsView, error) {
	if err := editSources(func(s *engine.ExtensionSources) error { return s.UnlinkFolder(folder) }); err != nil {
		return a.ExtensionsStatus(), err
	}
	return a.ReloadExtensions(), nil
}

// OpenExtensionsFolder opens a folder in the file manager, creating Klew's
// own extensions folder first if it does not exist yet.
func (a *App) OpenExtensionsFolder(folder string) error {
	if folder == "" {
		folder = engine.DefaultExtensionsDir()
	}
	if folder == engine.DefaultExtensionsDir() {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			return err
		}
	}
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, "file://"+folder)
	}
	return nil
}

// TestExtensions dry-runs every user rule against a namespace of the selected
// cluster, read once without logs. It never changes the cluster.
func (a *App) TestExtensions(namespace string) (ExtensionTestResult, error) {
	if namespace == "" {
		return ExtensionTestResult{}, errors.New("pick a namespace to test against")
	}
	a.mu.Lock()
	cluster := a.cluster
	a.mu.Unlock()
	client, err := kube.NewFromFlags(cluster.KubeconfigPath, cluster.SelectedContext, namespace)
	if err != nil {
		return ExtensionTestResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.baseContext(), 2*time.Minute)
	defer cancel()
	bundle, _, err := engine.CollectSnapshot(ctx, client, engine.SnapshotOptions{Namespace: namespace, NoLogs: true})
	if err != nil {
		return ExtensionTestResult{}, err
	}
	return ExtensionTestResult{
		Namespace: namespace,
		Context:   client.Context,
		Runs:      engine.ActiveRegistry().DryRunAll(bundle),
		Warnings:  bundle.Warnings,
	}, nil
}

func extensionsView(reg *engine.Registry) ExtensionsView {
	v := ExtensionsView{
		OwnFolder: engine.DefaultExtensionsDir(),
		Entries:   append([]engine.ExtensionEntry{}, reg.Entries...),
		Problems:  append([]string{}, reg.Warnings...),
		Linked:    []LinkedFolder{},
	}
	src, err := engine.ReadExtensionSources(engine.ExtensionSourcesPath())
	if err != nil {
		v.Problems = append(v.Problems, err.Error())
	}
	for _, f := range src.Folders {
		_, statErr := os.Stat(f)
		v.Linked = append(v.Linked, LinkedFolder{Path: f, Found: statErr == nil})
	}
	for _, e := range reg.Entries {
		if e.Status != engine.ExtensionLoaded {
			continue
		}
		switch e.Kind {
		case "Collector":
			v.Collectors++
		case "Rule":
			v.Rules++
		}
	}
	return v
}

func editSources(edit func(*engine.ExtensionSources) error) error {
	path := engine.ExtensionSourcesPath()
	src, err := engine.ReadExtensionSources(path)
	if err != nil {
		return err
	}
	if err := edit(&src); err != nil {
		return err
	}
	return engine.WriteExtensionSources(path, src)
}

// addExtensionFile copies src into dir if at least one of its documents
// loads. It never overwrites a different file of the same name.
func addExtensionFile(src, dir string) error {
	if dir == "" {
		return errors.New("klew has no data directory")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := checkExtensionData(filepath.Base(src), data); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(dir, filepath.Base(src))
	if existing, err := os.ReadFile(dst); err == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("Klew already has a file named %s; remove it first or rename yours", filepath.Base(src))
	}
	return os.WriteFile(dst, data, 0o600)
}

// checkExtensionData loads data on its own and fails when no document in it
// loads, naming each reason.
func checkExtensionData(name string, data []byte) error {
	tmp, err := os.MkdirTemp("", "klew-ext-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, name), data, 0o600); err != nil {
		return err
	}
	reg := engine.LoadExtensions(tmp)
	var reasons []string
	for _, e := range reg.Entries {
		if e.Status == engine.ExtensionLoaded {
			return nil
		}
		reasons = append(reasons, fmt.Sprintf("document %d: %s", e.Document, e.Reason))
	}
	if len(reasons) == 0 {
		return fmt.Errorf("%s has no Collector or Rule documents", name)
	}
	return fmt.Errorf("nothing in %s loads: %s", name, strings.Join(reasons, "; "))
}

func inFolder(path, folder string) bool {
	if path == "" || folder == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(folder), filepath.Clean(path))
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !strings.Contains(rel, string(filepath.Separator))
}
