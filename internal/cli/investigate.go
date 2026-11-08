// Package cli is the klew command's terminal commands. They run the same
// engine as the desktop app and the MCP server, read-only, and print for a
// person instead of an AI client.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/v2/internal/engine"
	"github.com/glnreddy421/klew/v2/internal/history"
	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/model"
)

// Exit statuses of klew investigate, so scripts and CI can act on the verdict.
const (
	ExitHealthy  = 0
	ExitError    = 1
	ExitUsage    = 2
	ExitWarning  = 3
	ExitCritical = 4
)

const (
	defaultObserve = 10 * time.Second
	maxObserve     = 5 * time.Minute
	defaultLimit   = 10
	// collectTimeout bounds collection on top of the watch time.
	collectTimeout = 2 * time.Minute
)

// investigateOpts are the parsed flags; run is replaced in tests.
type investigateOpts struct {
	kubeconfig, context, namespace, workload, historyDir, output string
	extensionsDir                                                string
	observe                                                      time.Duration
	limit                                                        int
	show                                                         map[string]bool
	noLogs, color                                                bool
}

// Investigate runs "klew investigate": one investigation of a namespace, or a
// workload in it, printed as the desktop app's Overview, Failures, Patterns
// and Evidence. args exclude the command name.
func Investigate(args []string, stdout, stderr io.Writer) int {
	o, code := parseInvestigate(args, stderr)
	if code >= 0 {
		return code
	}
	o.color = o.output == "text" && colorFor(stdout)

	engine.SetRegistry(loadRegistry(o.extensionsDir, stderr))
	kube.InitNetworkProxyFromEnv()

	client, err := kube.NewFromFlags(o.kubeconfig, o.context, o.namespace)
	if err != nil {
		fmt.Fprintf(stderr, "klew investigate: %v\n", err)
		return ExitError
	}
	if o.namespace == "" {
		o.namespace = firstNonEmpty(client.ContextNamespace, client.Namespace, "default")
	}
	if hr := openHistory(o.historyDir, stderr); hr != nil {
		defer hr.Close()
		client.History = hr
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if namespaceMissing(ctx, client, o.namespace) {
		fmt.Fprintf(stderr, "klew investigate: namespace %q not found in context %q\n", o.namespace, client.Context)
		return ExitError
	}
	if isTerminal(stderr) {
		target := o.namespace
		if o.workload != "" {
			target += "/" + o.workload
		}
		fmt.Fprintf(stderr, "Investigating %s in %s, watching for %s…\n", target, client.Context, o.observe)
	}

	ctx, cancel := context.WithTimeout(ctx, collectTimeout+o.observe)
	defer cancel()
	st, err := engine.RunFor(ctx, client, engine.LiveOptions{
		Query: o.workload, Namespace: o.namespace, Tail: 200, NoLogs: o.noLogs, CheckAdmissionPerimeter: true,
	}, o.observe)
	if err != nil && st.CollectedAt.IsZero() {
		fmt.Fprintf(stderr, "klew investigate: %v\n", err)
		return ExitError
	}
	r := buildReport(st, o.observe, o.limit)
	if err != nil {
		r.Warnings = append(r.Warnings, fmt.Sprintf("Investigation incomplete: %v. Results may be partial.", err))
	}
	if o.workload != "" && len(st.MatchedObjects) == 0 && st.Snapshot.Target == nil {
		r.Warnings = append(r.Warnings, fmt.Sprintf("Nothing in %s matched %q; the results cover the whole namespace.", o.namespace, o.workload))
	}

	if o.output == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(stderr, "klew investigate: %v\n", err)
			return ExitError
		}
	} else {
		writeText(stdout, r, o.show, o.color)
	}
	return exitFor(r.Status)
}

