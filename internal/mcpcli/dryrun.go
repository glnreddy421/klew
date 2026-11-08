package mcpcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/glnreddy421/klew/v2/internal/engine"
	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/model"
	"github.com/glnreddy421/klew/v2/internal/redact"
)

// checkExtensions prints what loaded from the extensions folder and every
// problem found. It returns false when any extension failed to load.
func checkExtensions(w io.Writer, folders []string, reg *engine.Registry) bool {
	for i, f := range folders {
		label := "Extensions folder:"
		if i > 0 {
			label = "Linked folder:    "
		}
		fmt.Fprintf(w, "%s %s\n", label, f)
	}
	collectors := make([]string, 0, len(reg.Collectors))
	for _, c := range reg.Collectors {
		collectors = append(collectors, c.Name())
	}
	fmt.Fprintf(w, "Collectors loaded: %s\n", listOrNone(collectors))
	fmt.Fprintf(w, "Rules loaded:      %s\n", listOrNone(reg.UserRuleNames()))
	if len(reg.Warnings) == 0 {
		fmt.Fprintln(w, "No problems found.")
		return true
	}
	fmt.Fprintf(w, "\n%d problem(s):\n", len(reg.Warnings))
	for _, warn := range reg.Warnings {
		fmt.Fprintf(w, "  - %s\n", strings.TrimPrefix(warn, "extensions: "))
	}
	return false
}

// dryRunOptions selects one rule and where its objects come from: a file of
// saved objects, or a namespace read live (read-only) from the cluster.
type dryRunOptions struct {
	Rule       string
	ObjectFile string
	Namespace  string
	Kubeconfig string
	Context    string
}

// dryRun runs one user rule and prints, for every object, whether it matched,
// its facts and what failed. It never changes the cluster.
func dryRun(w io.Writer, reg *engine.Registry, opts dryRunOptions) error {
	resource, summary, err := reg.RuleResource(opts.Rule)
	if err != nil {
		return err
	}
	if (opts.ObjectFile == "") == (opts.Namespace == "") {
		return fmt.Errorf("give -object FILE (saved objects) or -namespace NS (read live), not both")
	}

	var bundle model.EvidenceBundle
	var source string
	if opts.ObjectFile != "" {
		data, err := os.ReadFile(opts.ObjectFile)
		if err != nil {
			return err
		}
		objects, err := engine.ParseObjects(data)
		if err != nil {
			return fmt.Errorf("%s: %w", opts.ObjectFile, err)
		}
		if bundle, err = engine.BundleFromObjects(resource, namespaceOf(objects), objects); err != nil {
			return err
		}
		source = opts.ObjectFile
	} else if bundle, source, err = collectLive(opts); err != nil {
		return err
	}
	run, err := reg.DryRunRule(opts.Rule, bundle)
	if err != nil {
		return err
	}
	run.Warnings = append(run.Warnings, collectionWarnings(bundle, resource)...)
	fmt.Fprintf(w, "Rule %s on %s from %s\n", run.Rule, describeResource(resource, summary), source)
	printRun(w, run, true)
	return nil
}

// dryRunAll runs every user rule in the extensions folder against one
// namespace, read once. It prints matches and errors per rule, then a summary,
// and reports false when any extension failed to load or any object could not
// be evaluated, so it can gate a change in CI.
func dryRunAll(w io.Writer, reg *engine.Registry, opts dryRunOptions) (bool, error) {
	if opts.ObjectFile != "" {
		return false, fmt.Errorf("-dry-run-all reads a namespace (-namespace NS); a saved-objects file holds one type, so test it with -dry-run RULE -object FILE")
	}
	if opts.Namespace == "" {
		return false, fmt.Errorf("-dry-run-all needs -namespace NS")
	}
	names := reg.UserRuleNames()
	if len(names) == 0 {
		return false, fmt.Errorf("no user rules are loaded from the extensions folder")
	}
	bundle, source, err := collectLive(opts)
	if err != nil {
		return false, err
	}
	fmt.Fprintf(w, "%d rules on %s\n", len(names), source)
	ok := len(reg.Warnings) == 0
	type row struct {
		rule                     string
		matched, errors, checked int
	}
	var rows []row
	for _, name := range names {
		resource, summary, _ := reg.RuleResource(name)
		run, err := reg.DryRunRule(name, bundle)
		if err != nil {
			fmt.Fprintf(w, "\n%s: %v\n", name, err)
			ok = false
			continue
		}
		run.Warnings = append(run.Warnings, collectionWarnings(bundle, resource)...)
		fmt.Fprintf(w, "\nRule %s on %s\n", name, describeResource(resource, summary))
		printRun(w, run, false)
		r := row{rule: name, matched: run.Matched(), errors: countErrors(run), checked: len(run.Checks)}
		if r.errors > 0 {
			ok = false
		}
		rows = append(rows, r)
	}
	fmt.Fprintf(w, "\nSummary\n")
	for _, r := range rows {
		fmt.Fprintf(w, "  %-32s %3d matched  %3d errors  %3d checked\n", r.rule, r.matched, r.errors, r.checked)
	}
	if len(reg.Warnings) > 0 {
		fmt.Fprintf(w, "\n%d extension problem(s); run -check-extensions for details\n", len(reg.Warnings))
	}
	return ok, nil
}

