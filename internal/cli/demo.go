package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/glnreddy421/klew/internal/tui"
)

var demoCmd = &cobra.Command{
	Use:   "demo [query]",
	Short: "Fully simulated live investigation — no cluster required (payment | vault | checkout)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := "payment"
		if len(args) > 0 {
			query = args[0]
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return tui.RunDemo(ctx, query)
	},
}
