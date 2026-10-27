// Package mcpserver exposes Klew's read-only investigation engine as an MCP
// server (cmd/klew-mcp). Tools only list, get and watch cluster objects; none
// creates, updates, patches, deletes or executes anything in the cluster.
package mcpserver

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/history"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

const instructions = `Klew investigates Kubernetes incidents read-only. No tool changes the cluster.

klew_investigate collects pods, events, logs and workload changes for one namespace (optionally narrowed to a workload), watches briefly, and returns a verdict: status, leading signal, causal chain, failing pods, top log and event patterns, and next checks. Its investigation_id works with klew_investigation_details to page through full sections without collecting again.

klew_event_history lists events for a namespace or object, including events Kubernetes has already expired when the Klew desktop app recorded them. klew_workload_changes returns revision history with field-level diffs for a Deployment, StatefulSet, DaemonSet or Helm release.

Suggested actions and next checks are text for a person to review; they are never executed.`

// Options configures the server.
type Options struct {
	// Kubeconfig is an explicit kubeconfig path; "" uses KUBECONFIG or ~/.kube/config.
	Kubeconfig string
	// Context is the default kube context; "" uses the kubeconfig current-context.
	Context string
	// HistoryDir holds the desktop app's events.db; "" disables recorded events.
	HistoryDir string
	// Version is reported to MCP clients.
	Version string
}

// Server holds state shared by tool calls.
type Server struct {
	opts    Options
	history *history.Reader
	cache   *investigationCache

	// Seams for tests.
	newClient func(kubeContext, namespace string) (*kube.Client, error)
	runFor    func(ctx context.Context, client *kube.Client, opts engine.LiveOptions, d time.Duration) (model.InvestigationState, error)
}

// New opens recorded history (when present) and prepares the server.
func New(opts Options) (*Server, error) {
	hr, err := history.OpenReader(opts.HistoryDir)
	if err != nil {
		return nil, err
	}
	s := &Server{opts: opts, history: hr, cache: newInvestigationCache(maxCachedInvestigations), runFor: engine.RunFor}
	s.newClient = s.kubeClient
	return s, nil
}

// Close releases the history database.
func (s *Server) Close() error { return s.history.Close() }

// MCP builds the MCP server with every tool registered.
func (s *Server) MCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "klew", Title: "Klew", Version: s.opts.Version},
		&mcp.ServerOptions{Instructions: instructions})
	s.registerTools(srv)
	return srv
}

func (s *Server) kubeClient(kubeContext, namespace string) (*kube.Client, error) {
	if kubeContext == "" {
		kubeContext = s.opts.Context
	}
	c, err := kube.NewFromFlags(s.opts.Kubeconfig, kubeContext, namespace)
	if err != nil {
		return nil, err
	}
	if s.history != nil {
		c.History = s.history
	}
	return c, nil
}
