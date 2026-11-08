package main

import (
	"errors"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"github.com/glnreddy421/klew/v2/internal/kube"
)

func envValue(env []string, key string) (string, bool) {
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok && k == key {
			return v, true
		}
	}
	return "", false
}

func TestBuildClusterShellCommandRunsLoginShellDirectly(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("unix shell layout")
	}
	cmd, err := buildClusterShellCommand("/bin/zsh", kube.TerminalShellKube{
		KubeconfigPath: "/tmp/k",
		Context:        "ctx",
		Namespace:      "ns",
		Isolated:       true,
	}, "/opt/kubectl")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cmd.Args, []string{"/bin/zsh", "-l"}) {
		t.Fatalf("args = %v", cmd.Args)
	}
	if v, _ := envValue(cmd.Env, "KUBECONFIG"); v != "/tmp/k" {
		t.Fatalf("KUBECONFIG = %q", v)
	}
	if v, _ := envValue(cmd.Env, "KLEW_CONTEXT"); v != "ctx" {
		t.Fatalf("KLEW_CONTEXT = %q", v)
	}
	if v, _ := envValue(cmd.Env, "PATH"); !strings.HasPrefix(v, "/opt/kubectl") {
		t.Fatalf("PATH = %q", v)
	}
}

func TestBuildClusterShellCommandPlainShellKeepsUserKubeconfig(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("unix shell layout")
	}
	t.Setenv("KUBECONFIG", "/home/me/.kube/config")
	tk := kube.PlainTerminalShellKube(errors.New("kubeconfig has no current context"))
	cmd, err := buildClusterShellCommand("/bin/bash", tk, "")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := envValue(cmd.Env, "KUBECONFIG"); v != "/home/me/.kube/config" {
		t.Fatalf("plain shell must not override KUBECONFIG, got %q", v)
	}
	if _, ok := envValue(cmd.Env, "KLEW_CONTEXT"); ok {
		t.Fatal("plain shell must not set KLEW_CONTEXT")
	}
	if tk.Notice == "" {
		t.Fatal("plain shell should carry a notice")
	}
}

func TestBuildClusterShellCommandRequiresShell(t *testing.T) {
	if _, err := buildClusterShellCommand("  ", kube.TerminalShellKube{}, ""); err == nil {
		t.Fatal("expected error without a shell")
	}
}
