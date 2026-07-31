package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// FailuresView — tab 4 Failures: Can I immediately identify the failing runtime?
func FailuresView(st model.InvestigationState, width, cursor, height int) string {
	pods := rankPodsForTriage(st.Snapshot.Pods)
	if len(pods) == 0 {
		return Panel("Failures", width, dimStyle.Render("  no pods in investigation scope"))
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(pods) {
		cursor = len(pods) - 1
	}

	summary := strings.Join(overallFailureLines(st, pods), "\n")
	summaryPanel := Panel("Failures", width, summary)

	summaryLines := len(strings.Split(summary, "\n"))
	bodyH := height - summaryLines - 4 // summary panel chrome
	if bodyH < 12 {
		bodyH = 12
	}

	leftW := clampInt(width/3, 34, 44)
	rightW := width - leftW - 1

	tableBody := podTriageList(pods, cursor, leftW, bodyH)
	detailBody := podInvestigationPanel(st, pods[cursor], cursor)

	left := PanelH("Pod Triage", leftW, bodyH, tableBody)
	right := PanelH("Pod Investigation", rightW, bodyH, detailBody)

	return summaryPanel + "\n" + TwoCol(left, right)
}

func rankPodsForTriage(pods []model.PodSummary) []model.PodSummary {
	if len(pods) == 0 {
		return nil
	}
	out := append([]model.PodSummary(nil), pods...)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := podTriageScore(out[i]), podTriageScore(out[j])
		if si != sj {
			return si > sj
		}
		if out[i].RestartCount != out[j].RestartCount {
			return out[i].RestartCount > out[j].RestartCount
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func podTriageScore(p model.PodSummary) int {
	score := healthRank(podHealthLabel(p)) * 100
	score += int(p.RestartCount) * 5
	if !p.Ready {
		score += 20
	}
	c := worstContainer(p)
	if c.LastReason == "OOMKilled" {
		score += 30
	}
	return score
}

func worstContainer(p model.PodSummary) model.ContainerStatus {
	if len(p.Containers) == 0 {
		return model.ContainerStatus{}
	}
	best := p.Containers[0]
	bestRank := healthRank(containerHealthLabel(best))
	for _, c := range p.Containers[1:] {
		r := healthRank(containerHealthLabel(c))
		if r > bestRank || (r == bestRank && c.RestartCount > best.RestartCount) {
			best, bestRank = c, r
		}
	}
	return best
}

func overallFailureLines(st model.InvestigationState, pods []model.PodSummary) []string {
	signal := firstNonEmpty(st.Verdict.LeadingSignal, dominantFailureReason(pods))
	desc := failureSignalDescription(signal)
	observed := formatObservedTime(latestPodFailureTime(st, pods))

	failing, total := 0, len(pods)
	for _, p := range pods {
		if podHealthLabel(p) != "healthy" {
			failing++
		}
	}
	scope := markStyle.Render(fmt.Sprintf("%d of %d pods failing", failing, total))
	if failing == 0 {
		scope = okStyle.Render("all pods stable")
	}

	head := fmt.Sprintf("  %s   %s",
		critStyle.Render(signalDisplay(signal)),
		markStyle.Render(desc))
	if observed != "" {
		head += "   " + markStyle.Render("observed "+observed)
	}
	return []string{head, "  " + scope}
}

func failureSignalDescription(signal string) string {
	switch strings.ToLower(signal) {
	case "oomkilled":
		return "container exceeded memory limit"
	case "crashloopbackoff":
		return "container crash loop detected"
	case "imagepullbackoff", "errimagepull":
		return "image pull failing"
	case "failedmount":
		return "volume mount failed"
	default:
		if signal == "" || signal == "none" {
			return "no active failure signal"
		}
		return strings.ToLower(signal)
	}
}

func signalDisplay(signal string) string {
	if signal == "" {
		return "No leading failure"
	}
	return signal
}

func dominantFailureReason(pods []model.PodSummary) string {
	counts := map[string]int{}
	for _, p := range pods {
		c := worstContainer(p)
		r := firstNonEmpty(c.LastReason, c.Reason)
		if r != "" {
			counts[r]++
		}
	}
	best, n := "", 0
	for r, c := range counts {
		if c > n {
			best, n = r, c
		}
	}
	return best
}

func latestPodFailureTime(st model.InvestigationState, pods []model.PodSummary) time.Time {
	var latest time.Time
	names := podNameSet(pods)
	for _, e := range st.Timeline {
		if !timelineMatchesPod(e, names) {
			continue
		}
		if e.Severity >= model.SeverityHigh && e.Timestamp.After(latest) {
			latest = e.Timestamp
		}
	}
	for _, e := range st.LiveEvidence {
		if names[e.Pod] && e.Severity >= model.SeverityHigh && e.Timestamp.After(latest) {
			latest = e.Timestamp
		}
	}
	for _, p := range pods {
		c := worstContainer(p)
		if c.FinishedAt != nil && c.FinishedAt.After(latest) {
			latest = *c.FinishedAt
		}
	}
	return latest
}

func formatObservedTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("15:04:05")
}

func podTriageList(pods []model.PodSummary, cursor, width, height int) string {
	nameW := clampInt(width-8, 14, 24)
	var lines []string

	start := 0
	if cursor >= height-2 && len(pods) > height-2 {
		start = cursor - (height - 3)
	}
	end := start + height - 1
	if end > len(pods) {
		end = len(pods)
	}

	for i := start; i < end; i++ {
		p := pods[i]
		health := podHealthLabel(p)
		c := worstContainer(p)
		lastFail := firstNonEmpty(c.LastReason, c.Reason, "—")
		exitCode := ""
		if c.LastExitCode > 0 {
			exitCode = critStyle.Render(fmt.Sprintf("exit %d", c.LastExitCode))
		}
		ready := podReadinessLabel(p, health)

		mark := healthMark(health)
		name := padRight(truncVisual(p.Name, nameW), nameW)
		line1 := fmt.Sprintf("%s %s", mark, name)
		line2 := fmt.Sprintf("  %s · %d restarts", ready, p.RestartCount)
		reasonStyled := lastFail
		if health != "healthy" && lastFail != "—" {
			reasonStyled = sevStyle(podFailureSeverity(c)).Render(lastFail)
		}
		line3 := fmt.Sprintf("  %s", truncVisual(reasonStyled, nameW+2))
		if exitCode != "" {
			line3 += "  " + exitCode
		}

		if health == "healthy" {
			line1 = dimStyle.Render(line1)
			line2 = dimStyle.Render(line2)
			line3 = dimStyle.Render(line3)
		}
		if i == cursor {
			lines = append(lines, headStyle.Render("▸ "+line1), line2, line3, "")
		} else {
			lines = append(lines, " "+line1, line2, line3, "")
		}
	}
	if len(lines) == 0 {
		lines = append(lines, dimStyle.Render("  no pods"))
	}
	return strings.Join(lines, "\n")
}

func podReadinessLabel(p model.PodSummary, health string) string {
	switch {
	case health == "healthy":
		return "Stable"
	case !p.Ready:
		return "Unready"
	default:
		return "Degraded"
	}
}

func podInvestigationPanel(st model.InvestigationState, pod model.PodSummary, rank int) string {
	c := worstContainer(pod)
	var lines []string

	lines = append(lines, whyThisPodLines(st, pod, rank)...)
	lines = append(lines, "")

	lines = append(lines, markStyle.Render(fmt.Sprintf("  %s · %s · %s",
		pod.Name, firstNonEmpty(c.Name, "—"), shortImage(c.Image))))
	statusLine := podStatusPhrase(pod, c) + " · " + podReadinessLabel(pod, podHealthLabel(pod)) +
		fmt.Sprintf(" · %d restarts", pod.RestartCount)
	lines = append(lines, "  "+statusLine)
	if c.LastReason != "" {
		reason := sevStyle(podFailureSeverity(c)).Render(c.LastReason)
		exit := ""
		if c.LastExitCode > 0 {
			exit = "  " + critStyle.Render(fmt.Sprintf("exit %d", c.LastExitCode))
		}
		lines = append(lines, "  "+markStyle.Render("failure")+" · "+reason+exit+" · "+tsLabel(c.FinishedAt))
	}
	lines = append(lines, "")

	evidence := podUniqueEvidence(st, pod)
	if len(evidence) > 0 {
		evLines := make([]string, 0, len(evidence))
		for _, e := range evidence {
			evLines = append(evLines, "  "+markStyle.Render("•")+" "+e)
		}
		lines = append(lines, triageSection("Evidence", evLines)...)
	}

	timeline := podFailureTimeline(st, pod)
	if len(timeline) > 0 {
		tlLines := []string{}
		for i, step := range timeline {
			if i > 0 {
				tlLines = append(tlLines, markStyle.Render("    ↓"))
			}
			tlLines = append(tlLines, "  "+step)
		}
		lines = append(lines, triageSection("Failure Timeline", tlLines)...)
	}

	if len(lines) == 0 {
		return dimStyle.Render("  select a pod")
	}
	return strings.Join(lines, "\n")
}

func triageSection(title string, lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	rule := strings.Repeat("─", clampInt(len(title)+4, 12, 20))
	out := []string{headStyle.Render(title), markStyle.Render("  "+rule)}
	return append(out, lines...)
}

func podStatusPhrase(p model.PodSummary, c model.ContainerStatus) string {
	health := podHealthLabel(p)
	reason := firstNonEmpty(c.LastReason, c.Reason)
	switch health {
	case "critical":
		if reason != "" {
			return critStyle.Render("Failing · " + reason)
		}
		return critStyle.Render("Failing")
	case "warning", "degraded", "unknown":
		if reason != "" {
			return highStyle.Render("Degraded · " + reason)
		}
		return highStyle.Render("Degraded")
	default:
		return okStyle.Render("Stable")
	}
}

func whyThisPodLines(st model.InvestigationState, pod model.PodSummary, rank int) []string {
	var reasons []string
	leading := st.Verdict.LeadingSignal
	c := worstContainer(pod)
	podReason := firstNonEmpty(c.LastReason, c.Reason)

	if leading != "" && (podReason == leading || strings.Contains(strings.ToLower(podReason), strings.ToLower(leading))) {
		reasons = append(reasons, okStyle.Render("✓")+" Matches current leading signal ("+leading+")")
	}
	if rank == 0 {
		reasons = append(reasons, okStyle.Render("✓")+" Highest priority pod in workload")
	}
	if maxRestarts(st.Snapshot.Pods) == pod.RestartCount && pod.RestartCount > 0 {
		reasons = append(reasons, okStyle.Render("✓")+fmt.Sprintf(" Highest restart count (%d)", pod.RestartCount))
	}
	if isLatestCriticalPod(st, pod.Name) {
		reasons = append(reasons, okStyle.Render("✓")+" Latest critical event in scope")
	}
	for _, ap := range st.Verdict.AffectedPods {
		if strings.HasSuffix(ap, "/"+pod.Name) || ap == pod.Name {
			reasons = append(reasons, okStyle.Render("✓")+" Listed in affected pods")
			break
		}
	}
	if st.Verdict.Confidence > 0 && rank == 0 {
		reasons = append(reasons, okStyle.Render("✓")+fmt.Sprintf(" Confidence %.0f%% on this failure pattern", st.Verdict.Confidence*100))
	}

	if len(reasons) == 0 {
		reasons = append(reasons, markStyle.Render("·")+" Stable pod — no triage signals")
	}

	out := []string{headStyle.Render("Why this pod?")}
	for _, r := range reasons {
		out = append(out, "  "+r)
	}
	return out
}

func maxRestarts(pods []model.PodSummary) int32 {
	var m int32
	for _, p := range pods {
		if p.RestartCount > m {
			m = p.RestartCount
		}
	}
	return m
}

func isLatestCriticalPod(st model.InvestigationState, pod string) bool {
	latest := latestPodFailureTime(st, st.Snapshot.Pods)
	if latest.IsZero() {
		return false
	}
	names := map[string]bool{pod: true}
	for _, e := range st.Timeline {
		if timelineMatchesPod(e, names) && e.Severity >= model.SeverityHigh && e.Timestamp.Equal(latest) {
			return true
		}
	}
	for _, e := range st.LiveEvidence {
		if e.Pod == pod && e.Severity >= model.SeverityHigh && e.Timestamp.Equal(latest) {
			return true
		}
	}
	return false
}

func podUniqueEvidence(st model.InvestigationState, pod model.PodSummary) []string {
	type item struct {
		ts    time.Time
		label string
		score int
	}
	var items []item
	seen := map[string]bool{}
	add := func(label string, ts time.Time, score int) {
		label = strings.TrimSpace(label)
		if label == "" || seen[strings.ToLower(label)] {
			return
		}
		seen[strings.ToLower(label)] = true
		items = append(items, item{ts: ts, label: label, score: score})
	}

	c := worstContainer(pod)
	if c.LastReason != "" {
		add(c.LastReason, timeFromPtr(c.FinishedAt), 100)
	}
	if c.Reason != "" && c.Reason != c.LastReason {
		add(c.Reason, timeFromPtr(c.FinishedAt), 90)
	}

	for _, e := range st.LiveEvidence {
		if e.Pod != pod.Name && !strings.Contains(e.SourceName, pod.Name) {
			continue
		}
		score := evidenceSeverityScore(e.Severity)
		add(e.Reason, e.Timestamp, score)
		if e.Severity >= model.SeverityWarning {
			add(shortEvidenceMessage(e.Message), e.Timestamp, score)
		}
	}
	for _, e := range st.Timeline {
		if !timelineMatchesPod(e, map[string]bool{pod.Name: true}) {
			continue
		}
		score := evidenceSeverityScore(e.Severity)
		add(e.Reason, e.Timestamp, score)
		if e.Severity >= model.SeverityWarning {
			add(shortEvidenceMessage(e.Message), e.Timestamp, score)
		}
	}
	for _, e := range st.Snapshot.Events {
		if e.InvolvedObject.Name != pod.Name {
			continue
		}
		add(e.Reason, e.Timestamp, 50)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].ts.After(items[j].ts)
	})

	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.label)
	}
	return out
}

