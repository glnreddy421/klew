package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/service"
	"github.com/glnreddy421/klew/internal/tui"
)

var (
	analyzeNoTUI bool
	tailFlag     int
	refreshFlag  string
	demoFlag     bool
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [query]",
	Short: "Live Kubernetes investigation (snapshot + watch + log stream + TUI)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runLiveInvestigation,
}

func addLiveFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&analyzeNoTUI, "no-tui", false, "print summary instead of TUI")
	cmd.Flags().IntVar(&tailFlag, "tail", 200, "log tail lines per container")
	cmd.Flags().StringVar(&refreshFlag, "refresh", "10s", "cluster snapshot auto-refresh interval (0 or off to disable)")
	cmd.Flags().BoolVar(&demoFlag, "demo", false, "run a fully simulated investigation (no cluster)")
}

func init() {
	addLiveFlags(analyzeCmd)
}

// resolveRefresh parses --refresh (default 10s). Returns interval and whether auto-refresh is enabled.
func resolveRefresh(s string) (time.Duration, bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" || s == "off" || s == "false" {
		return 0, false, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, false, fmt.Errorf("invalid --refresh %q: %w", s, err)
	}
	if d <= 0 {
		return 0, false, nil
	}
	return d, true, nil
}

func liveOpts(query, ns string, allNS bool, tail int) (engine.LiveOptions, error) {
	pollEvery, autoRefresh, err := resolveRefresh(refreshFlag)
	if err != nil {
		return engine.LiveOptions{}, err
	}
	if autoRefresh && pollEvery <= 0 {
		pollEvery = 10 * time.Second
	}
	return engine.LiveOptions{
		Query: query, Namespace: ns, AllNS: allNS,
		PollEvery: pollEvery, AutoRefresh: autoRefresh,
		Tail: tail,
	}, nil
}

func runLiveInvestigation(cmd *cobra.Command, args []string) error {
	query := args[0]

	if demoFlag {
		return demoCmd.RunE(cmd, args)
	}

	noTUI := analyzeNoTUI

	tail := tailFlag
	if tail <= 0 {
		tail = 200
	}
	opts, err := liveOpts(query, namespace, allNamespaces, tail)
	if err != nil {
		return exitErr(err)
	}

	client, err := kube.NewFromFlags(kubeconfig, contextName, namespace)
	if err != nil {
		return exitErr(err)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if noTUI {
		ns, allNS, err := kube.RequireNamespaceOrError(namespace, allNamespaces)
		if err != nil {
			return exitErr(err)
		}
		if allNS {
			allNamespaces = true
		} else {
			client.Namespace = ns
		}

		svc, err := service.Start(ctx, client, opts)
		if err != nil {
			return exitErr(err)
		}
		defer svc.Stop()

		printWarnings(svc.State().Warnings)
		printStateSummary(svc.State())
		<-ctx.Done()
		return nil
	}

	return tui.RunLiveStartup(ctx, tui.LiveStartupOptions{
		Client:           client,
		Query:            query,
		Namespace:        namespace,
		ContextNamespace: client.ContextNamespace,
		AllNamespaces:    allNamespaces,
		Tail:             tail,
		PollEvery:        opts.PollEvery,
		AutoRefresh:      opts.AutoRefresh,
	})
}

func printStateSummary(st model.InvestigationState) {
	fmt.Printf("Status: %s (confidence %.0f%%)\n", st.Verdict.Status, st.Verdict.Confidence*100)
	fmt.Printf("Leading signal: %s\n", st.Verdict.LeadingSignal)
	fmt.Printf("Likely trigger: %s\n", st.Verdict.LikelyTrigger)
	fmt.Println(st.Verdict.Summary)
}
