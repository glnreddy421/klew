package api

import (
	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/render"
)

// View is the shared investigation snapshot for desktop (and any future UI).
type View struct {
	Summary           engine.IncidentSummary `json:"summary"`
	ConfidenceLabel   string                 `json:"confidenceLabel"`
	Hypothesis        string                 `json:"hypothesis"`
	HypothesisReasons []string               `json:"hypothesisReasons"`
	HypothesisStatus  string                 `json:"hypothesisStatus"`
	HypothesisAlts    []model.Hypothesis     `json:"hypothesisAlternatives"`
	ConfidenceTrend   string                 `json:"confidenceTrend"`
	CausalChain       []string               `json:"causalChain"`
	NextChecks        []string               `json:"nextChecks"`
	Correlation       []string               `json:"correlation"`
	Signals           []model.Signal         `json:"signals"`
	Evidence          []model.EvidenceEvent  `json:"evidence"`
	LogPatterns       *model.LogPatterns     `json:"logPatterns,omitempty"`
	Watching          int                    `json:"watching"`
	ExpectedWatches   int                    `json:"expectedWatches"`
	WatchNote         string                 `json:"watchNote"`
	HypothesisChanges int                    `json:"hypothesisChanges"`
	Dropped           int64                  `json:"dropped"`
	UpdatedAt         model.Timestamp          `json:"updatedAt"`
	State             model.InvestigationState `json:"state"`
	Graph             render.GraphLayout     `json:"graph"`
}

// Build constructs a View from the current investigation state.
func Build(st model.InvestigationState) View {
	sigs := append([]model.Signal{}, st.Verdict.StrongSignals...)
	sigs = append(sigs, st.Verdict.MediumSignals...)
	sigs = append(sigs, st.Verdict.WeakSignals...)
	ev := st.LiveEvidence
	if len(ev) > maxLiveEvidence {
		ev = ev[:maxLiveEvidence]
	}
	summary := engine.BuildIncidentSummary(st)
	return View{
		Summary:           summary,
		ConfidenceLabel:   engine.ConfidenceLabel(summary.Confidence),
		Hypothesis:        st.HypothesisLabel,
		HypothesisReasons: st.HypothesisReasons,
		HypothesisStatus:  st.HypothesisStatus,
		HypothesisAlts:    st.HypothesisAlts,
		ConfidenceTrend:   st.ConfidenceTrend,
		CausalChain:       st.CausalChain,
		NextChecks:        st.NextChecks,
		Correlation:       st.Correlation,
		Signals:           sigs,
		Evidence:          ev,
		LogPatterns:       st.LogPatterns,
		Watching:          len(st.ActiveWatches),
		ExpectedWatches:   st.ExpectedWatches,
		WatchNote:         st.WatchNote,
		HypothesisChanges: st.HypothesisChanges,
		Dropped:           st.DroppedEvidence,
		UpdatedAt:         model.TimestampFrom(st.LastUpdatedAt.Time()),
		State:             CapState(st),
		Graph:             render.LayoutGraph(st.WorkloadGraph),
	}
}