func podFailureSeverity(c model.ContainerStatus) model.Severity {
	if c.LastReason == "OOMKilled" || strings.Contains(strings.ToLower(c.LastReason), "crash") {
		return model.SeverityCritical
	}
	if c.LastExitCode > 0 && c.LastExitCode != 137 {
		return model.SeverityHigh
	}
	return model.SeverityWarning
}

func evidenceSeverityScore(sev model.Severity) int {
	switch sev {
	case model.SeverityCritical:
		return 100
	case model.SeverityHigh:
		return 70
	case model.SeverityWarning:
		return 40
	default:
		return 10
	}
}

func timeFromPtr(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func shortEvidenceMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "memory"):
		return "Memory allocation failures"
	case strings.Contains(lower, "panic"):
		return "Runtime panic"
	case strings.Contains(lower, "readiness"):
		return "Readiness probe failed"
	case strings.Contains(lower, "timeout"):
		return "Connection timeout"
	}
	if len(msg) > 40 {
		return truncVisual(msg, 40)
	}
	return msg
}

func podFailureTimeline(st model.InvestigationState, pod model.PodSummary) []string {
	type step struct {
		ts    time.Time
		label string
	}
	var steps []step
	names := map[string]bool{pod.Name: true}

	if !pod.CreatedAt.IsZero() {
		steps = append(steps, step{pod.CreatedAt, "Started"})
	}
	for _, e := range st.Timeline {
		if !timelineMatchesPod(e, names) {
			continue
		}
		if label := timelineStepLabel(e); label != "" {
			steps = append(steps, step{e.Timestamp, label})
		}
	}
	for _, e := range st.Snapshot.Events {
		if e.InvolvedObject.Name != pod.Name {
			continue
		}
		if label := eventStepLabel(e.Reason, e.Message); label != "" {
			steps = append(steps, step{e.Timestamp, label})
		}
	}
	for _, e := range st.LiveEvidence {
		if e.Pod != pod.Name {
			continue
		}
		if label := eventStepLabel(e.Reason, e.Message); label != "" {
			steps = append(steps, step{e.Timestamp, label})
		}
	}

	sort.Slice(steps, func(i, j int) bool {
		if steps[i].ts.Equal(steps[j].ts) {
			return steps[i].label < steps[j].label
		}
		return steps[i].ts.Before(steps[j].ts)
	})

	var chain []string
	prev := ""
	for _, s := range steps {
		if s.label == prev {
			continue
		}
		prev = s.label
		chain = append(chain, s.label)
	}
	if len(chain) == 0 {
		c := worstContainer(pod)
		if c.LastReason != "" {
			chain = append(chain, "Started", c.LastReason, "Current")
		}
	}
	if len(chain) > 0 && chain[len(chain)-1] != "Current" {
		chain = append(chain, "Current")
	}
	return chain
}

