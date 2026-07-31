package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/model"
)

const kvLabelW = 17

// kv renders an aligned label/value row (labels are muted, not accent-colored).
func kv(k, v string) string {
	return padRight(labelStyle.Render(k), kvLabelW) + "  " + v
}

// kvLines renders a label/value row, stacking the value on the next line when it
// would otherwise be truncated in a narrow panel.
func kvLines(k, v string, width int) []string {
	line := kv(k, v)
	if width <= 0 || visualWidth(line) <= width {
		return []string{line}
	}
	return []string{
		padRight(labelStyle.Render(k), kvLabelW) + "  ",
		strings.Repeat(" ", kvLabelW+2) + v,
	}
}

// metaKV renders secondary metadata with reduced visual weight.
func metaKV(k, v string) string {
	return "  " + padRight(dimStyle.Render(k), kvLabelW) + "  " + v
}

// Section renders a lightweight section header (no border).
func Section(title string) string {
	return headStyle.Render(title)
}

// sectionGap is a single blank line between logical blocks inside a panel.
func sectionGap() string { return "" }

// sectionRule renders a light divider with breathing room.
func sectionRule(w int) string {
	if w < 8 {
		w = 8
	}
	return dimStyle.Render(strings.Repeat("─", w))
}

// confidencePhrase renders e.g. "High 84% ↑" colored by qualitative bucket.
func confidencePhrase(conf float64, trend string) string {
	if conf <= 0 {
		return ""
	}
	arrow := ""
	switch trend {
	case "up":
		arrow = " ↑"
	case "down":
		arrow = " ↓"
	}
	txt := fmt.Sprintf("%s %.0f%%%s", engine.ConfidenceLabel(conf), conf*100, arrow)
	switch {
	case conf >= 0.8:
		return critStyle.Render(txt)
	case conf >= 0.55:
		return warnStyle.Render(txt)
	default:
		return okStyle.Render(txt)
	}
}

// OperationalStatus returns human operational wording for the current verdict.
func OperationalStatus(st model.InvestigationState) string {
	if st.Verdict.Status == model.VerdictHealthy {
		return "Operating normally"
	}
	if phase := strings.ToLower(st.HypothesisStatus); phase != "" {
		switch phase {
		case "confirmed":
			return "Incident active"
		case "likely":
			return "Service degraded"
		case "nominal":
			return "Operating normally"
		}
	}
	switch st.Verdict.Status {
	case model.VerdictCritical:
		if endpointsDegraded(st) {
			return "Service degraded"
		}
		return "Incident active"
	case model.VerdictWarning:
		return "Service degraded"
	case model.VerdictUnknown:
		return "Investigating"
	default:
		return "Investigating"
	}
}

func endpointsDegraded(st model.InvestigationState) bool {
	for _, s := range st.Snapshot.Services {
		if s.TotalEndpoints > 0 && s.ReadyEndpoints < s.TotalEndpoints {
			return true
		}
	}
	return false
}

// InvestigationPhase is a short phase label for the global header.
func InvestigationPhase(st model.InvestigationState) string {
	if st.Paused {
		return "Paused"
	}
	if st.Mode == model.ModeBundle {
		return "Review"
	}
	if recovering(st) {
		return "Recovering"
	}
	return OperationalStatus(st)
}

func recovering(st model.InvestigationState) bool {
	if st.Verdict.Status == model.VerdictHealthy {
		return false
	}
	for _, e := range st.Timeline {
		r := strings.ToLower(e.Reason + " " + e.Message)
		if strings.Contains(r, "recovered") || strings.Contains(r, "restored") ||
			strings.Contains(r, "became ready") {
			return true
		}
	}
	return false
}

// EvidenceCount returns a deterministic observation tally for the header.
func EvidenceCount(st model.InvestigationState) int {
	n := int(st.Counters.EventsIngested + st.Counters.LogsIngested +
		st.Counters.ObjectChanges + st.Counters.MetricSamples)
	if n > 0 {
		return n
	}
	return len(st.LiveEvidence)
}

// InvestigationDuration formats elapsed investigation time.
func InvestigationDuration(st model.InvestigationState) string {
	start := st.CollectedAt.Time()
	if start.IsZero() {
		start = st.Snapshot.CollectedAt.Time()
	}
	end := st.LastUpdatedAt.Time()
	if end.IsZero() {
		end = time.Now()
	}
	d := end.Sub(start)
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// HeaderMetaLine renders compact investigation metadata for the global header.
func HeaderMetaLine(st model.InvestigationState) string {
	parts := []string{
		"phase=" + InvestigationPhase(st),
		fmt.Sprintf("evidence=%d", EvidenceCount(st)),
	}
	if st.Mode == model.ModeLive && !st.Snapshot.CollectedAt.IsZero() {
		parts = append(parts, "snapshot="+formatAge(time.Since(st.Snapshot.CollectedAt.Time())))
	}
	if st.HypothesisChanges > 0 {
		parts = append(parts, fmt.Sprintf("hypothesis_rev=%d", st.HypothesisChanges))
	}
	parts = append(parts, "duration="+InvestigationDuration(st))
	return strings.Join(parts, "  ")
}

func formatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm ago", int(d.Minutes()))
}

// formatOperationalStatus renders operational status with severity color.
func formatOperationalStatus(st model.InvestigationState) string {
	label := OperationalStatus(st)
	switch st.Verdict.Status {
	case model.VerdictCritical:
		return critStyle.Render(label)
	case model.VerdictWarning:
		return warnStyle.Render(label)
	case model.VerdictHealthy:
		return okStyle.Render(label)
	default:
		return dimStyle.Render(label)
	}
}

func statusStyled(s model.VerdictStatus) string {
	return formatOperationalStatus(model.InvestigationState{Verdict: model.Verdict{Status: s}})
}

func statusPhrase(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "confirmed", "incident active":
		return critStyle.Render("Incident active")
	case "likely", "service degraded":
		return warnStyle.Render("Service degraded")
	case "nominal", "operating normally":
		return okStyle.Render("Operating normally")
	case "recovering":
		return okStyle.Render("Recovering")
	case "paused":
		return dimStyle.Render("Paused")
	case "review":
		return dimStyle.Render("Review")
	case "collecting":
		return dimStyle.Render("Collecting")
	case "active", "investigating":
		return warnStyle.Render("Investigating")
	default:
		if s == "" {
			return dimStyle.Render("—")
		}
		return markStyle.Render(s)
	}
}
