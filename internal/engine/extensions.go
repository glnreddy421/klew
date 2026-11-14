package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/ext"
	"sigs.k8s.io/yaml"

	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/model"
)

// User extensions are YAML files of Collector and Rule documents:
//
//	apiVersion: klew.dev/v1alpha1
//	kind: Collector
//	metadata: {name: certificates}
//	spec: {group: cert-manager.io, version: v1, resource: certificates}
//	---
//	apiVersion: klew.dev/v1alpha1
//	kind: Rule
//	metadata: {name: certificate-not-ready}
//	spec:
//	  resource: certificates
//	  when: "object.status.conditions.exists(c, c.type == 'Ready' && c.status != 'True')"
//	  severity: high
//	  state: CertificateNotReady
//	  facts: ["object.status.conditions.filter(c, c.type == 'Ready')[0].message"]
//
// Collectors can only list one resource type. Rules are CEL expressions over
// one object at a time, so neither can change the cluster.

// ExtensionAPIVersion is the apiVersion extension documents must declare.
const ExtensionAPIVersion = "klew.dev/v1alpha1"

const (
	maxRuleFindings = 200
	celCostLimit    = 1_000_000
)

var (
	statePattern  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9:_-]{0,63}$`)
	docSeparator  = regexp.MustCompile(`(?m)^---\s*$`)
	ruleResources = map[string]string{
		"pods": "Pod", "services": "Service", "workloads": "", "replicasets": "ReplicaSet",
		"jobs": "Job", "events": "Event", "nodes": "Node",
	}
)

type extensionDoc struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec json.RawMessage `json:"spec"`
}

// RuleSpec defines a user rule.
type RuleSpec struct {
	// Resource is a built-in kind (pods, services, workloads, replicasets,
	// jobs, events, nodes) or the name of a user collector.
	Resource string `json:"resource"`
	// When is a CEL expression over `object` (and `ns`) that is true
	// when the object has the problem.
	When     string         `json:"when"`
	Severity model.Severity `json:"severity,omitempty"`
	// State names the problem, e.g. CertificateNotReady.
	State string `json:"state"`
	// Facts are CEL expressions whose values describe the problem.
	Facts []string `json:"facts,omitempty"`
}

// ExtensionEntry is one Collector or Rule document and whether it loaded, so
// the desktop app can list every extension with its status.
type ExtensionEntry struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Folder   string `json:"folder"`
	File     string `json:"file"`
	Document int    `json:"document"`
	// Status is "loaded" or "skipped"; Reason says why a document was skipped.
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Extension entry statuses.
const (
	ExtensionLoaded  = "loaded"
	ExtensionSkipped = "skipped"
)

// LoadExtensions returns a registry with the default rules plus the
// collectors and rules defined in dir's *.yaml and *.yml files. A missing dir
// is not an error. Documents that fail to load are skipped and described in
// Registry.Warnings.
func LoadExtensions(dir string) *Registry {
	return LoadExtensionDirs(dir, nil)
}

// LoadExtensionDirs loads Klew's own extensions folder (dir, which may not
// exist yet) and the folders the user linked, which are read in place. A
// linked folder that is missing is a warning. Names must be unique across all
// folders.
func LoadExtensionDirs(dir string, linked []string) *Registry {
	reg := DefaultRegistry()
	names := map[string]bool{}
	for _, r := range reg.Rules {
		names["rule/"+r.Name()] = true
	}
	for _, n := range kube.DefaultCollectorNames() {
		names["collector/"+n] = true
	}
	if dir != "" {
		reg.loadFolder(dir, false, names)
	}
	for _, l := range linked {
		if l != "" && filepath.Clean(l) != filepath.Clean(dir) {
			reg.loadFolder(l, true, names)
		}
	}
	reg.dropRulesWithoutResource()
	return reg
}

func (r *Registry) loadFolder(dir string, mustExist bool, names map[string]bool) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		if mustExist {
			r.Warnings = append(r.Warnings, fmt.Sprintf("extensions: linked folder %s was not found", dir))
		}
		return
	}
	if err != nil {
		r.Warnings = append(r.Warnings, fmt.Sprintf("extensions: %v", err))
		return
	}
	var files []string
	for _, e := range entries {
		if ext := filepath.Ext(e.Name()); !e.IsDir() && (ext == ".yaml" || ext == ".yml") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			r.Warnings = append(r.Warnings, fmt.Sprintf("extensions: %v", err))
			continue
		}
		for i, doc := range docSeparator.Split(string(data), -1) {
			if strings.TrimSpace(doc) == "" {
				continue
			}
			entry := ExtensionEntry{Folder: dir, File: f, Document: i + 1, Status: ExtensionLoaded}
			var head extensionDoc
			if yaml.Unmarshal([]byte(doc), &head) == nil {
				entry.Kind, entry.Name = head.Kind, head.Metadata.Name
			}
			if err := r.addExtension([]byte(doc), names); err != nil {
				entry.Status, entry.Reason = ExtensionSkipped, err.Error()
				r.Warnings = append(r.Warnings, fmt.Sprintf("extensions: %s document %d: %v", filepath.Base(f), i+1, err))
			}
			r.Entries = append(r.Entries, entry)
		}
	}
}

// RuleResources lists what a rule's resource may name: Klew's built-in
// summaries, the built-in resource collectors and the registry's collectors.
func (r *Registry) RuleResources() []string {
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for n := range ruleResources {
		add(n)
	}
	for _, n := range kube.ResourceCollectorNames() {
		add(n)
	}
	for _, c := range r.Collectors {
		add(c.Name())
	}
	sort.Strings(out)
	return out
}

// dropRulesWithoutResource removes user rules whose resource names nothing a
// rule can read. Such a rule would never see an object and so look like
// "nothing wrong"; it is rejected with a warning instead. It runs after every
// file has loaded, so a rule may use a collector defined in another file.
func (r *Registry) dropRulesWithoutResource() {
	valid := r.RuleResources()
	known := map[string]bool{}
	for _, n := range valid {
		known[n] = true
	}
	kept := r.Rules[:0]
	for _, rule := range r.Rules {
		cr, ok := rule.(*celRule)
		if !ok || known[cr.spec.Resource] {
			kept = append(kept, rule)
			continue
		}
		msg := fmt.Sprintf("extensions: rule %s: resource %q is not a collector or built-in resource, so the rule is skipped", cr.name, cr.spec.Resource)
		if s := closest(cr.spec.Resource, valid); s != "" {
			msg += fmt.Sprintf(". Did you mean %q?", s)
		}
		r.Warnings = append(r.Warnings, msg)
		for i := range r.Entries {
			if e := &r.Entries[i]; e.Kind == "Rule" && e.Name == cr.name && e.Status == ExtensionLoaded {
				e.Status, e.Reason = ExtensionSkipped, strings.TrimPrefix(msg, "extensions: rule "+cr.name+": ")
			}
		}
	}
	r.Rules = kept
}

// closest returns the candidate within two edits of s, if any.
func closest(s string, candidates []string) string {
	best, bestD := "", 3
	for _, c := range candidates {
		if d := editDistance(s, c); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func (r *Registry) addExtension(data []byte, names map[string]bool) error {
	var doc extensionDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.APIVersion != ExtensionAPIVersion {
		return fmt.Errorf("apiVersion %q, want %s", doc.APIVersion, ExtensionAPIVersion)
	}
	name := doc.Metadata.Name
	if name == "" {
		return fmt.Errorf("%s needs metadata.name", doc.Kind)
	}
	key := strings.ToLower(doc.Kind) + "/" + name
	if names[key] {
		return fmt.Errorf("%s %q already exists", doc.Kind, name)
	}
	switch doc.Kind {
	case "Collector":
		var spec kube.ResourceCollectorSpec
		if err := strictJSON(doc.Spec, &spec); err != nil {
			return fmt.Errorf("collector %s: %w", name, err)
		}
		spec.Name = name
		c, err := kube.NewResourceCollector(spec)
		if err != nil {
			return err
		}
		r.Collectors = append(r.Collectors, c)
	case "Rule":
		var spec RuleSpec
		if err := strictJSON(doc.Spec, &spec); err != nil {
			return fmt.Errorf("rule %s: %w", name, err)
		}
		rule, err := NewCELRule(name, spec)
		if err != nil {
			return err
		}
		r.Rules = append(r.Rules, rule)
	default:
		return fmt.Errorf("kind %q, want Collector or Rule", doc.Kind)
	}
	names[key] = true
	return nil
}

func strictJSON(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

type celRule struct {
	name  string
	spec  RuleSpec
	when  cel.Program
	facts []cel.Program
}

// NewCELRule compiles spec. Expressions see `object` (one object of
// spec.Resource, as JSON) and `ns`, the investigation namespace. CEL reserves
// the word namespace, so the variable is ns.
func NewCELRule(name string, spec RuleSpec) (Rule, error) {
	if spec.Resource == "" || spec.When == "" {
		return nil, fmt.Errorf("rule %s needs resource and when", name)
	}
	if !statePattern.MatchString(spec.State) {
		return nil, fmt.Errorf("rule %s: state %q must be a short identifier such as CertificateNotReady", name, spec.State)
	}
	switch spec.Severity {
	case "":
		spec.Severity = model.SeverityWarning
	case model.SeverityCritical, model.SeverityHigh, model.SeverityWarning, model.SeverityInfo:
	default:
		return nil, fmt.Errorf("rule %s: severity %q, want critical, high, warning or info", name, spec.Severity)
	}
	env, err := cel.NewEnv(
		cel.Variable("object", cel.DynType),
		cel.Variable("ns", cel.StringType),
		ext.Strings(),
	)
	if err != nil {
		return nil, err
	}
	compile := func(expr string, want *cel.Type) (cel.Program, error) {
		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			if strings.Contains(iss.Err().Error(), "reserved identifier: namespace") {
				return nil, fmt.Errorf("%w\nnamespace is a reserved word in CEL: use ns for the investigation namespace (object.metadata.namespace still works)", iss.Err())
			}
			return nil, iss.Err()
		}
		if t := ast.OutputType(); !t.IsExactType(want) && !t.IsExactType(cel.DynType) {
			return nil, fmt.Errorf("%q returns %s, want %s", expr, t, want)
		}
		return env.Program(ast, cel.CostLimit(celCostLimit))
	}
	r := &celRule{name: name, spec: spec}
	if r.when, err = compile(spec.When, cel.BoolType); err != nil {
		return nil, fmt.Errorf("rule %s when: %w", name, err)
	}
	for _, f := range spec.Facts {
		p, err := compile(f, cel.StringType)
		if err != nil {
			return nil, fmt.Errorf("rule %s fact: %w", name, err)
		}
		r.facts = append(r.facts, p)
	}
	return r, nil
}

func (r *celRule) Name() string { return r.name }

func (r *celRule) Evaluate(b model.EvidenceBundle) RuleResult {
	var out RuleResult
	var whenErrs, factErrs evalErrors
	objects := ruleObjects(b, r.spec.Resource)
	for i, obj := range objects {
		c := r.check(obj, b.Namespace)
		if c.Error != "" {
			whenErrs.add(c.Error)
			continue
		}
		factErrs.add(c.FactErrors...)
		if !c.Matched {
			continue
		}
		out.Findings = append(out.Findings, model.Finding{Object: obj.ref, Severity: r.spec.Severity, State: r.spec.State, Facts: c.Facts})
		if len(out.Findings) >= maxRuleFindings {
			if rest := len(objects) - i - 1; rest > 0 {
				out.Warnings = append(out.Warnings, fmt.Sprintf("rule %s: stopped at %d findings; %d more objects were not checked", r.name, maxRuleFindings, rest))
			}
			break
		}
	}
	if whenErrs.n > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("rule %s: %d objects could not be evaluated (first: %s); use has() for optional fields", r.name, whenErrs.n, whenErrs.first))
	}
	if factErrs.n > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("rule %s: %d facts could not be evaluated and were left out (first: %s)", r.name, factErrs.n, factErrs.first))
	}
	return out
}

// RuleCheck is one object's result in a dry run: whether the rule matched,
// its facts, and what failed.
type RuleCheck struct {
	Object     model.ObjectRef `json:"object"`
	Matched    bool            `json:"matched"`
	Facts      []string        `json:"facts,omitempty"`
	FactErrors []string        `json:"factErrors,omitempty"`
	// Error is set when `when` could not be evaluated for this object.
	Error string `json:"error,omitempty"`
}

// check evaluates the rule on one object exactly as an investigation does.
func (r *celRule) check(obj ruleObject, namespace string) RuleCheck {
	c := RuleCheck{Object: obj.ref}
	vars := map[string]any{"object": obj.data, "ns": namespace}
	val, _, err := r.when.Eval(vars)
	if err != nil {
		c.Error = err.Error()
		return c
	}
	hit, ok := val.Value().(bool)
	if !ok {
		c.Error = fmt.Sprintf("when returned %v (%s), not true or false", val.Value(), val.Type().TypeName())
		return c
	}
	if !hit {
		return c
	}
	c.Matched = true
	for i, p := range r.facts {
		v, _, err := p.Eval(vars)
		if err != nil {
			c.FactErrors = append(c.FactErrors, fmt.Sprintf("fact %d: %v", i+1, err))
			continue
		}
		c.Facts = append(c.Facts, fmt.Sprint(v.Value()))
	}
	return c
}

// evalErrors counts failures and keeps the first message.
type evalErrors struct {
	n     int
	first string
}

func (e *evalErrors) add(msgs ...string) {
	for _, m := range msgs {
		if e.n == 0 {
			e.first = m
		}
		e.n++
	}
}

type ruleObject struct {
	ref  model.ObjectRef
	data map[string]any
}

// ruleObjects returns the objects a rule over resource sees: built-in
// summaries as JSON, or a user collector's objects.
func ruleObjects(b model.EvidenceBundle, resource string) []ruleObject {
	if kind, ok := ruleResources[resource]; ok {
		return builtinObjects(b, resource, kind)
	}
	var out []ruleObject
	for _, obj := range b.Resources[resource] {
		ref := model.ObjectRef{Kind: str(obj["kind"])}
		if meta, ok := obj["metadata"].(map[string]any); ok {
			ref.Name, ref.Namespace = str(meta["name"]), str(meta["namespace"])
		}
		out = append(out, ruleObject{ref: ref, data: obj})
	}
	return out
}

func builtinObjects(b model.EvidenceBundle, resource, kind string) []ruleObject {
	var items []any
	var refs []model.ObjectRef
	switch resource {
	case "pods":
		for _, p := range b.Pods {
			items, refs = append(items, p), append(refs, model.ObjectRef{Kind: kind, Name: p.Name, Namespace: p.Namespace})
		}
	case "services":
		for _, s := range b.Services {
			items, refs = append(items, s), append(refs, model.ObjectRef{Kind: kind, Name: s.Name, Namespace: s.Namespace})
		}
	case "workloads":
		for _, w := range b.Workloads {
			items, refs = append(items, w), append(refs, model.ObjectRef{Kind: w.Kind, Name: w.Name, Namespace: w.Namespace})
		}
	case "replicasets":
		for _, rs := range b.ReplicaSets {
			items, refs = append(items, rs), append(refs, model.ObjectRef{Kind: kind, Name: rs.Name, Namespace: rs.Namespace})
		}
	case "jobs":
		for _, j := range b.Jobs {
			items, refs = append(items, j), append(refs, model.ObjectRef{Kind: kind, Name: j.Name, Namespace: j.Namespace})
		}
	case "events":
		for _, e := range b.Events {
			items, refs = append(items, e), append(refs, e.InvolvedObject)
		}
	case "nodes":
		for _, n := range b.Nodes {
			items, refs = append(items, n), append(refs, model.ObjectRef{Kind: kind, Name: n.Name})
		}
	}
	out := make([]ruleObject, 0, len(items))
	for i, it := range items {
		raw, err := json.Marshal(it)
		if err != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			out = append(out, ruleObject{ref: refs[i], data: m})
		}
	}
	return out
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// DefaultExtensionsDir is where the desktop app and the klew command look for
// extensions: "extensions" in Klew's data directory.
func DefaultExtensionsDir() string {
	if dir := kube.DataDir(); dir != "" {
		return filepath.Join(dir, "extensions")
	}
	return ""
}

// TargetGroups lists what an investigation can target in ns, from the
// built-in collectors and the active registry's user collectors.
func TargetGroups(ctx context.Context, client *kube.Client, ns string) model.TargetList {
	return kube.ListTargets(ctx, client, ns, ActiveRegistry().Collectors)
}

// InvestigableTypes lists the resource types an investigation can target,
// from the built-in collectors and the active registry's user collectors.
func InvestigableTypes() []model.InvestigableType {
	return kube.InvestigableTypes(ActiveRegistry().Collectors)
}
