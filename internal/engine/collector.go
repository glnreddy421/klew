package engine

import (
	"github.com/glnreddy421/klew/internal/model"
)

// Analyze runs the deterministic investigation pipeline on a bundle.
func Analyze(bundle model.EvidenceBundle) model.Investigation {
	timeline := BuildTimeline(bundle)
	graph := BuildGraph(bundle)
	signals := ScoreSignals(bundle)
	verdict := GenerateVerdict(bundle, timeline, signals)
	return model.Investigation{
		Bundle:   bundle,
		Timeline: timeline,
		Graph:    graph,
		Verdict:  verdict,
	}
}
