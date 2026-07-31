package cli

import (
	"github.com/spf13/cobra"

	"github.com/glnreddy421/klew/internal/bundle"
	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/tui"
)

var viewNoTUI bool

var viewCmd = &cobra.Command{
	Use:   "view [bundle.json]",
	Short: "Open the investigation TUI from a collected bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := bundle.ReadState(args[0])
		if err != nil {
			return exitErr(err)
		}
		printWarnings(st.Warnings)
		if len(st.LiveEvidence) == 0 && st.Verdict.Confidence == 0 {
			inv := engine.Analyze(st.Snapshot)
			st.Timeline = inv.Timeline
			st.WorkloadGraph = inv.Graph
			st.Verdict = inv.Verdict
		}
		st.Mode = model.ModeBundle
		if viewNoTUI {
			printStateSummary(st)
			return nil
		}
		return tui.RunBundle(st)
	},
}

func init() {
	viewCmd.Flags().BoolVar(&viewNoTUI, "no-tui", false, "print summary instead of TUI")
}
