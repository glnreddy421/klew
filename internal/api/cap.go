package api

import "github.com/glnreddy421/klew/internal/model"

const (
	maxLiveEvidence = 200
	maxTimeline     = 200
	maxLogs         = 50
	maxEvents       = 100
)

// CapState returns a copy of investigation state sized for UI transport.
func CapState(st model.InvestigationState) model.InvestigationState {
	out := st.Clone()
	if len(out.LiveEvidence) > maxLiveEvidence {
		out.LiveEvidence = out.LiveEvidence[len(out.LiveEvidence)-maxLiveEvidence:]
	}
	if len(out.Timeline) > maxTimeline {
		out.Timeline = out.Timeline[len(out.Timeline)-maxTimeline:]
	}
	if len(out.Snapshot.Logs) > maxLogs {
		out.Snapshot.Logs = out.Snapshot.Logs[len(out.Snapshot.Logs)-maxLogs:]
	}
	if len(out.Snapshot.Events) > maxEvents {
		out.Snapshot.Events = capEvents(out.Snapshot.Events, maxEvents)
	}
	return out
}

// capEvents keeps the newest max events but prefers Warnings, so older recorded
// Warnings are not pushed out by a burst of Normal events. Order is preserved.
func capEvents(events []model.EventRecord, max int) []model.EventRecord {
	keep := make([]bool, len(events))
	n := 0
	for pass := 0; pass < 2 && n < max; pass++ {
		for i := len(events) - 1; i >= 0 && n < max; i-- {
			if keep[i] {
				continue
			}
			if pass == 0 && events[i].Type != "Warning" {
				continue
			}
			keep[i] = true
			n++
		}
	}
	out := make([]model.EventRecord, 0, n)
	for i, e := range events {
		if keep[i] {
			out = append(out, e)
		}
	}
	return out
}
