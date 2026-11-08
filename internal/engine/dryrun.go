package engine

import (
	"fmt"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// DryRun is a user rule's result on every object it saw, without building an
// investigation. Each object is checked exactly as an investigation checks it.
type DryRun struct {
	Rule     string      `json:"rule"`
	Resource string      `json:"resource"`
	Checks   []RuleCheck `json:"checks"`
	// Warnings are the ones the rule would add to an investigation.
	Warnings []string `json:"warnings,omitempty"`
}

// Matched counts the objects the rule reported.
func (d DryRun) Matched() int {
	n := 0
	for _, c := range d.Checks {
		if c.Matched {
			n++
		}
	}
	return n
}

// UserRuleNames lists the registry's user rules.
func (r *Registry) UserRuleNames() []string {
	var out []string
	for _, rule := range r.Rules {
		if cr, ok := rule.(*celRule); ok {
			out = append(out, cr.name)
		}
	}
	sort.Strings(out)
	return out
}

func (r *Registry) userRule(name string) (*celRule, error) {
	for _, rule := range r.Rules {
		if cr, ok := rule.(*celRule); ok && cr.name == name {
			return cr, nil
		}
	}
	names := r.UserRuleNames()
	if len(names) == 0 {
		return nil, fmt.Errorf("no user rule %q: no user rules are loaded", name)
	}
	return nil, fmt.Errorf("no user rule %q; loaded rules: %s", name, strings.Join(names, ", "))
}

// RuleResource is the resource a user rule reads, and whether it is one of
// Klew's built-in summaries (pods, services, …) rather than Kubernetes objects.
func (r *Registry) RuleResource(name string) (resource string, summary bool, err error) {
	cr, err := r.userRule(name)
	if err != nil {
		return "", false, err
	}
	_, summary = ruleResources[cr.spec.Resource]
	return cr.spec.Resource, summary, nil
}

// DryRunRule runs the named user rule on b.
func (r *Registry) DryRunRule(name string, b model.EvidenceBundle) (out DryRun, err error) {
	cr, err := r.userRule(name)
	if err != nil {
		return DryRun{}, err
	}
	out = DryRun{Rule: name, Resource: cr.spec.Resource}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("rule %s failed: %v", name, p)
		}
	}()
	for _, obj := range ruleObjects(b, cr.spec.Resource) {
		out.Checks = append(out.Checks, cr.check(obj, b.Namespace))
	}
	out.Warnings = safeEvaluate(cr, b).Warnings
	return out, nil
}

// BundleFromObjects is an evidence bundle holding objects as the collector
// named resource would, for dry runs against saved objects (kubectl get -o
// yaml). It refuses Klew's built-in summaries, whose shape differs from the
// Kubernetes objects in such files.
func BundleFromObjects(resource, namespace string, objects []map[string]any) (model.EvidenceBundle, error) {
	if _, summary := ruleResources[resource]; summary {
		return model.EvidenceBundle{}, fmt.Errorf("rule reads Klew's %s summary, not Kubernetes objects; dry-run it against a namespace instead", resource)
	}
	return model.EvidenceBundle{Namespace: namespace, Resources: map[string][]map[string]any{resource: objects}}, nil
}

// ParseObjects reads Kubernetes objects from YAML or JSON: one object, several
// documents separated by ---, or a List (kubectl get -o yaml).
func ParseObjects(data []byte) ([]map[string]any, error) {
	var out []map[string]any
	for i, doc := range docSeparator.Split(string(data), -1) {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var obj map[string]any
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			return nil, fmt.Errorf("document %d: %w", i+1, err)
		}
		if obj == nil {
			continue
		}
		if items, ok := obj["items"].([]any); ok && strings.HasSuffix(fmt.Sprint(obj["kind"]), "List") {
			for _, it := range items {
				if m, ok := it.(map[string]any); ok {
					out = append(out, m)
				}
			}
			continue
		}
		out = append(out, obj)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no objects found")
	}
	return out, nil
}

// DryRunAll runs every user rule on b, in name order.
func (r *Registry) DryRunAll(b model.EvidenceBundle) []DryRun {
	var out []DryRun
	for _, name := range r.UserRuleNames() {
		run, err := r.DryRunRule(name, b)
		if err != nil {
			run = DryRun{Rule: name, Warnings: []string{err.Error()}}
		}
		out = append(out, run)
	}
	return out
}
