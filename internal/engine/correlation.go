package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// Correlation is the output of the CorrelationEngine: a ranked leading signal, a
// short hypothesis label, its reasons, a confidence, ✓ correlation bullets, and
// the supporting ranked signals (evidence).
type Correlation struct {
	LeadingSignal   string
	HypothesisLabel string
	Reasons         []string
	Bullets         []string
	Confidence      float64
	Signals         []model.Signal
	Alternatives    []model.Hypothesis
	NextChecks      []string
	FixActions      []string
}

// CorrelationEngine merges live evidence signals with structural snapshot signals
// and applies deterministic correlation rules to form a leading hypothesis.
type CorrelationEngine struct{}

// Correlate combines evidence-derived and snapshot-derived signals with live events.
func (CorrelationEngine) Correlate(b model.EvidenceBundle, evidenceSignals []model.Signal, events []model.EvidenceEvent) Correlation {
	if WorkloadNominal(b) && !IncidentActive(b) {
		return recoveredCorrelation(b, evidenceSignals)
	}

	signals := append([]model.Signal{}, evidenceSignals...)
	signals = append(signals, structuralSignals(b)...)

	f := detectFlags(signals, b)

	// OOM is the root failure; BackOff/CrashLoop are restart symptoms.
	if oomPodCount(b) > 0 || (f.oom && IncidentActive(b)) {
		f.oom = true
	}

	// deterministic correlation boosts toward combined hypotheses
	rules := []struct {
		match    bool
		category string
		boost    float64
	}{
		{f.oom && (f.memLogs || f.rollout), "OOMKilled", 30},
		{f.oom, "OOMKilled", 45},
		{f.readiness && f.endpointsDropped, "Service impact", 24},
		{f.podFailures && !f.rollout && !f.oom, "CrashLoopBackOff", 18},
		{f.rollout && f.podFailures, "Rollout correlation", 20},
		{f.imagePull, "ImagePullBackOff", 26},
		{f.failedMount && f.hasConfigRefs, "Mount/config", 22},
		{f.redis && f.readiness, "Dependency failure", 18},
	}

	boost := map[string]float64{}
	matched := 0
	for _, r := range rules {
		if !r.match {
			continue
		}
		boost[r.category] += r.boost
		matched++
	}

	// rank signals by raw score for the Investigation Intelligence panel
	sort.SliceStable(signals, func(i, j int) bool {
		if signals[i].Score != signals[j].Score {
			return signals[i].Score > signals[j].Score
		}
		return signals[i].Label < signals[j].Label
	})

	// the leading signal is the single strongest observation once deterministic
	// correlation boosts are applied to its category. Info-level noise (Created,
	// Scheduled, Metric …) never leads — until a real problem appears Klew is
	// simply "collecting evidence".
	leadingIdx, bestEff := -1, -1.0
	for i, s := range signals {
		if s.Severity == model.SeverityInfo {
			continue
		}
		eff := s.Score + boost[category(s.Label)]
		if eff > bestEff {
			bestEff, leadingIdx = eff, i
		}
	}

	leadingLabel, leadingCat := "", ""
	if leadingIdx >= 0 {
		leadingLabel = signals[leadingIdx].Label
		leadingCat = category(leadingLabel)
		// float the leading signal to the top of the ranking
		s := signals[leadingIdx]
		copy(signals[1:leadingIdx+1], signals[:leadingIdx])
		signals[0] = s
	}

	label, reasons := hypothesisFor(leadingCat)
	nextChecks := nextChecksFor(leadingCat, b)
	bullets := correlationBullets(f)

	if oomPodCount(b) > 0 || (f.oom && IncidentActive(b)) {
		if rc := analyzeOOMRootCause(b, events); rc != nil {
			if rc.Label != "" {
				label = rc.Label
			}
			if len(rc.Reasons) > 0 {
				reasons = quantifyReasons(rc.Reasons, signals)
			}
			bullets = appendUniqueStrings(bullets, rc.Bullets)
			nextChecks = mergeChecks(nextChecks, rc.NextSteps)
		}
		leadingLabel = "OOMKilled"
		leadingCat = "OOMKilled"
		for i, s := range signals {
			if category(s.Label) == "OOMKilled" {
				copy(signals[1:i+1], signals[:i])
				signals[0] = s
				break
			}
		}
	}

	return Correlation{
		LeadingSignal:   leadingLabel,
		HypothesisLabel: label,
		Reasons:         reasons,
		Bullets:         bullets,
		Confidence:      confidenceFor(leadingCat, signals, matched),
		Signals:         signals,
		Alternatives:    alternativeHypotheses(signals, boost, matched, leadingCat),
		NextChecks:      nextChecks,
		FixActions:      fixActionsFor(leadingCat, b),
	}
}

