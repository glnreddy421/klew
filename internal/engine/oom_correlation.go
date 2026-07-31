package engine

import (
	"fmt"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// OOMRootCause is deterministic context for why memory was exceeded.
type OOMRootCause struct {
	Category  string
	Label     string
	Reasons   []string
	Bullets   []string
	NextSteps []string
}

// analyzeOOMRootCause correlates OOMKilled pod state with logs, events, and spec.
func analyzeOOMRootCause(b model.EvidenceBundle, events []model.EvidenceEvent) *OOMRootCause {
	if oomPodCount(b) == 0 && !bundleHasOOMEvent(b) {
		return nil
	}

	lines := collectOOMContextLines(b, events)
	cmdText := containerCommandText(b)
	limit := podMemoryLimitMi(b, failingPodName(b))

	rc := &OOMRootCause{Category: "unknown", Label: "Memory limit exceeded"}

	switch {
	case matchesStartupAllocation(lines, cmdText):
		rc.Category = "startup_allocation"
		rc.Label = "Memory limit exceeded — startup workload allocates beyond limit"
		rc.Reasons = []string{"OOMKilled", "Startup memory allocation", "Command override"}
		rc.Bullets = []string{"✓ Logs or container command allocate memory before exit"}
		if snippet := firstMatchingLine(lines, startupAllocationPatterns); snippet != "" {
			rc.Bullets = append(rc.Bullets, "✓ Log: "+truncSnippet(snippet, 72))
		}
		rc.NextSteps = []string{
			"Inspect the container startup command for memory-heavy operations (dd, bulk load, cache warm)",
			"Confirm whether the allocation is intentional (fault injection, job) or misconfiguration",
		}
		if limit > 0 {
			rc.NextSteps = append(rc.NextSteps,
				fmt.Sprintf("Compare the %s limit against what the startup command allocates", formatMemMi(limit)))
		}

	case hasRecentRollout(b):
		rc.Category = "limit_regression"
		rc.Label = "Memory limit exceeded after rollout change"
		rc.Reasons = []string{"OOMKilled", "New ReplicaSet", "Recent rollout"}
		rc.Bullets = []string{"✓ OOM started on pods from a new revision"}
		if matchesInProcessAllocation(lines) {
			rc.Bullets = append(rc.Bullets, "✓ Application logs report allocation failure on the new revision")
		}
		rc.NextSteps = []string{
			"Diff memory requests/limits and image between the failing and last healthy revision",
			"Roll back if the new revision introduced higher baseline memory use",
		}

	case matchesInProcessAllocation(lines):
		rc.Category = "in_process_allocation"
		rc.Label = "Memory limit exceeded during request processing"
		rc.Reasons = []string{"OOMKilled", "In-process allocation failure"}
		rc.Bullets = []string{"✓ Application logs report allocation failure before OOM"}
		if snippet := firstMatchingLine(lines, inProcessAllocationPatterns); snippet != "" {
			rc.Bullets = append(rc.Bullets, "✓ Log: "+truncSnippet(snippet, 72))
		}
		rc.NextSteps = []string{
			"Find the code path logging allocation failures — likely the OOM trigger under load",
			"Check for a traffic spike or larger payload sizes coinciding with the kills",
		}

	case matchesMemoryLeak(lines):
		rc.Category = "memory_leak"
		rc.Label = "Suspected memory leak"
		rc.Reasons = []string{"OOMKilled", "Memory growth in logs"}
		rc.Bullets = []string{"✓ Logs suggest gradual memory growth before OOM"}
		rc.NextSteps = []string{
			"Compare restart timing — repeated OOM at similar uptime suggests leak not burst",
			"Profile heap growth or enable live metrics to confirm rising usage between restarts",
		}

	default:
		rc.Reasons = []string{"OOMKilled", "Exit code 137"}
		if limit > 0 {
			rc.Reasons = append(rc.Reasons, fmt.Sprintf("Limit %s", formatMemMi(limit)))
		}
		rc.NextSteps = []string{
			"Review logs from the container immediately before each OOM kill",
			"Check whether limits, traffic, or dependencies changed recently",
		}
	}

	if anyLineMatches(lines, append(startupAllocationPatterns, inProcessAllocationPatterns...)) {
		rc.Bullets = appendUniqueString(rc.Bullets, "✓ Memory-related logs precede OOM")
	}
	if hasRecentRollout(b) && rc.Category != "limit_regression" {
		rc.Bullets = appendUniqueString(rc.Bullets, "✓ Rollout coincides with OOM pods")
	}

	return rc
}

var startupAllocationPatterns = []string{
	"dd if=", "allocating memory", "/tmp/leak", "/dev/shm", "head -c", "stress",
}
var inProcessAllocationPatterns = []string{
	"out of memory allocating", "cannot allocate", "memory allocation failed", "runtime: out of memory",
}
var memoryLeakPatterns = []string{
	"memory leak", "heap growth", "gc pause", "goroutine leak",
}

func matchesStartupAllocation(lines []string, cmdText string) bool {
	if lineMatchesAny(cmdText, startupAllocationPatterns) {
		return true
	}
	return anyLineMatches(lines, startupAllocationPatterns)
}

func matchesInProcessAllocation(lines []string) bool {
	return anyLineMatches(lines, inProcessAllocationPatterns)
}

func matchesMemoryLeak(lines []string) bool {
	return anyLineMatches(lines, memoryLeakPatterns)
}

func collectOOMContextLines(b model.EvidenceBundle, events []model.EvidenceEvent) []string {
	oomPods := oomPodNames(b)
	var lines []string
	add := func(line string) {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}

	for _, lr := range append(b.Logs, b.PreviousLogs...) {
		if len(oomPods) > 0 && !containsString(oomPods, lr.PodName) {
			continue
		}
		for _, ln := range lr.Lines {
			add(ln)
		}
	}
	for _, e := range events {
		if e.SourceType != model.SourceLog {
			continue
		}
		if len(oomPods) > 0 && e.Pod != "" && !containsString(oomPods, e.Pod) {
			continue
		}
		add(firstNonEmptyStr(e.Raw, e.Message))
	}
	return lines
}

func containerCommandText(b model.EvidenceBundle) string {
	var parts []string
	for _, p := range b.Pods {
		for _, c := range p.Containers {
			parts = append(parts, c.Command...)
			parts = append(parts, c.Args...)
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func oomPodNames(b model.EvidenceBundle) []string {
	var names []string
	for _, p := range b.Pods {
		if podWasOOMKilled(p) {
			names = append(names, p.Name)
		}
	}
	return names
}

func bundleHasOOMEvent(b model.EvidenceBundle) bool {
	for _, e := range b.Events {
		if strings.EqualFold(e.Reason, "OOMKilled") || strings.EqualFold(e.Reason, "OOMKilling") {
			return true
		}
	}
	return false
}

func failingPodName(b model.EvidenceBundle) string {
	pod, _ := failingPod(b)
	return pod
}

func anyLineMatches(lines []string, patterns []string) bool {
	for _, ln := range lines {
		if lineMatchesAny(strings.ToLower(ln), patterns) {
			return true
		}
	}
	return false
}

func lineMatchesAny(text string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(text, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func firstMatchingLine(lines []string, patterns []string) string {
	for _, ln := range lines {
		if lineMatchesAny(strings.ToLower(ln), patterns) {
			return ln
		}
	}
	return ""
}

func truncSnippet(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func appendUniqueString(dst []string, s string) []string {
	for _, d := range dst {
		if d == s {
			return dst
		}
	}
	return append(dst, s)
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ClassifyLogReason maps a log line to a correlation-friendly reason label.
func ClassifyLogReason(line string) string {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "allocating memory"), strings.Contains(l, "dd if="),
		strings.Contains(l, "/tmp/leak"), strings.Contains(l, "/dev/shm"):
		return "Memory allocation"
	case strings.Contains(l, "out of memory allocating"), strings.Contains(l, "cannot allocate"),
		strings.Contains(l, "memory allocation failed"):
		return "Memory allocation failures"
	case strings.Contains(l, "oom"), strings.Contains(l, "out of memory"):
		return "OOMKilled"
	case strings.Contains(l, "memory leak"), strings.Contains(l, "heap growth"):
		return "Memory leak"
	case strings.Contains(l, "redis"), strings.Contains(l, "timeout"):
		return "Redis timeout"
	case strings.Contains(l, "error"), strings.Contains(l, "failed"):
		return "Error"
	case strings.Contains(l, "warn"):
		return "Warning"
	default:
		return ""
	}
}
