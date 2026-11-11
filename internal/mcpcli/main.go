// Package mcpcli is the command line of Klew's MCP server. The klew command's
// "klew mcp" and the desktop app's "Klew mcp" both run it, so they take the
// same flags and behave the same.
package mcpcli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/glnreddy421/klew/v2/internal/engine"
	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/mcpserver"
	"github.com/glnreddy421/klew/v2/internal/version"
)

// Main parses args (without the program name) and runs the command, returning
// the process exit status. prog, CLIProg or AppProg, names the command in
// usage, version and error output. The -check-extensions, -dry-run and -version modes print to stdout;
// serving MCP always uses the process's stdin and stdout, and every log line
// goes to stderr.
func Main(prog string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(prog, flag.ContinueOnError)
	fs.SetOutput(stderr)
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path (default: KUBECONFIG or ~/.kube/config)")
	contextName := fs.String("context", "", "default kube context (default: kubeconfig current-context)")
	historyDir := fs.String("history-dir", defaultHistoryDir(), "directory with the Klew desktop app's recorded events")
	noHistory := fs.Bool("no-history", false, "do not read recorded event history")
	extensionsDir := fs.String("extensions-dir", "", "directory of collector and rule YAML files to use instead of Klew's extensions folder and the folders linked in the desktop app")
	noLogSamples := fs.Bool("no-log-samples", false, "leave raw container log text out of results; generalised patterns, counts and non-log evidence are still returned")
	showVersion := fs.Bool("version", false, "print version and exit")
	checkExt := fs.Bool("check-extensions", false, "load the extensions folder, report what loaded and every problem, and exit")
	dryRunRule := fs.String("dry-run", "", "run one user rule and print, for each object, whether it matched; needs -object or -namespace; read-only")
	dryRunObject := fs.String("object", "", "with -dry-run: a file of saved objects (kubectl get -o yaml) to check")
	dryRunAllRules := fs.Bool("dry-run-all", false, "run every user rule in the extensions folder against -namespace, read once; exit status 1 on any load problem or evaluation error; read-only")
	dryRunNamespace := fs.String("namespace", "", "with -dry-run or -dry-run-all: a namespace to read live, read-only, and check")
	fs.Usage = func() { printHelp(fs.Output(), prog, fs) }
	if err := fs.Parse(args); err != nil {
		// Matches flag.ExitOnError: -h exits 0, a bad flag exits 2.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *showVersion {
		v := version.Get()
		fmt.Fprintf(stdout, "%s %s (%s, %s)\n", prog, v.Version, v.Commit, v.Date)
		return 0
	}
	// A mistyped folder would otherwise load no rules without a word.
	if *extensionsDir != "" {
		if fi, err := os.Stat(*extensionsDir); err != nil || !fi.IsDir() {
			fmt.Fprintf(stderr, "%s: -extensions-dir %s is not a directory\n", prog, *extensionsDir)
			return 2
		}
	}

	// stdout carries the MCP protocol; every log line goes to stderr.
	slog.SetDefault(slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	// Hosts such as Claude Desktop start servers without the login shell's
	// PATH, KUBECONFIG or cloud credentials that exec auth plugins need.
	kube.BootstrapLoginShellEnv()
	kube.InitNetworkProxyFromEnv()

	if *noHistory {
		*historyDir = ""
	}
	reg := engine.LoadConfiguredExtensions()
	if *extensionsDir != "" {
		reg = engine.LoadExtensions(*extensionsDir)
	}
	// Not serving MCP: these modes print to stdout and exit.
	if *checkExt {
		if !checkExtensions(stdout, extensionFolders(*extensionsDir), reg) {
			return 1
		}
		return 0
	}
	if *dryRunRule != "" || *dryRunAllRules {
		for _, w := range reg.Warnings {
			fmt.Fprintf(stderr, "warning: %s\n", w)
		}
		engine.SetRegistry(reg)
		opts := dryRunOptions{
			Rule: *dryRunRule, ObjectFile: *dryRunObject, Namespace: *dryRunNamespace,
			Kubeconfig: *kubeconfig, Context: *contextName,
		}
		ok, err := true, error(nil)
		switch {
		case *dryRunRule != "" && *dryRunAllRules:
			err = fmt.Errorf("use -dry-run RULE or -dry-run-all, not both")
		case *dryRunAllRules:
			ok, err = dryRunAll(stdout, reg, opts)
		default:
			err = dryRun(stdout, reg, opts)
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", prog, err)
			return 1
		}
		if !ok {
			return 1
		}
		return 0
	}
	for _, w := range reg.Warnings {
		slog.Warn(w)
	}
	engine.SetRegistry(reg)
	if err := serve(mcpserver.Options{
		Kubeconfig: *kubeconfig, Context: *contextName, HistoryDir: *historyDir, Version: version.Version,
		NoLogSamples: *noLogSamples,
	}); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", prog, err)
		return 1
	}
	return 0
}

func serve(opts mcpserver.Options) error {
	srv := mcpserver.New(opts)
	defer srv.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.MCP().Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func defaultHistoryDir() string {
	if dir := kube.DataDir(); dir != "" {
		return filepath.Join(dir, "history")
	}
	return ""
}
