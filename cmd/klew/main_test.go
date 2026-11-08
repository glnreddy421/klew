package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunDispatches(t *testing.T) {
	for _, c := range []struct {
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{nil, 2, "", "Commands:"},
		{[]string{"-h"}, 0, "", "investigate  investigate"},
		{[]string{"investigate", "-h"}, 0, "", "Exit status:"},
		{[]string{"version"}, 0, "klew ", ""},
		{[]string{"mcp", "-version"}, 0, "klew mcp ", ""},
		{[]string{"mcp", "-h"}, 0, "", `"$(command -v klew)" mcp`},
		{[]string{"serve"}, 2, "", `unknown command "serve"`},
	} {
		var out, errOut bytes.Buffer
		if code := run(c.args, &out, &errOut); code != c.code {
			t.Errorf("%v: exit %d, want %d", c.args, code, c.code)
		}
		if !strings.HasPrefix(out.String(), c.stdout) || (c.stdout == "" && out.Len() != 0) {
			t.Errorf("%v: stdout %q, want prefix %q", c.args, out.String(), c.stdout)
		}
		if !strings.Contains(errOut.String(), c.stderr) {
			t.Errorf("%v: stderr lacks %q:\n%s", c.args, c.stderr, errOut.String())
		}
	}
}