// parseInvestigate returns the options, or an exit status >= 0 when the
// command should stop (help, or a usage error). Flags may come before or
// after the workload.
func parseInvestigate(args []string, stderr io.Writer) (investigateOpts, int) {
	var o investigateOpts
	fs := flag.NewFlagSet("klew investigate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.kubeconfig, "kubeconfig", "", "kubeconfig path (default: KUBECONFIG or ~/.kube/config)")
	fs.StringVar(&o.context, "context", "", "kube context (default: kubeconfig current-context)")
	fs.StringVar(&o.namespace, "n", "", "namespace (default: the context's namespace, else default)")
	fs.StringVar(&o.namespace, "namespace", "", "same as -n")
	fs.DurationVar(&o.observe, "observe", defaultObserve, "how long to watch for new events and logs before reporting, up to 5m")
	fs.StringVar(&o.output, "o", "text", "output: text or json")
	only := fs.String("only", "", "comma-separated sections to print: "+strings.Join(sections, ", ")+" (default: all)")
	all := fs.Bool("all", false, "print every item instead of the top 10 of each list")
	fs.BoolVar(&o.noLogs, "no-logs", false, "do not read container logs")
	fs.StringVar(&o.historyDir, "history-dir", defaultHistoryDir(), "directory with the Klew desktop app's recorded events")
	noHistory := fs.Bool("no-history", false, "do not read recorded event history")
	fs.StringVar(&o.extensionsDir, "extensions-dir", "", "directory of collector and rule YAML files to use instead of Klew's extensions folder and the folders linked in the desktop app")
	fs.Usage = func() { investigateHelp(fs.Output(), fs) }

	var rest []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return o, 0
			}
			return o, ExitUsage
		}
		if fs.NArg() == 0 {
			break
		}
		rest = append(rest, fs.Arg(0))
		args = fs.Args()[1:]
	}
	switch {
	case len(rest) > 1:
		return usageErr(stderr, "takes at most one workload, got %s", strings.Join(rest, " "))
	case o.output != "text" && o.output != "json":
		return usageErr(stderr, "-o must be text or json, not %q", o.output)
	case o.observe < 0 || o.observe > maxObserve:
		return usageErr(stderr, "-observe must be between 0s and %s", maxObserve)
	case o.extensionsDir != "" && !isDir(o.extensionsDir):
		return usageErr(stderr, "-extensions-dir %s is not a directory", o.extensionsDir)
	}
	if len(rest) == 1 {
		o.workload = rest[0]
	}
	if *noHistory {
		o.historyDir = ""
	}
	o.limit = defaultLimit
	if *all {
		o.limit = 0
	}
	o.show = map[string]bool{}
	for _, s := range sections {
		o.show[s] = *only == ""
	}
	for _, s := range strings.Split(*only, ",") {
		if s = strings.TrimSpace(strings.ToLower(s)); s == "" {
			continue
		}
		if _, ok := o.show[s]; !ok {
			return usageErr(stderr, "unknown section %q in -only; use %s", s, strings.Join(sections, ", "))
		}
		o.show[s] = true
	}
	return o, -1
}

func usageErr(stderr io.Writer, format string, args ...any) (investigateOpts, int) {
	fmt.Fprintf(stderr, "klew investigate: "+format+"\n", args...)
	fmt.Fprintln(stderr, "Run klew investigate -h for usage.")
	return investigateOpts{}, ExitUsage
}

func investigateHelp(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprint(w, `klew investigate: investigate a namespace, or one workload in it, read-only.

Prints what the desktop app shows in Overview, Failures, Patterns and
Evidence: the verdict, problems, causal chain, failing pods with their
containers and events, log and event patterns, correlated evidence and next
checks. It only lists, gets and watches; nothing in the cluster changes.

Usage:
  klew investigate [-n NAMESPACE] [WORKLOAD] [options]

WORKLOAD narrows the investigation, e.g. api, deploy/api or pod/api-7f9c.

Options:
`)
	fs.PrintDefaults()
	fmt.Fprint(w, `
Exit status:
  0 healthy · 1 error · 2 usage · 3 warning · 4 critical

Output is not redacted: it shows log lines and event messages as they are,
like kubectl logs. Review it before sharing.

Examples:
  klew investigate -n shop
  klew investigate -n shop checkout -observe 30s
  klew investigate -n shop -only failures,patterns
  klew investigate -n shop -o json | jq .overview.problems
  klew investigate -n shop -extensions-dir ./team-rules
`)
}

func exitFor(s model.VerdictStatus) int {
	switch s {
	case model.VerdictCritical:
		return ExitCritical
	case model.VerdictWarning:
		return ExitWarning
	}
	return ExitHealthy
}

// loadRegistry reads Klew's extensions folder and the linked folders, or
// only dir when -extensions-dir names one.
func loadRegistry(dir string, stderr io.Writer) *engine.Registry {
	reg := engine.LoadConfiguredExtensions()
	if dir != "" {
		reg = engine.LoadExtensions(dir)
	}
	for _, w := range reg.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}
	return reg
}

func openHistory(dir string, stderr io.Writer) *history.Reader {
	hr, err := history.OpenReader(dir)
	if err != nil {
		fmt.Fprintf(stderr, "warning: event history unavailable, using live events only: %v\n", err)
		return nil
	}
	return hr
}

func defaultHistoryDir() string {
	if dir := kube.DataDir(); dir != "" {
		return filepath.Join(dir, "history")
	}
	return ""
}

// namespaceMissing reports whether the API says ns does not exist. Any other
// result, such as no permission to get namespaces, is not treated as missing.
func namespaceMissing(ctx context.Context, client *kube.Client, ns string) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := client.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	return apierrors.IsNotFound(err)
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
