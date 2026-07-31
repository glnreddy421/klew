package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	kubeconfig      string
	contextName     string
	namespace       string
	allNamespaces   bool
)

// Execute runs the klew CLI.
func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "klew",
	Short: "Kubernetes-native live incident investigation TUI",
	Long:  "Klew continuously combines Kubernetes workload state, events, and multi-pod logs into an evolving incident timeline and visual investigation TUI.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig (default $KUBECONFIG or ~/.kube/config)")
	rootCmd.PersistentFlags().StringVar(&contextName, "context", "", "kubeconfig context override")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	rootCmd.PersistentFlags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Search all namespaces")

	rootCmd.AddCommand(ctxCmd)
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(collectCmd)
	rootCmd.AddCommand(viewCmd)
}

func exitErr(err error) error {
	fmt.Fprintln(os.Stderr, "error:", err)
	return err
}