func mergeChecks(base, extra []string) []string {
	out := append([]string{}, base...)
	for _, e := range extra {
		out = appendUniqueCheck(out, e)
	}
	return out
}

func appendUniqueStrings(dst []string, src []string) []string {
	for _, s := range src {
		dst = appendUniqueString(dst, s)
	}
	return dst
}

// knownCategories are the correlation categories that map to a named hypothesis.
var knownCategories = map[string]bool{
	"OOMKilled": true, "Service impact": true, "Readiness failed": true,
	"CrashLoopBackOff": true, "Rollout correlation": true, "ImagePullBackOff": true,
	"Mount/config": true, "Dependency failure": true,
}

// alternativeHypotheses returns the runner-up explanations Klew is still
// weighing (deterministic, ranked by effective score), excluding the leading
// one. Real investigations keep more than one theory open.
func alternativeHypotheses(signals []model.Signal, boost map[string]float64, matched int, leadingCat string) []model.Hypothesis {
	best := map[string]float64{}
	for _, s := range signals {
		if s.Severity == model.SeverityInfo {
			continue
		}
		cat := category(s.Label)
		if !knownCategories[cat] || cat == leadingCat {
			continue
		}
		if eff := s.Score + boost[cat]; eff > best[cat] {
			best[cat] = eff
		}
	}
	cats := make([]string, 0, len(best))
	for c := range best {
		cats = append(cats, c)
	}
	sort.Slice(cats, func(i, j int) bool {
		if best[cats[i]] != best[cats[j]] {
			return best[cats[i]] > best[cats[j]]
		}
		return cats[i] < cats[j]
	})
	var out []model.Hypothesis
	for i, c := range cats {
		if i >= 2 {
			break
		}
		label, _ := hypothesisFor(c)
		out = append(out, model.Hypothesis{
			Label:      label,
			Category:   c,
			Confidence: confidenceFor(c, signals, matched),
		})
	}
	return out
}

// quantifyReasons annotates each conceptual reason with the observation count of
// the signal that backs it, e.g. "OOMKilled" -> "OOMKilled ×24".
func quantifyReasons(reasons []string, signals []model.Signal) []string {
	if len(reasons) == 0 {
		return reasons
	}
	out := make([]string, 0, len(reasons))
	for _, r := range reasons {
		count := 0
		for _, s := range signals {
			if reasonMatchesSignal(r, s.Label) && s.Count > count {
				count = s.Count
			}
		}
		if count > 1 {
			out = append(out, fmt.Sprintf("%s ×%d", r, count))
		} else {
			out = append(out, r)
		}
	}
	return out
}

func reasonMatchesSignal(reason, label string) bool {
	r := strings.ToLower(reason)
	l := strings.ToLower(label)
	if strings.Contains(l, r) || strings.Contains(r, l) {
		return true
	}
	// share the leading keyword (e.g. "memory allocation failures" ~ "memory logs")
	if rw := strings.Fields(r); len(rw) > 0 && strings.Contains(l, rw[0]) {
		return true
	}
	return false
}

// nextChecksFor returns human-readable investigation steps for the leading hypothesis.
func nextChecksFor(cat string, b model.EvidenceBundle) []string {
	return investigationStepsFor(cat, b)
}

