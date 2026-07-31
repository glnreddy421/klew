package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/tui/views"
)

var (
	graphWidth int
	graphDemo  bool
)

var graphCmd = &cobra.Command{
	Use:   "graph [query]",
	Short: "Print workload map and propagation chain (snapshot, no TUI)",
	Long: `Collect a read-only cluster snapshot and print the full workload graph to stdout.

Same namespace and kubeconfig flags as analyze. Requires -n (non-interactive).

Example:
  klew graph payment-api -n klew-lab`,
	Args: cobra.MinimumNArgs(1),
	RunE: runGraph,
}

func init() {
	graphCmd.Flags().IntVar(&graphWidth, "width", 0, "output width in columns (default: terminal width)")
	graphCmd.Flags().BoolVar(&graphDemo, "demo", false, "use simulated demo data (no cluster)")
}

func runGraph(cmd *cobra.Command, args []string) error {
	query := args[0]
	width := graphWidth
	if width <= 0 {
		width = terminalWidth()
	}

	if graphDemo {
		st := engine.DemoState()
		st.Query = query
		printGraphReport(st, width)
		return nil
	}

	client, err := kube.NewFromFlags(kubeconfig, contextName, namespace)
	if err != nil {
		return exitErr(err)
	}

	ns, allNS, err := kube.RequireNamespaceOrError(namespace, allNamespaces)
	if err != nil {
		return exitErr(err)
	}
	if allNS {
		allNamespaces = true
	} else {
		client.Namespace = ns
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	bundle, _, err := engine.CollectSnapshot(ctx, client, engine.SnapshotOptions{
		Namespace: ns,
		Query:     query,
		AllNS:     allNS,
	})
	if err != nil {
		return exitErr(err)
	}

	scope := kube.ScopeFromFlags(namespace, allNamespaces, client.Namespace)
	st := engine.BootstrapState(bundle, scope, query, model.ModeLive)
	inv := engine.Analyze(bundle)
	st.WorkloadGraph = inv.Graph
	st.Verdict = inv.Verdict
	st.Timeline = inv.Timeline
	st.HypothesisLabel = inv.Verdict.LikelyTrigger
	st.Hypothesis = inv.Verdict.LikelyTrigger

	printGraphReport(*st, width)
	return nil
}

func printGraphReport(st model.InvestigationState, width int) {
	printWarnings(st.Warnings)
	ns := st.Snapshot.Namespace
	if ns == "" {
		ns = st.NamespaceScope.Primary
	}
	fmt.Fprintf(os.Stdout, "Klew graph · query=%s · ns=%s · ctx=%s\n\n",
		st.Query, ns, st.KubeContext.Context)
	printStateSummary(st)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, views.GraphReport(st, width))
}

func terminalWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w < 40 {
		return 100
	}
	return w
}
