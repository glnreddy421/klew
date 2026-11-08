package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// Rule turns collected evidence into signals, which drive the verdict, and
// findings, which list what is wrong with each object. Rules only read the
// bundle.
type Rule interface {
	Name() string
	Evaluate(b model.EvidenceBundle) RuleResult
}

// RuleResult is what one rule observed.
type RuleResult struct {
	Signals  []model.Signal
	Findings []model.Finding
	// Warnings describe evaluation problems, such as objects a rule could not read.
	Warnings []string
}

type ruleFunc struct {
	name string
	fn   func(b model.EvidenceBundle) RuleResult
}

func (r ruleFunc) Name() string                               { return r.name }
func (r ruleFunc) Evaluate(b model.EvidenceBundle) RuleResult { return r.fn(b) }

// NewRule wraps fn as a named rule.
func NewRule(name string, fn func(b model.EvidenceBundle) RuleResult) Rule {
	return ruleFunc{name: name, fn: fn}
}

func signalRule(name string, fn func(b model.EvidenceBundle) []model.Signal) Rule {
	return NewRule(name, func(b model.EvidenceBundle) RuleResult { return RuleResult{Signals: fn(b)} })
}

func findingRule(name string, fn func(b model.EvidenceBundle) []model.Finding) Rule {
	return NewRule(name, func(b model.EvidenceBundle) RuleResult { return RuleResult{Findings: fn(b)} })
}

// DefaultRules returns Klew's built-in rules. Signal rules come first, in the
// order the verdict has always scored them.
func DefaultRules() []Rule {
	rules := []Rule{
		signalRule("admission", scoreAdmissionSignals),
		signalRule("containers", containerSignals),
		signalRule("events", eventSignals),
		signalRule("service-endpoints", serviceSignals),
		signalRule("nodes", nodeSignals),
		signalRule("hpa", hpaSignals),
		signalRule("rollout", rolloutSignals),
		signalRule("metrics", metricsSignals),
		findingRule("pod-state", podStateFindings),
		findingRule("liveness-restarts", livenessFindings),
		findingRule("pod-creation-rejected", podCreationFindings),
		findingRule("job-failed", jobFindings),
		findingRule("deployment-conditions", deploymentConditionFindings),
		findingRule("service-no-pods", serviceSelectorFindings),
		findingRule("service-unready", serviceUnreadyFindings),
	}
	rules = append(rules, workloadRules()...)
	return append(rules, resourceRules()...)
}

// Registry holds the collectors and rules an investigation runs: the defaults
// plus any user-defined ones.
type Registry struct {
	// Collectors run after kube.DefaultCollectors.
	Collectors []kube.EvidenceCollector
	Rules      []Rule
	// Warnings describe extensions that failed to load or evaluate.
	Warnings []string
	// Entries lists every extension document and whether it loaded.
	Entries []ExtensionEntry
}

// DefaultRegistry has Klew's built-in rules and no extra collectors.
func DefaultRegistry() *Registry {
	return &Registry{Rules: DefaultRules()}
}

var active atomic.Pointer[Registry]

// SetRegistry makes r the registry every investigation uses. The desktop app
// and the klew command call it once at startup; nil restores the default.
func SetRegistry(r *Registry) {
	active.Store(r)
}

// ActiveRegistry returns the registry set by SetRegistry, or the default.
func ActiveRegistry() *Registry {
	if r := active.Load(); r != nil {
		return r
	}
	return DefaultRegistry()
}

// CollectorList returns the default collectors followed by the registry's own.
func (r *Registry) CollectorList() []kube.EvidenceCollector {
	return append(kube.DefaultCollectors(), r.Collectors...)
}

// Evaluate runs every rule on b.
func (r *Registry) Evaluate(b model.EvidenceBundle) RuleResult {
	var out RuleResult
	for _, rule := range r.Rules {
		res := safeEvaluate(rule, b)
		out.Signals = append(out.Signals, res.Signals...)
		out.Warnings = append(out.Warnings, res.Warnings...)
		for _, f := range res.Findings {
			if f.Rule == "" {
				f.Rule = rule.Name()
			}
			out.Findings = append(out.Findings, f)
		}
	}
	return out
}

