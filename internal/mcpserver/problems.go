package mcpserver

import (
	"strings"

	"github.com/glnreddy421/klew/v2/internal/model"
)

const maxProblems = 20

// problemBrief is one entry of the engine's problem list, with facts cleaned.
type problemBrief struct {
	Object   string         `json:"object"`
	Severity model.Severity `json:"severity"`
	State    string         `json:"state"`
	Facts    []string       `json:"facts,omitempty"`
	Pods     int            `json:"pods,omitempty"`
	Unready  int            `json:"unready,omitempty"`
	Restarts int32          `json:"restarts,omitempty"`
	Rules    []string       `json:"rules,omitempty"`
}

func briefProblem(p model.Problem) problemBrief {
	return problemBrief{
		Object: p.Object, Severity: p.Severity, State: p.State, Facts: mapSlice(p.Facts, clean),
		Pods: p.Pods, Unready: p.Unready, Restarts: p.Restarts, Rules: p.Rules,
	}
}

// problem drops facts that quote a container's log line when log samples are off.
func (sh shaper) problem(p model.Problem) problemBrief {
	out := briefProblem(p)
	if !sh.noLogSamples {
		return out
	}
	kept := out.Facts[:0]
	for _, f := range out.Facts {
		if !strings.HasPrefix(f, model.LogExcerptPrefix) {
			kept = append(kept, f)
		}
	}
	out.Facts = kept
	return out
}