func timelineStepLabel(e model.TimelineEvent) string {
	return eventStepLabel(e.Reason, e.Message)
}

func eventStepLabel(reason, message string) string {
	combined := strings.ToLower(reason + " " + message)
	switch {
	case strings.Contains(combined, "oom"):
		return "OOMKilled"
	case strings.Contains(combined, "readiness"):
		return "Readiness failed"
	case strings.Contains(combined, "memory"):
		return "Memory allocation"
	case strings.Contains(combined, "restart"), strings.Contains(combined, "backoff"):
		return "Restarted"
	case strings.Contains(combined, "started"), strings.Contains(combined, "scheduled"):
		return "Started"
	case reason != "":
		return reason
	}
	return ""
}

func timelineMatchesPod(e model.TimelineEvent, names map[string]bool) bool {
	if names[e.SourceName] {
		return true
	}
	if names[e.InvolvedObject.Name] {
		return true
	}
	for name := range names {
		if strings.Contains(e.SourceName, name) || strings.Contains(e.Message, name) {
			return true
		}
	}
	return false
}

func podNameSet(pods []model.PodSummary) map[string]bool {
	m := make(map[string]bool, len(pods))
	for _, p := range pods {
		m[p.Name] = true
	}
	return m
}

func containerHealthLabel(c model.ContainerStatus) string {
	if c.LastReason == "OOMKilled" || strings.Contains(strings.ToLower(c.LastReason), "crash") {
		return "critical"
	}
	if !c.Ready {
		return "warning"
	}
	return "healthy"
}

func tsLabel(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "—"
	}
	return t.Format("15:04:05")
}
