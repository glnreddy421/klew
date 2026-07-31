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
		out.Snapshot.Events = out.Snapshot.Events[len(out.Snapshot.Events)-maxEvents:]
	}
	return out
}