// safeEvaluate runs one rule, turning a panic into a warning so a faulty rule
// cannot stop the investigation.
func safeEvaluate(rule Rule, b model.EvidenceBundle) (res RuleResult) {
	defer func() {
		if p := recover(); p != nil {
			res = RuleResult{Warnings: []string{fmt.Sprintf("rule %s failed and was skipped: %v", rule.Name(), p)}}
		}
	}()
	return rule.Evaluate(b)
}

const maxProblemFacts = 8

// maxFactLen caps one fact; event and condition messages can be long.
const maxFactLen = 500

// BuildProblems groups findings by the workload that owns each object, worst
// first. Pod, unready and restart counts cover every pod the workload owns.
func BuildProblems(b model.EvidenceBundle, findings []model.Finding) []model.Problem {
	owners := newOwnerIndex(b)
	var out []model.Problem
	index := map[string]int{}
	// stateFromPod marks problems whose state came from a pod finding. At equal
	// severity a finding about the workload itself (a failed Job, a rejected
	// ReplicaSet) names the state better than one of its pods.
	stateFromPod := map[int]bool{}
	for _, f := range findings {
		key := owners.of(f.Object)
		i, ok := index[key]
		if !ok {
			index[key] = len(out)
			out = append(out, model.Problem{Object: key, Severity: f.Severity, State: f.State})
			i = len(out) - 1
			stateFromPod[i] = f.Object.Kind == "Pod"
		}
		p := &out[i]
		higher := severityRank(f.Severity) > severityRank(p.Severity)
		sameButOwner := severityRank(f.Severity) == severityRank(p.Severity) && stateFromPod[i] && f.Object.Kind != "Pod"
		if higher || sameButOwner {
			p.Severity, p.State = f.Severity, f.State
			stateFromPod[i] = f.Object.Kind == "Pod"
		}
		p.Rules = appendUniqueString(p.Rules, f.Rule)
		for _, fact := range f.Facts {
			if len(p.Facts) < maxProblemFacts {
				p.Facts = appendUniqueString(p.Facts, truncateRunes(fact, maxFactLen))
			}
		}
	}
	for _, pod := range b.Pods {
		i, ok := index[owners.pod[pod.Name]]
		if !ok {
			continue
		}
		out[i].Pods++
		out[i].Restarts += pod.RestartCount
		if PodCountsAsUnready(pod) {
			out[i].Unready++
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return severityRank(out[i].Severity) > severityRank(out[j].Severity)
	})
	return out
}

// ownerIndex maps objects to the workload that owns them, built once per
// bundle so grouping stays linear in the number of pods and findings.
type ownerIndex struct {
	pod       map[string]string // pod name -> owner key
	rs        map[string]string // ReplicaSet -> owner key
	job       map[string]string // Job -> owner key
	claimUser map[string]string // PVC -> owner key of a pod that mounts it
}

func newOwnerIndex(b model.EvidenceBundle) ownerIndex {
	idx := ownerIndex{pod: map[string]string{}, rs: map[string]string{}, job: map[string]string{}, claimUser: map[string]string{}}
	for _, rs := range b.ReplicaSets {
		if rs.DeploymentOwner != "" {
			idx.rs[rs.Name] = "Deployment/" + rs.DeploymentOwner
		}
	}
	for _, j := range b.Jobs {
		if j.CronJob != "" {
			idx.job[j.Name] = "CronJob/" + j.CronJob
		}
	}
	for _, p := range b.Pods {
		key := idx.podOwner(p)
		idx.pod[p.Name] = key
		for _, c := range p.PVCRefs {
			if _, seen := idx.claimUser[c]; !seen {
				idx.claimUser[c] = key
			}
		}
	}
	return idx
}

