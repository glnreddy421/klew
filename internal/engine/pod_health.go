package engine

import (
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// PodTerminalSuccess reports Job/CronJob pods that finished successfully.
// They are not "unready" in the operational sense — Ready=false is normal.
func PodTerminalSuccess(p model.PodSummary) bool {
	phase := strings.ToLower(p.Phase)
	return phase == "succeeded" || phase == "completed"
}

func podTerminalFailure(p model.PodSummary) bool {
	return strings.EqualFold(p.Phase, "Failed")
}

// PodCountsAsUnready is true when a pod should affect live health / degraded UI.
func PodCountsAsUnready(p model.PodSummary) bool {
	if PodTerminalSuccess(p) {
		return false
	}
	if podTerminalFailure(p) {
		return true
	}
	return !p.Ready || podActivelyFailing(p)
}
