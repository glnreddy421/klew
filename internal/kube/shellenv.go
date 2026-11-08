package kube

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// loginShellTimeout bounds reading the login shell's environment: it runs
// before anything else starts, so a slow or hanging profile must not block it.
const loginShellTimeout = 10 * time.Second

// BootstrapLoginShellEnv merges credential-related variables from the user's
// login shell into the process environment. macOS GUI apps start with a minimal
// PATH and without KUBECONFIG / cloud CLI session vars; the embedded terminal
// works because it execs a login shell (zsh -l), which loads the same profile
// scripts as Terminal, Lens, and K9s.
func BootstrapLoginShellEnv() {
	if runtime.GOOS == "windows" {
		return
	}
	env, err := loginShellEnvironment(loginShellTimeout)
	if err != nil {
		slog.Warn("login shell environment unavailable; using the process environment", "err", err)
		return
	}
	for key, val := range env {
		if val == "" || !mergeLoginShellVar(key) {
			continue
		}
		_ = os.Setenv(key, val) // best effort: a var that fails to set keeps its current value
	}
}

func mergeLoginShellVar(key string) bool {
	switch key {
	case "PATH", "KUBECONFIG", "HOME", "USER", "USERPROFILE", "USE_GKE_GCLOUD_AUTH_PLUGIN",
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy":
		return true
	}
	if strings.HasPrefix(key, "AWS_") {
		return true
	}
	if strings.HasPrefix(key, "AZURE") {
		return true
	}
	if strings.HasPrefix(key, "GOOGLE_") {
		return true
	}
	if strings.HasPrefix(key, "CLOUDSDK_") {
		return true
	}
	return strings.HasPrefix(key, "KUBE")
}

func loginShellEnvironment(timeout time.Duration) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	cmd := exec.CommandContext(ctx, shell, "-lc", "env -0") //nolint:gosec // G702: runs the user's own login shell ($SHELL) with a fixed script to read its environment
	// A background process from the profile can hold stdout open after the shell dies.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseNullTerminatedEnv(out), nil
}

func parseNullTerminatedEnv(out []byte) map[string]string {
	result := make(map[string]string)
	for _, part := range bytes.Split(out, []byte{0}) {
		if len(part) == 0 {
			continue
		}
		key, val, ok := strings.Cut(string(part), "=")
		if !ok || key == "" {
			continue
		}
		result[key] = val
	}
	return result
}