// collectLive reads the namespace the way an investigation does, without
// logs, and never changes the cluster.
func collectLive(opts dryRunOptions) (model.EvidenceBundle, string, error) {
	client, err := kube.NewFromFlags(opts.Kubeconfig, opts.Context, opts.Namespace)
	if err != nil {
		return model.EvidenceBundle{}, "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	bundle, _, err := engine.CollectSnapshot(ctx, client, engine.SnapshotOptions{Namespace: opts.Namespace, NoLogs: true})
	if err != nil {
		return model.EvidenceBundle{}, "", err
	}
	return bundle, fmt.Sprintf("namespace %s (context %s, read-only)", opts.Namespace, client.Context), nil
}

// collectionWarnings are the investigation warnings about resource, such as
// "no permission", so an empty result is not mistaken for a healthy one.
func collectionWarnings(b model.EvidenceBundle, resource string) []string {
	var out []string
	for _, warn := range b.Warnings {
		if strings.Contains(warn, resource) {
			out = append(out, warn)
		}
	}
	return out
}

func describeResource(resource string, summary bool) string {
	if summary {
		return "Klew " + resource + " summaries"
	}
	return resource + " objects"
}

// printRun lists each object's result. Without showNoMatch, objects that did
// not match are counted, not listed, to keep many rules readable.
func printRun(w io.Writer, run engine.DryRun, showNoMatch bool) {
	if len(run.Checks) == 0 {
		fmt.Fprintf(w, "  No %s objects to check, so the rule cannot match anything here.\n", run.Resource)
	}
	for _, c := range run.Checks {
		name := c.Object.Kind + "/" + c.Object.Name
		switch {
		case c.Error != "":
			fmt.Fprintf(w, "  ERROR    %s: %s\n", name, redact.String(c.Error))
		case c.Matched:
			fmt.Fprintf(w, "  MATCH    %s\n", name)
			for _, f := range c.Facts {
				fmt.Fprintf(w, "             fact: %s\n", redact.String(f))
			}
			for _, f := range c.FactErrors {
				fmt.Fprintf(w, "             fact failed: %s\n", redact.String(f))
			}
		case showNoMatch:
			fmt.Fprintf(w, "  no match %s\n", name)
		}
	}
	fmt.Fprintf(w, "  %d of %d matched", run.Matched(), len(run.Checks))
	if errs := countErrors(run); errs > 0 {
		fmt.Fprintf(w, ", %d could not be evaluated", errs)
	}
	fmt.Fprintln(w)
	for _, warn := range run.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", redact.String(warn))
	}
}

func countErrors(run engine.DryRun) int {
	n := 0
	for _, c := range run.Checks {
		if c.Error != "" {
			n++
		}
	}
	return n
}

// namespaceOf is the namespace saved objects share, for rules that read it.
func namespaceOf(objects []map[string]any) string {
	for _, o := range objects {
		if meta, ok := o["metadata"].(map[string]any); ok {
			if ns, ok := meta["namespace"].(string); ok && ns != "" {
				return ns
			}
		}
	}
	return ""
}

func listOrNone(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// extensionFolders lists the folders klew mcp reads: the -extensions-dir
// override alone, or Klew's own folder plus the folders linked in the app.
func extensionFolders(override string) []string {
	if override != "" {
		return []string{override}
	}
	src, _ := engine.ReadExtensionSources(engine.ExtensionSourcesPath())
	return append([]string{engine.DefaultExtensionsDir()}, src.Folders...)
}
