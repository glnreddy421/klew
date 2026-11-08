package mcpcli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

const docsURL = "https://github.com/glnreddy421/klew/blob/main/docs/mcp.md"

// appExecutable is where the desktop app's binary is installed; its help
// shows this path, since the app's binary is not on PATH.
const appExecutable = "/Applications/Klew.app/Contents/MacOS/Klew"

// The prog names Main takes: the standalone klew command's "klew mcp", and
// the desktop app's "Klew mcp". Help shows how to run the one it was given.
const (
	CLIProg = "klew mcp"
	AppProg = "Klew mcp"
)

// Flag groups for the help text. A flag in none of them is listed under
// "Other", so a new flag always shows up.
var (
	serverFlags    = []string{"context", "kubeconfig", "no-log-samples", "no-history", "history-dir", "extensions-dir"}
	extensionFlags = []string{"check-extensions", "dry-run", "dry-run-all", "object", "namespace"}
	otherFlags     = []string{"version"}
)

// commandForms is how prog is written in a terminal, and as the command and
// leading args of a JSON client entry.
func commandForms(prog string) (shell, jsonCommand string, jsonArgs []string) {
	if prog == AppProg {
		return appExecutable + " mcp", appExecutable, []string{"mcp"}
	}
	return `"$(command -v klew)" mcp`, "klew", []string{"mcp"}
}

// printHelp writes the -h text: what the server is, its flags in groups, and
// how to add it to a client.
func printHelp(w io.Writer, prog string, fs *flag.FlagSet) {
	shell, jsonCommand, jsonArgs := commandForms(prog)
	name := prog
	if prog == AppProg {
		name = appExecutable + " mcp"
	}
	args := append(append([]string{}, jsonArgs...), "-no-log-samples")
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = fmt.Sprintf("%q", a)
	}

	fmt.Fprintf(w, `%s: Klew's read-only MCP server for Kubernetes.

An MCP client (Claude Code, Claude Desktop, Cursor, ...) starts this command
and talks to it over stdin and stdout; you do not normally run it yourself.
Its tools only list, get and watch, and never change the cluster. They use
your kubeconfig and its RBAC.

Usage:
  %s [server options]
  %s -check-extensions | -dry-run RULE ... | -dry-run-all ...

`, prog, name, name)
	printFlagGroup(w, fs, "Server options", serverFlags)
	fmt.Fprintln(w)
	printFlagGroup(w, fs, "Extension tools (print a report and exit; read-only)", extensionFlags)
	fmt.Fprintln(w)
	other := append([]string{}, otherFlags...)
	grouped := map[string]bool{}
	for _, n := range append(append(append([]string{}, serverFlags...), extensionFlags...), otherFlags...) {
		grouped[n] = true
	}
	fs.VisitAll(func(f *flag.Flag) {
		if !grouped[f.Name] {
			other = append(other, f.Name)
		}
	})
	printFlagGroup(w, fs, "Other", other)
	fmt.Fprintf(w, `  -h, -help
    	show this help

Add to Claude Code:
  claude mcp add --scope user klew -- %[1]s
  claude mcp add --scope user klew -- %[1]s -context prod -no-log-samples

Add to Claude Desktop, Cursor and other JSON-configured clients:
  {"mcpServers": {"klew": {"command": %[2]q, "args": [%[3]s]}}}

Check your own rules:
  %[4]s -dry-run my-rule -namespace shop

More: %[5]s
`, shell, jsonCommand, strings.Join(quoted, ", "), name, docsURL)
}

// printFlagGroup prints the named flags in the layout flag.PrintDefaults uses.
func printFlagGroup(w io.Writer, fs *flag.FlagSet, title string, names []string) {
	fmt.Fprintf(w, "%s:\n", title)
	for _, n := range names {
		f := fs.Lookup(n)
		if f == nil {
			continue
		}
		typ, usage := flag.UnquoteUsage(f)
		line := "  -" + f.Name
		if typ != "" {
			line += " " + typ
		}
		fmt.Fprintf(w, "%s\n    \t%s", line, strings.ReplaceAll(usage, "\n", "\n    \t"))
		if f.DefValue != "" && f.DefValue != "false" {
			fmt.Fprintf(w, " (default %q)", f.DefValue)
		}
		fmt.Fprintln(w)
	}
}