// hypothesisFor maps a leading category to a short hypothesis label and its
// deterministic supporting reasons (kept concise — no paragraphs).
func hypothesisFor(leading string) (string, []string) {
	switch leading {
	case "OOMKilled":
		return "Memory regression after rollout", []string{"OOMKilled", "Memory allocation failures", "New ReplicaSet"}
	case "Service impact", "Readiness failed":
		return "Service degradation", []string{"Readiness failures", "Endpoint loss"}
	case "CrashLoopBackOff":
		return "Container crash loop", []string{"CrashLoopBackOff", "High restart count", "Exit errors"}
	case "Rollout correlation":
		return "Deployment regression", []string{"Rollout", "New ReplicaSet", "Failing pods"}
	case "ImagePullBackOff":
		return "Registry issue", []string{"ImagePullBackOff"}
	case "Mount/config":
		return "Configuration issue", []string{"FailedMount", "Secret/ConfigMap ref"}
	case "Dependency failure":
		return "Dependency issue", []string{"Redis timeout", "Readiness failures"}
	case "":
		return "Collecting evidence…", nil
	default:
		return leading, nil
	}
}

// correlationBullets renders concise ✓ correlation findings from detected flags.
func correlationBullets(f corrFlags) []string {
	var out []string
	if f.rollout && f.podFailures {
		out = append(out, "✓ Rollout preceded failures")
	}
	if f.memLogs && f.oom {
		out = append(out, "✓ Memory logs precede OOM")
	}
	if f.oom && f.endpointsDropped {
		out = append(out, "✓ Endpoint loss followed OOM")
	}
	if f.readiness && f.endpointsDropped {
		out = append(out, "✓ Readiness failures dropped endpoints")
	}
	if f.imagePull {
		out = append(out, "✓ Image pull failing")
	}
	if f.failedMount && f.hasConfigRefs {
		out = append(out, "✓ Mount/config reference failing")
	}
	if f.redis {
		out = append(out, "✓ Redis timeout likely secondary")
	}
	return out
}

type corrFlags struct {
	oom, memLogs, readiness, endpointsDropped, rollout, podFailures bool
	imagePull, failedMount, hasConfigRefs, redis                    bool
}

func detectFlags(signals []model.Signal, b model.EvidenceBundle) corrFlags {
	var f corrFlags
	for _, s := range signals {
		l := strings.ToLower(s.Label)
		switch {
		case strings.Contains(l, "oom"):
			f.oom = true
		case strings.Contains(l, "readiness") || strings.Contains(l, "unhealthy") || strings.Contains(l, "probe"):
			f.readiness = true
		case strings.Contains(l, "backoff") || strings.Contains(l, "crashloop"):
			f.podFailures = true
		case strings.Contains(l, "imagepull") || strings.Contains(l, "errimage"):
			f.imagePull = true
		case strings.Contains(l, "failedmount") || strings.Contains(l, "mount"):
			f.failedMount = true
		case strings.Contains(l, "redis") || strings.Contains(l, "timeout"):
			f.redis = true
		}
		if strings.Contains(l, "memory") || strings.Contains(l, "oom") {
			if s.Source == "LOG" {
				f.memLogs = true
			}
		}
		if strings.Contains(l, "rollout") || strings.Contains(l, "scaling") || strings.Contains(l, "replicaset") {
			f.rollout = true
		}
	}
	for _, svc := range b.Services {
		if svc.ReadyEndpoints < svc.TotalEndpoints {
			f.endpointsDropped = true
		}
	}
	for _, p := range b.Pods {
		if podActivelyFailing(p) {
			f.podFailures = true
		}
	}
	if len(b.ReplicaSets) > 1 {
		f.rollout = true
	}
	if len(b.ConfigRefs) > 0 || len(b.SecretRefs) > 0 {
		f.hasConfigRefs = true
	}
	for _, e := range b.Events {
		switch e.Reason {
		case "OOMKilled", "OOMKilling":
			f.oom = true
		case "ImagePullBackOff", "ErrImagePull":
			f.imagePull = true
		case "FailedMount":
			f.failedMount = true
		}
	}
	return f
}

