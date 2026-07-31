package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/glnreddy421/klew/internal/kube"
)

var ctxCmd = &cobra.Command{
	Use:   "ctx",
	Short: "Show current kube context and validate read permissions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kube.NewFromFlags(kubeconfig, contextName, namespace)
		if err != nil {
			return exitErr(err)
		}
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		ctxName, cluster, user, ns := client.ContextInfo()
		fmt.Println("Klew Kubernetes Context")
		fmt.Println("────────────────────────")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Context:\t%s\n", ctxName)
		fmt.Fprintf(w, "Cluster:\t%s\n", cluster)
		fmt.Fprintf(w, "User:\t%s\n", user)
		fmt.Fprintf(w, "Namespace:\t%s\n", ns)
		kcfg := kube.DefaultKubeconfigPath()
		if kcfg != "" {
			fmt.Fprintf(w, "Kubeconfig:\t%s\n", kcfg)
		}
		w.Flush()

		checks, warnings := kube.CheckPermissions(ctx, client, ns)
		fmt.Println("\nRead permissions")
		for _, c := range checks {
			status := "denied"
			if c.Allowed {
				status = "allowed"
			}
			fmt.Printf("  %-6s %-28s %s\n", c.Verb, c.Resource, status)
		}
		if len(warnings) > 0 {
			fmt.Println("\nWarnings")
			for _, w := range warnings {
				fmt.Println(" ", w)
			}
		} else {
			fmt.Println("\nAll probed read permissions granted.")
		}
		return nil
	},
}

func printWarnings(warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "warnings:")
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, " ", strings.TrimSpace(w))
	}
}
