package engine

import (
	"github.com/glnreddy421/klew/internal/model"
)

// IncidentSummary is the flat set of fields shown in the top-left panel and web UI.
type IncidentSummary struct {
	Context        string              `json:"context"`
	Namespace      string              `json:"namespace"`
	Query          string              `json:"query"`
	Status         model.VerdictStatus `json:"status"`
	LeadingSignal  string              `json:"leadingSignal"`
	LikelyTrigger  string              `json:"likelyTrigger"`
	Confidence     float64             `json:"confidence"`
	ReadyPods      int                 `json:"readyPods"`
	UnreadyPods    int                 `json:"unreadyPods"`
	AffectedPods   int                 `json:"affectedPods"`
	Restarts       int                 `json:"restarts"`
	EndpointsReady int                 `json:"endpointsReady"`
	EndpointsTotal int                 `json:"endpointsTotal"`
	Window         int64               `json:"window"` // milliseconds
	Live           bool                `json:"live"`
}

// BuildIncidentSummary assembles the incident summary from investigation state.
func BuildIncidentSummary(st model.InvestigationState) IncidentSummary {
	b := st.Snapshot
	ready, unready, restarts, affected := 0, 0, 0, 0
	for _, p := range b.Pods {
		if p.Ready {
			ready++
		} else {
			unready++
		}
		restarts += int(p.RestartCount)
		if !p.Ready || p.RestartCount > 0 {
			affected++
		}
	}
	epReady, epTotal := 0, 0
	for _, s := range b.Services {
		epReady += s.ReadyEndpoints
		epTotal += s.TotalEndpoints
	}
	ns := st.NamespaceScope.Primary
	if st.NamespaceScope.AllNamespaces {
		ns = "*"
	}
	if ns == "" {
		ns = b.Namespace
	}
	return IncidentSummary{
		Context:        st.KubeContext.Context,
		Namespace:      ns,
		Query:          st.Query,
		Status:         st.Verdict.Status,
		LeadingSignal:  st.Verdict.LeadingSignal,
		LikelyTrigger:  st.Verdict.LikelyTrigger,
		Confidence:     st.Verdict.Confidence,
		ReadyPods:      ready,
		UnreadyPods:    unready,
		AffectedPods:   affected,
		Restarts:       restarts,
		EndpointsReady: epReady,
		EndpointsTotal: epTotal,
		Window:         st.Window,
		Live:           st.Mode == model.ModeLive && !st.Paused,
	}
}
