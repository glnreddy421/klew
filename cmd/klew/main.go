// Command klew is Klew's command line. "klew investigate" prints an
// investigation for a person; "klew mcp" serves the same read-only engine to
// AI assistants over the Model Context Protocol (stdio). The desktop app runs
// the same server as "Klew mcp".
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/glnreddy421/klew/v2/internal/cli"
	"github.com/glnreddy421/klew/v2/internal/mcpcli"
	"github.com/glnreddy421/klew/v2/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches args (without the program name) to a subcommand and returns
// the exit status. Help goes to stderr, like the subcommands' own.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "investigate":
		return cli.Investigate(args[1:], stdout, stderr)
	case "mcp":
		return mcpcli.Main(mcpcli.CLIProg, args[1:], stdout, stderr)
	case "version", "-version", "--version":
		v := version.Get()
		fmt.Fprintf(stdout, "klew %s (%s, %s)\n", v.Version, v.Commit, v.Date)
		return 0
	case "help", "-h", "-help", "--help":
		usage(stderr)
		return 0
	}
	fmt.Fprintf(stderr, "klew: unknown command %q\n\n", args[0])
	usage(stderr)
	return 2
}

func usage(w io.Writer) {
	fmt.Fprint(w, `klew: Klew's command line for Kubernetes investigation.

Usage:
  klew <command> [options]

Commands:
  investigate  investigate a namespace or workload and print the findings
  mcp          serve Klew's read-only MCP server to an AI client
  version      print version and exit

Run klew <command> -h for a command's options. Nothing klew runs changes the
cluster: it only lists, gets and watches.

More: https://github.com/glnreddy421/klew
`)
}
