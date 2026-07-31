package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/glnreddy421/klew/internal/bundle"
	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
)

var (
	collectOutput   string
	collectDuration string
)

var collectCmd = &cobra.Command{
	Use:   "collect [query]",
	Short: "Collect snapshot + live evidence into a JSON bundle",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if collectOutput == "" {
			return exitErr(fmt.Errorf("--output is required"))
		}
		query := strings.Join(args, " ")
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

		dur := 30 * time.Second
		if collectDuration != "" {
			parsed, err := time.ParseDuration(collectDuration)
			if err != nil {
				return exitErr(fmt.Errorf("invalid --duration: %w", err))
			}
			dur = parsed
		}

		pollEvery, autoRefresh, err := resolveRefresh(refreshFlag)
		if err != nil {
			return exitErr(err)
		}

		st, err := engine.RunFor(cmd.Context(), client, engine.LiveOptions{
			Query: query, Namespace: ns, AllNS: allNamespaces,
			PollEvery: pollEvery, AutoRefresh: autoRefresh,
		}, dur)
		if err != nil {
			return exitErr(err)
		}
		printWarnings(st.Warnings)
		if err := bundle.WriteState(collectOutput, st); err != nil {
			return exitErr(err)
		}
		fmt.Printf("Wrote bundle to %s after %s (%d live events)\n", collectOutput, dur, len(st.LiveEvidence))
		return nil
	},
}

func init() {
	collectCmd.Flags().StringVar(&collectOutput, "output", "", "output bundle JSON path")
	collectCmd.Flags().StringVar(&collectDuration, "duration", "30s", "live collection duration (e.g. 5m)")
	collectCmd.Flags().StringVar(&refreshFlag, "refresh", "10s", "cluster snapshot auto-refresh interval (0 or off to disable)")
	_ = collectCmd.MarkFlagRequired("output")
}
