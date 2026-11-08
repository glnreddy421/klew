package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/glnreddy421/klew/internal/kube"
)

// ExtensionSources lists the folders of extensions a user linked. Klew reads
// them in place, so edits there apply on the next reload. It is stored as
// extension-sources.json in Klew's data directory, which the desktop app and
// the klew command both read.
type ExtensionSources struct {
	Folders []string `json:"folders"`
}

// ExtensionSourcesPath is where the linked folders are stored, or "" when
// Klew has no data directory.
func ExtensionSourcesPath() string {
	if dir := kube.DataDir(); dir != "" {
		return filepath.Join(dir, "extension-sources.json")
	}
	return ""
}

// ReadExtensionSources returns the linked folders. A missing file means none.
func ReadExtensionSources(path string) (ExtensionSources, error) {
	var s ExtensionSources
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return ExtensionSources{}, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// WriteExtensionSources saves the linked folders atomically.
func WriteExtensionSources(path string, s ExtensionSources) error {
	if path == "" {
		return errors.New("klew has no data directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LinkFolder adds an absolute folder to s. It refuses Klew's own extensions
// folder (always read) and a folder already linked.
func (s *ExtensionSources) LinkFolder(folder, ownDir string) error {
	folder = filepath.Clean(folder)
	if !filepath.IsAbs(folder) {
		return fmt.Errorf("%s is not an absolute path", folder)
	}
	if ownDir != "" && folder == filepath.Clean(ownDir) {
		return errors.New("that is Klew's own extensions folder, which is always read")
	}
	if slices.Contains(s.Folders, folder) {
		return fmt.Errorf("%s is already linked", folder)
	}
	info, err := os.Stat(folder)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a folder", folder)
	}
	s.Folders = append(s.Folders, folder)
	return nil
}

// UnlinkFolder removes folder from s. Its files are left untouched.
func (s *ExtensionSources) UnlinkFolder(folder string) error {
	folder = filepath.Clean(folder)
	i := slices.Index(s.Folders, folder)
	if i < 0 {
		return fmt.Errorf("%s is not linked", folder)
	}
	s.Folders = slices.Delete(s.Folders, i, i+1)
	return nil
}

// LoadConfiguredExtensions loads Klew's own extensions folder and every linked
// folder: what the desktop app and the klew command use by default.
func LoadConfiguredExtensions() *Registry {
	src, err := ReadExtensionSources(ExtensionSourcesPath())
	reg := LoadExtensionDirs(DefaultExtensionsDir(), src.Folders)
	if err != nil {
		reg.Warnings = append(reg.Warnings, fmt.Sprintf("extensions: linked folders could not be read: %v", err))
	}
	return reg
}
