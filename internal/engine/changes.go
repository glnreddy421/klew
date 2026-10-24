package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// changeLeadWindow: a change up to this long before the first incident is
// pinned even when it is older than the investigation window.
const changeLeadWindow = time.Hour

// PinChange picks the workload change to pin at the top of Overview: the most
// recent change inside the investigation window, or within changeLeadWindow
// before the first incident started. Returns nil when none qualifies.
func PinChange(markers []model.WorkloadChangeMarker, incidents []model.Incident, now time.Time, window time.Duration) *model.PinnedChange {
	var firstIncident time.Time
	for _, inc := range incidents {
		if t := inc.Start.Time(); !t.IsZero() && (firstIncident.IsZero() || t.Before(firstIncident)) {
			firstIncident = t
		}
	}
	var best *model.WorkloadChangeMarker
	var bestT time.Time
	for i := range markers {
		m := &markers[i]
		t := m.Time.Time()
		if t.IsZero() || t.After(now) {
			continue
		}
		inWindow := !t.Before(now.Add(-window))
		beforeIncident := !firstIncident.IsZero() && !t.After(firstIncident) && firstIncident.Sub(t) <= changeLeadWindow
		if !inWindow && !beforeIncident {
			continue
		}
		if best == nil || t.After(bestT) {
			best, bestT = m, t
		}
	}
	if best == nil {
		return nil
	}
	pin := &model.PinnedChange{Change: *best}
	var lead string
	if !firstIncident.IsZero() && !bestT.After(firstIncident) {
		pin.BeforeIncident = true
		lead = fmt.Sprintf("changed %s before the incident started", humanDur(firstIncident.Sub(bestT)))
	} else {
		lead = fmt.Sprintf("changed %s ago", humanDur(now.Sub(bestT)))
	}
	detail := fmt.Sprintf("rev %d", best.Revision)
	if len(best.Areas) > 0 {
		detail += " (" + strings.Join(best.Areas, ", ") + ")"
	} else if best.Source == model.ChangeSourceRollback {
		detail += " (rollback)"
	}
	pin.Text = fmt.Sprintf("%s %s: %s", best.Name, lead, detail)
	return pin
}

// humanDur: "<1m", "12m", "1h 5m", "3h".
func humanDur(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// changeReason labels an enriched rollout timeline entry, e.g. "Rollout: image, env".
func changeReason(m model.WorkloadChangeMarker) string {
	switch {
	case m.Source == model.ChangeSourceRollback:
		return "Rollback"
	case m.Source == model.ChangeSourceRestart:
		return "Rollout restart"
	case len(m.Areas) > 0:
		return "Rollout: " + strings.Join(m.Areas, ", ")
	}
	return "Rollout"
}
