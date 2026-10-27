// Command klew-mcp serves Klew's read-only Kubernetes investigation engine to
// AI assistants over the Model Context Protocol (stdio).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/mcpserver"
	"github.com/glnreddy421/klew/internal/version"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "kubeconfig path (default: KUBECONFIG or ~/.kube/config)")
	contextName := flag.String("context", "", "default kube context (default: kubeconfig current-context)")
	historyDir := flag.String("history-dir", defaultHistoryDir(), "directory with the Klew desktop app's recorded events")
	noHistory := flag.Bool("no-history", false, "do not read recorded event history")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		v := version.Get()
		fmt.Printf("klew-mcp %s (%s, %s)\n", v.Version, v.Commit, v.Date)
		return
	}

	// stdout carries the MCP protocol; every log line goes to stderr.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	// Hosts such as Claude Desktop start servers without the login shell's
	// PATH, KUBECONFIG or cloud credentials that exec auth plugins need.
	kube.BootstrapLoginShellEnv()
	kube.InitNetworkProxyFromEnv()

	if *noHistory {
		*historyDir = ""
	}
	if err := run(mcpserver.Options{
		Kubeconfig: *kubeconfig, Context: *contextName, HistoryDir: *historyDir, Version: version.Version,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "klew-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run(opts mcpserver.Options) error {
	srv, err := mcpserver.New(opts)
	if err != nil {
		return err
	}
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