// structuralSignals derives signals from snapshot object state (not the stream).
func structuralSignals(b model.EvidenceBundle) []model.Signal {
	var out []model.Signal
	for _, svc := range b.Services {
		if svc.TotalEndpoints > 0 && svc.ReadyEndpoints == 0 {
			out = append(out, model.Signal{ID: "zero_endpoints", Label: "Zero ready endpoints", Severity: model.SeverityCritical,
				Source: "OBJECT", Count: 1, Score: 70, Strength: "strong", Confidence: 0.9,
				Evidence: fmt.Sprintf("service %s has 0/%d ready", svc.Name, svc.TotalEndpoints)})
		} else if svc.TotalEndpoints > 0 && svc.ReadyEndpoints < svc.TotalEndpoints {
			out = append(out, model.Signal{ID: "few_endpoints", Label: "Reduced endpoints", Severity: model.SeverityHigh,
				Source: "OBJECT", Count: 1, Score: 40, Strength: "medium", Confidence: 0.8,
				Evidence: fmt.Sprintf("service %s has %d/%d ready", svc.Name, svc.ReadyEndpoints, svc.TotalEndpoints)})
		}
	}
	restarts := 0
	for _, p := range b.Pods {
		restarts += int(p.RestartCount)
	}
	if restarts >= 5 && IncidentActive(b) {
		out = append(out, model.Signal{ID: "high_restarts", Label: "High restart count", Severity: model.SeverityHigh,
			Source: "OBJECT", Count: restarts, Score: 45, Strength: "medium", Confidence: 0.85,
			Evidence: fmt.Sprintf("%d restarts across pods", restarts)})
	}
	return out
}

func category(label string) string {
	l := strings.ToLower(label)
	switch {
	case strings.Contains(l, "oom"):
		return "OOMKilled"
	case strings.Contains(l, "endpoint"):
		return "Service impact"
	case strings.Contains(l, "readiness") || strings.Contains(l, "unhealthy") || strings.Contains(l, "probe"):
		return "Readiness failed"
	case strings.Contains(l, "imagepull") || strings.Contains(l, "errimage"):
		return "ImagePullBackOff"
	case strings.Contains(l, "mount"):
		return "Mount/config"
	case strings.Contains(l, "redis") || strings.Contains(l, "timeout"):
		return "Dependency failure"
	case strings.Contains(l, "crashloop"):
		return "CrashLoopBackOff"
	case strings.Contains(l, "restart"):
		return "CrashLoopBackOff"
	case strings.Contains(l, "backoff"):
		return "CrashLoopBackOff"
	default:
		return label
	}
}

func confidenceFor(leading string, signals []model.Signal, correlations int) float64 {
	base := 0.4
	for _, s := range signals {
		if category(s.Label) == leading {
			switch s.Severity {
			case model.SeverityCritical:
				base = 0.75
			case model.SeverityHigh:
				if base < 0.6 {
					base = 0.6
				}
			}
		}
	}
	base += float64(correlations) * 0.05
	// Never claim certainty — deterministic evidence is never 100%.
	if base > 0.95 {
		base = 0.95
	}
	return clamp01(base)
}

func recoveredCorrelation(b model.EvidenceBundle, evidenceSignals []model.Signal) Correlation {
	signals := demoteHistoricalSignals(evidenceSignals)
	bullets := []string{"✓ All scoped pods are ready", "✓ No active crash, OOM, or endpoint failures"}
	next := investigationStepsFor("", b)
	if hadRecentFailureSignals(evidenceSignals) {
		bullets = append(bullets, "· Recent failure evidence in stream — incident appears cleared")
	}
	return Correlation{
		LeadingSignal:   "",
		HypothesisLabel: "Workload operating normally",
		Reasons:         []string{"Pods ready", "Endpoints healthy", "No active failure state"},
		Bullets:         bullets,
		Confidence:      0.85,
		Signals:         signals,
		NextChecks:      next,
	}
}

func demoteHistoricalSignals(signals []model.Signal) []model.Signal {
	if len(signals) == 0 {
		return nil
	}
	out := append([]model.Signal(nil), signals...)
	for i := range out {
		if severityRank(out[i].Severity) >= severityRank(model.SeverityHigh) {
			out[i].Severity = model.SeverityInfo
			out[i].Score *= 0.15
			out[i].Strength = "weak"
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func hadRecentFailureSignals(signals []model.Signal) bool {
	for _, s := range signals {
		if severityRank(s.Severity) >= severityRank(model.SeverityHigh) {
			return true
		}
	}
	return false
}

// ConfidenceLabel maps a confidence to a qualitative bucket.
func ConfidenceLabel(f float64) string {
	switch {
	case f >= 0.8:
		return "High"
	case f >= 0.55:
		return "Medium"
	case f > 0:
		return "Low"
	default:
		return "—"
	}
}