// podOwner mirrors PodWorkload using the index's maps.
func (idx ownerIndex) podOwner(p model.PodSummary) string {
	if len(p.OwnerRefs) == 0 {
		return "Pod/" + p.Name
	}
	o := p.OwnerRefs[0]
	switch o.Kind {
	case "StatefulSet", "DaemonSet":
		return o.Kind + "/" + o.Name
	case "Job":
		if key, ok := idx.job[o.Name]; ok {
			return key
		}
		return "Job/" + o.Name
	case "ReplicaSet":
		if key, ok := idx.rs[o.Name]; ok {
			return key
		}
		if hash := p.Labels["pod-template-hash"]; hash != "" && strings.HasSuffix(o.Name, "-"+hash) {
			return "Deployment/" + strings.TrimSuffix(o.Name, "-"+hash)
		}
	}
	return "Pod/" + p.Name
}

// of names the workload that owns ref, or ref itself.
func (idx ownerIndex) of(ref model.ObjectRef) string {
	var key string
	switch ref.Kind {
	case "Pod":
		key = idx.pod[ref.Name]
	case "ReplicaSet":
		key = idx.rs[ref.Name]
	case "Job":
		key = idx.job[ref.Name]
	case "PersistentVolumeClaim":
		// A claim belongs with the workload whose pods mount it.
		key = idx.claimUser[ref.Name]
	}
	if key == "" {
		key = ref.Kind + "/" + ref.Name
	}
	return key
}

// findingSignals turns serious findings about objects other than pods into
// signals. Pod state already reaches the verdict through container and event
// signals; Services, ReplicaSets, Jobs and custom kinds otherwise would not.
func findingSignals(findings []model.Finding) []model.Signal {
	var out []model.Signal
	for _, f := range findings {
		if f.Object.Kind == "Pod" || severityRank(f.Severity) < severityRank(model.SeverityHigh) {
			continue
		}
		score := 65.0
		if f.Severity == model.SeverityCritical {
			score = 80
		}
		evidence := f.Object.Kind + "/" + f.Object.Name
		if len(f.Facts) > 0 {
			evidence += ": " + f.Facts[0]
		}
		out = append(out, model.Signal{
			ID: "finding_" + f.Rule, Label: f.State, Severity: f.Severity, Strength: "strong",
			Source: "OBJECT", Count: 1, Score: score, Confidence: 0.85, Evidence: evidence, ObjectRef: f.Object,
		})
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

const headlineWorst = 3

// ProblemsHeadline summarises problems in one line, naming the worst few.
func ProblemsHeadline(problems []model.Problem) string {
	if len(problems) == 0 {
		return ""
	}
	counts := map[model.Severity]int{}
	for _, p := range problems {
		counts[p.Severity]++
	}
	var parts []string
	for _, sev := range []model.Severity{model.SeverityCritical, model.SeverityHigh, model.SeverityWarning, model.SeverityInfo} {
		if n := counts[sev]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, sev))
		}
	}
	noun := "problems"
	if len(problems) == 1 {
		noun = "problem"
	}
	var worst []string
	for _, p := range problems[:min(headlineWorst, len(problems))] {
		worst = append(worst, fmt.Sprintf("%s (%s)", p.Object, p.State))
	}
	more := ""
	if extra := len(problems) - headlineWorst; extra > 0 {
		more = fmt.Sprintf(" and %d more", extra)
	}
	return fmt.Sprintf("%d %s: %s. Worst: %s%s", len(problems), noun, strings.Join(parts, ", "), strings.Join(worst, ", "), more)
}

// genericLeading reports whether label says only that something was logged
// or observed, not what is wrong.
func genericLeading(label string) bool {
	switch label {
	case "log signal", "signal", "metric", "Warning logs", "Error logs", "Info logs":
		return true
	}
	return false
}

// leadWithWorstProblem replaces a generic leading signal with the worst
// serious problem, so the verdict names what is broken.
func leadWithWorstProblem(corr *Correlation, problems []model.Problem) {
	if len(problems) == 0 || !genericLeading(corr.LeadingSignal) {
		return
	}
	worst := problems[0]
	if severityRank(worst.Severity) < severityRank(model.SeverityHigh) {
		return
	}
	corr.LeadingSignal = worst.State
	corr.HypothesisLabel = fmt.Sprintf("%s on %s", worst.State, worst.Object)
	corr.Reasons = append([]string(nil), worst.Facts[:min(2, len(worst.Facts))]...)
}
