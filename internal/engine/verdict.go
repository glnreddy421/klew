package engine

import (
	"fmt"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// GenerateVerdict produces deterministic conclusion with confidence.
func GenerateVerdict(b model.EvidenceBundle, timeline []model.TimelineEvent, signals []model.Signal) model.Verdict {
	v := model.Verdict{
		Status:                model.VerdictUnknown,
		Confidence:            0.4,
		Summary:               "Insufficient evidence for a definitive verdict.",
		RecommendedNextChecks: defaultChecks(b),
	}

	for _, s := range signals {
		switch s.Strength {
		case "strong":
			v.StrongSignals = append(v.StrongSignals, s)
		case "medium":
			v.MediumSignals = append(v.MediumSignals, s)
		default:
			v.WeakSignals = append(v.WeakSignals, s)
		}
		if s.ObjectRef.Name != "" {
			v.AffectedObjects = appendUniqueRef(v.AffectedObjects, s.ObjectRef)
		}
	}

	if len(v.StrongSignals) > 0 {
		v.Status = model.VerdictCritical
		v.Confidence = 0.85
		v.LeadingSignal = v.StrongSignals[0].Label
	} else if len(v.MediumSignals) > 0 {
		v.Status = model.VerdictWarning
		v.Confidence = 0.65
		v.LeadingSignal = v.MediumSignals[0].Label
	} else if allPodsHealthy(b) {
		v.Status = model.VerdictHealthy
		v.Confidence = 0.75
		v.Summary = "No strong failure signals detected in collected evidence."
	}

	trigger, conf := detectTrigger(timeline)
	v.LikelyTrigger = trigger
	if conf > v.Confidence {
		v.Confidence = conf
	}

	v.Summary = buildSummary(b, v)
	if len(b.Warnings) > 0 {
		v.RecommendedNextChecks = append(v.RecommendedNextChecks, "Review collection warnings for missing permissions/data.")
	}
	return v
}

func detectTrigger(timeline []model.TimelineEvent) (string, float64) {
	var best *model.TimelineEvent
	for i := range timeline {
		e := &timeline[i]
		if severityRank(e.Severity) < 3 {
			continue
		}
		if best == nil || e.Timestamp.Before(best.Timestamp) || (e.Timestamp.Equal(best.Timestamp) && e.Confidence > best.Confidence) {
			best = e
		}
	}
	if best == nil {
		return "No high-confidence trigger identified", 0.4
	}
	return fmt.Sprintf("%s %s/%s: %s (%s)", best.Timestamp.Time().Format("15:04:05"), best.SourceKind, best.SourceName, best.Reason, best.Message), best.Confidence
}

func buildSummary(b model.EvidenceBundle, v model.Verdict) string {
	wl := "workload"
	if len(b.Workloads) > 0 {
		wl = b.Workloads[0].Kind + "/" + b.Workloads[0].Name
	}
	parts := []string{
		fmt.Sprintf("Investigation for %s in namespace %s.", wl, b.Namespace),
		fmt.Sprintf("Status: %s (confidence %.0f%%).", v.Status, v.Confidence*100),
	}
	if v.LikelyTrigger != "" {
		parts = append(parts, "Likely trigger: "+v.LikelyTrigger)
	}
	if len(v.StrongSignals) > 0 {
		parts = append(parts, fmt.Sprintf("Strong signals: %d.", len(v.StrongSignals)))
	}
	return strings.Join(parts, " ")
}

func allPodsHealthy(b model.EvidenceBundle) bool {
	if len(b.Pods) == 0 {
		return false
	}
	for _, p := range b.Pods {
		if !p.Ready || p.RestartCount > 0 {
			return false
		}
	}
	return true
}

func defaultChecks(b model.EvidenceBundle) []string {
	return investigationStepsFor("", b)
}

func appendUniqueRef(refs []model.ObjectRef, ref model.ObjectRef) []model.ObjectRef {
	for _, r := range refs {
		if r.Kind == ref.Kind && r.Name == ref.Name && r.Namespace == ref.Namespace {
			return refs
		}
	}
	return append(refs, ref)
}
