package mcpcli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestMainVersionAndFlagErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Main(AppProg, []string{"-version"}, &out, &errOut); code != 0 {
		t.Fatalf("-version exit %d", code)
	}
	if !strings.HasPrefix(out.String(), "Klew mcp ") {
		t.Errorf("-version printed %q", out.String())
	}

	out.Reset()
	if code := Main(CLIProg, []string{"-no-such-flag"}, &out, &errOut); code != 2 {
		t.Errorf("unknown flag exit %d, want 2", code)
	}
	if code := Main(CLIProg, []string{"-h"}, &out, &errOut); code != 0 {
		t.Errorf("-h exit %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("usage went to stdout: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "-no-log-samples") {
		t.Errorf("usage lacks flags:\n%s", errOut.String())
	}
}

func TestHelpListsEveryFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	Main(CLIProg, []string{"-h"}, &out, &errOut)
	help := errOut.String()
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^  -([a-z-]+)`).FindAllStringSubmatch(help, -1) {
		if seen[m[1]] {
			t.Errorf("-%s is listed twice", m[1])
		}
		seen[m[1]] = true
	}
	for _, group := range [][]string{serverFlags, extensionFlags, otherFlags} {
		for _, n := range group {
			if !seen[n] {
				t.Errorf("help lacks -%s; is it still a flag?", n)
			}
		}
	}
	// Grouped flags plus -h; anything else landed in "Other" ungrouped.
	if want := len(serverFlags) + len(extensionFlags) + len(otherFlags) + 1; len(seen) != want {
		t.Errorf("help lists %d flags, want %d: put new flags in a group", len(seen), want)
	}
}

func TestHelpShowsHowToRunEachBinary(t *testing.T) {
	for _, c := range []struct {
		prog string
		want []string
	}{
		{CLIProg, []string{
			"klew mcp: Klew's read-only MCP server",
			`claude mcp add --scope user klew -- "$(command -v klew)" mcp`,
			`{"mcpServers": {"klew": {"command": "klew", "args": ["mcp", "-no-log-samples"]}}}`,
			"Server options:",
			"Extension tools",
		}},
		{AppProg, []string{
			"claude mcp add --scope user klew -- /Applications/Klew.app/Contents/MacOS/Klew mcp",
			`"command": "/Applications/Klew.app/Contents/MacOS/Klew", "args": ["mcp", "-no-log-samples"]`,
		}},
	} {
		var out, errOut bytes.Buffer
		if code := Main(c.prog, []string{"-help"}, &out, &errOut); code != 0 {
			t.Fatalf("%s -help exit %d", c.prog, code)
		}
		for _, w := range c.want {
			if !strings.Contains(errOut.String(), w) {
				t.Errorf("%s help lacks %q:\n%s", c.prog, w, errOut.String())
			}
		}
	}
}
