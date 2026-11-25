package engine

import (
	"fmt"
	"strings"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// A Ready pod whose container keeps logging the same error is failing quietly:
// probes pass, nothing restarts, and nothing goes red. errorsWhileReadyFindings
// turns that into a problem so it cannot hide behind a green pod.
const (
	// minErrorLines and minErrorShare keep an occasional error from counting.
	minErrorLines = 5
	minErrorShare = 0.2
	// highErrorShare makes the problem high rather than a warning.
	highErrorShare = 0.5
	// recentErrorLines: an error must appear among the last lines read, so a
	// container that has since recovered is not reported.
	recentErrorLines = 10
	maxLogExcerpt    = 200
)

func errorsWhileReadyFindings(b model.EvidenceBundle) []model.Finding {
	pods := make(map[string]model.PodSummary, len(b.Pods))
	for _, p := range b.Pods {
		pods[p.Name] = p
	}
	var out []model.Finding
	for _, lr := range b.Logs {
		if lr.Previous {
			continue
		}
		p, ok := pods[lr.PodName]
		if !ok || p.Phase != "Running" || PodCountsAsUnready(p) {
			continue
		}
		if f, ok := erroringContainer(p, lr); ok {
			out = append(out, f)
		}
	}
	return out
}

func erroringContainer(p model.PodSummary, lr model.LogRecord) (model.Finding, bool) {
	var lines []string
	for _, l := range lr.Lines {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	errs, last := 0, -1
	counts := map[string]int{}
	latest := map[string]string{}
	for i, l := range lines {
		switch ClassifyLogSeverity(l) {
		case model.SeverityHigh, model.SeverityCritical:
		default:
			continue
		}
		errs++
		last = i
		key := normalizeMsg(l)
		counts[key]++
		latest[key] = l
	}
	share := float64(errs) / float64(max(len(lines), 1))
	if errs < minErrorLines || share < minErrorShare || last < len(lines)-recentErrorLines {
		return model.Finding{}, false
	}
	top := ""
	for key, n := range counts {
		if top == "" || n > counts[top] || (n == counts[top] && key < top) {
			top = key
		}
	}
	sev := model.SeverityWarning
	if share >= highErrorShare {
		sev = model.SeverityHigh
	}
	facts := []string{
		fmt.Sprintf("container %s is Ready but %d of its last %d log lines are errors", lr.ContainerName, errs, len(lines)),
	}
	if n := counts[top]; n > 1 {
		facts = append(facts, fmt.Sprintf("the same error repeats %d times; the pod looks healthy, so nothing restarts it", n))
	}
	facts = append(facts, model.LogExcerptPrefix+truncateRunes(strings.TrimSpace(latest[top]), maxLogExcerpt))
	return model.Finding{
		Object:   model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
		Severity: sev, State: "ErrorsWhileReady", Facts: facts,
	}, true
}
