package kube

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// BootstrapLoginShellEnv merges credential-related variables from the user's
// login shell into the process environment. macOS GUI apps start with a minimal
// PATH and without KUBECONFIG / cloud CLI session vars; the embedded terminal
// works because it execs a login shell (zsh -l), which loads the same profile
// scripts as Terminal, Lens, and K9s.
func BootstrapLoginShellEnv() {
	if runtime.GOOS == "windows" {
		return
	}
	env, err := loginShellEnvironment()
	if err != nil {
		return
	}
	for key, val := range env {
		if val == "" || !mergeLoginShellVar(key) {
			continue
		}
		os.Setenv(key, val)
	}
}

func mergeLoginShellVar(key string) bool {
	switch key {
	case "PATH", "KUBECONFIG", "HOME", "USER", "USERPROFILE", "USE_GKE_GCLOUD_AUTH_PLUGIN":
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

func loginShellEnvironment() (map[string]string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	cmd := exec.Command(shell, "-lc", "env -0")
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
