package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/version"
)

// MCPSetupInfo is what the Set Up MCP dialog needs to show a working client
// configuration for this machine.
type MCPSetupInfo struct {
	// Executable is this app's binary; "<Executable> mcp" serves MCP.
	Executable string `json:"executable"`
	// Translocated is true when macOS runs the app from a temporary or
	// disk-image path that will not exist later, so a config pointing at it
	// would break. The app must be moved to Applications first.
	Translocated bool   `json:"translocated"`
	Version      string `json:"version"`
	// HistoryDir is where this app records events for klew_event_history.
	HistoryDir string `json:"historyDir"`
	// StandalonePath is an installed klew command, "" when none was found.
	StandalonePath string `json:"standalonePath"`
}

// standaloneDirs are checked as well as PATH: apps started from the Dock get
// a minimal PATH without Homebrew's or Go's bin folders.
var standaloneDirs = []string{"/opt/homebrew/bin", "/usr/local/bin", "/home/linuxbrew/.linuxbrew/bin"}

// MCPSetupInfo reports this binary and any installed klew command. Read-only:
// it changes nothing on the machine.
func (a *App) MCPSetupInfo() MCPSetupInfo {
	info := MCPSetupInfo{Version: version.Version}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		info.Executable = exe
		info.Translocated = strings.Contains(exe, "/AppTranslocation/") || strings.HasPrefix(exe, "/Volumes/")
	}
	if dir := kube.DataDir(); dir != "" {
		info.HistoryDir = filepath.Join(dir, "history")
	}
	info.StandalonePath = findStandalone(exec.LookPath, standaloneDirs, goBinDir())
	return info
}

func goBinDir() string {
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		return gobin
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return filepath.Join(gopath, "bin")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "go", "bin")
	}
	return ""
}

// findStandalone looks for klew on PATH, then in dirs, then in goBin.
func findStandalone(lookPath func(string) (string, error), dirs []string, goBin string) string {
	if p, err := lookPath("klew"); err == nil {
		return p
	}
	if goBin != "" {
		dirs = append(dirs, goBin)
	}
	for _, d := range dirs {
		p := filepath.Join(d, "klew")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}
