package cli

import "github.com/spf13/cobra"

var watchCmd = &cobra.Command{
	Use:   "watch [query]",
	Short: "Continuous live investigation mode",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runLiveInvestigation,
}

func init() {
	addLiveFlags(watchCmd)
}
