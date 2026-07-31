package cli

import (
	"fmt"

	"github.com/glnreddy421/klew/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the klew CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("klew %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
