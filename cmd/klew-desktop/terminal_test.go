package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/kube"
)

func TestShellQuoteRoundTrip(t *testing.T) {
	for _, v := range []string{"", "plain", "it's", "a'b'c", `%s "x" $HOME`} {
		out, err := exec.Command("/bin/sh", "-c", "printf %s "+shellQuote(v)).Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != v {
			t.Fatalf("shellQuote(%q) round-tripped to %q", v, out)
		}
	}
}

func TestPsQuote(t *testing.T) {
	if got := psQuote("o'brien"); got != "'o''brien'" {
		t.Fatalf("psQuote = %s", got)
	}
}

func TestBuildUnixInitScriptQuotesNames(t *testing.T) {
	script := buildUnixInitScript("/bin/zsh", kube.TerminalShellKube{
		KubeconfigPath: "/tmp/k",
		Context:        "ctx'%s",
		Namespace:      "ns",
		Isolated:       true,
	})
	// Run everything except the final exec to verify the banner renders verbatim.
	banner := script[:strings.LastIndex(script, "exec ")]
	out, err := exec.Command("/bin/sh", "-c", banner).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "context: ctx'%s · namespace: ns") {
		t.Fatalf("banner = %q", out)
	}
	if strings.Contains(string(out), "session copy unavailable") {
		t.Fatal("isolated session should not show fallback notice")
	}
}

func TestBuildUnixInitScriptPlainShellShowsNotice(t *testing.T) {
	script := buildUnixInitScript("/bin/zsh", kube.PlainTerminalShellKube(errors.New("kubeconfig has no current context")))
	if strings.Contains(script, "export KUBECONFIG") {
		t.Fatal("plain shell must not override KUBECONFIG")
	}
	banner := script[:strings.LastIndex(script, "exec ")]
	out, err := exec.Command("/bin/sh", "-c", banner).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "no cluster context") || !strings.Contains(string(out), "no current context") {
		t.Fatalf("banner = %q", out)
	}
}
