package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// EvidenceView — tab 6 Evidence: Does it convincingly justify Klew's verdict?
func EvidenceView(st model.InvestigationState, _ string, scroll, height, width int) string {
	rows := height - 2
	if rows < 16 {
		rows = 16
	}

	summaryH := clampInt(rows*20/100, 5, 7)
	midH := clampInt(rows*34/100, 8, 12)
	corrH := clampInt(rows*14/100, 4, 6)
	bottomH := rows - summaryH - midH - corrH
	if bottomH < 5 {
		bottomH = 5
	}

	summaryTitle := "Summary"
	if st.Mode == model.ModeLive && !st.Paused {
		summaryTitle = "Summary · Live"
	}
	summary := PanelH(summaryTitle, width, summaryH, finalSummaryBody(st, width))
	midRow := TwoCol(
		PanelH("Evidence", (width-1)/2, midH, evidenceGroupsBody(st, (width-1)/2, midH)),
		PanelH("Claims", width-(width-1)/2-1, midH, claimsBody(st, width-(width-1)/2-1, midH)),
	)
	corr := PanelH("Cross Correlation", width, corrH, crossCorrelationBody(st, width))
	bottomRow := TwoCol(
		PanelH("Confidence", (width-1)/2, bottomH, confidenceTimelineBody(st, (width-1)/2, bottomH)),
		PanelH("Gaps & Next", width-(width-1)/2-1, bottomH, gapsAndVerificationBody(st, width-(width-1)/2-1, bottomH)),
	)

	body := summary + "\n" + midRow + "\n" + corr + "\n" + bottomRow
	if scroll > 0 {
		lines := strings.Split(body, "\n")
		if scroll < len(lines) {
			body = strings.Join(lines[scroll:], "\n")
		}
	}
	return body
}

// ── Investigation Summary ────────────────────────────────────────────────────

func finalSummaryBody(st model.InvestigationState, width int) string {
	v := st.Verdict
	obs := evidenceObservationCount(st)
	window := formatInvestigationWindow(st)

	verdict := firstNonEmpty(st.HypothesisLabel, v.LikelyTrigger, "Collecting evidence…")
	signal := firstNonEmpty(v.LeadingSignal, "None")
	if v.Status == model.VerdictHealthy {
		verdict = "No active incident"
		signal = "None"
	}

	conf := dimStyle.Render("—")
	if v.Confidence > 0 {
		conf = confidencePhrase(v.Confidence, st.ConfidenceTrend)
	}

	state := investigationStateLabel(st)
	_ = state

	cw := width - 4
	if cw < 20 {
		cw = 20
	}
	var out []string
	out = append(out, kv("Hypothesis", truncVisual(verdict, cw-13)))
	out = append(out, kv("Confidence", conf))
	out = append(out, kv("Leading Signal", signalStyled(signal)))
	out = append(out, metaKV("Evidence", fmt.Sprintf("%d observations", obs)))
	out = append(out, metaKV("Time Window", window))
	if st.Mode == model.ModeLive && !st.Paused && !st.LastUpdatedAt.IsZero() {
		out = append(out, metaKV("Last Updated", st.LastUpdatedAt.Time().Format("15:04:05")))
	}
	return strings.Join(out, "\n")
}

func investigationStateLabel(st model.InvestigationState) string {
	if st.Paused {
		return statusPhrase("Paused")
	}
	if st.Mode == model.ModeBundle {
		return dimStyle.Render("Review")
	}
	phase := InvestigationPhase(st)
	live := ""
	if st.Mode == model.ModeLive && !st.Paused {
		live = dimStyle.Render(" · live")
	}
	return statusPhrase(phase) + live
}

func evidenceObservationCount(st model.InvestigationState) int {
	seen := map[string]bool{}
	add := func(key string) {
		if key != "" {
			seen[strings.ToLower(key)] = true
		}
	}
	for _, e := range st.LiveEvidence {
		if e.SourceType == model.SourceSystem || e.SourceType == model.SourceLog {
			continue
		}
		add(dedupeKey(e))
	}
	for _, e := range st.Snapshot.Events {
		add(e.Reason + "|" + e.Message)
	}
	for _, s := range rankedSignals(st.Verdict) {
		add(s.Label)
	}
	if n := len(seen); n > 0 {
		return n
	}
	return int(st.Counters.EventsIngested + st.Counters.ObjectChanges + st.Counters.MetricSamples)
}

func formatInvestigationWindow(st model.InvestigationState) string {
	d := model.DurationFromMS(st.Window)
	if d <= 0 {
		d = 15 * time.Minute
	}
	if m := int(d.Minutes()); m >= 1 {
		return fmt.Sprintf("%d minutes", m)
	}
	return fmt.Sprintf("%d seconds", int(d.Seconds()))
}

func signalStyled(signal string) string {
	switch strings.ToLower(signal) {
	case "oomkilled", "oomkilling":
		return critStyle.Render(signal)
	case "backoff", "crashloopbackoff", "failedmount", "errimagepull":
		return warnStyle.Render(signal)
	case "none", "":
		return dimStyle.Render("None")
	default:
		return signal
	}
}

// ── Evidence Groups ──────────────────────────────────────────────────────────

type groupedItem struct {
	label    string
	count    int
	critical bool
	partial  bool
}

func evidenceGroupsBody(st model.InvestigationState, half, rows int) string {
	groups := collectEvidenceGroups(st)
	order := []string{"Events", "Metrics", "Objects", "Reasoning"}
	var lines []string
	maxPerGroup := clampInt((rows-2)/len(order), 1, 4)

	for _, name := range order {
		items := groups[name]
		if len(items) == 0 {
			continue
		}
		lines = append(lines, headStyle.Render(name))
		shown := 0
		for _, it := range items {
			if shown >= maxPerGroup {
				break
			}
			lines = append(lines, "  "+evidenceMark(it)+truncVisual(it.label, half-8)+countSuffix(it.count))
			shown++
		}
	}
	if len(lines) == 0 {
		return dimStyle.Render("  no grouped evidence yet")
	}
	return joinTruncatedLines(lines, rows)
}

func appendBodyLines(out []string, body string) []string {
	for _, ln := range strings.Split(body, "\n") {
		out = append(out, ln)
	}
	return out
}

func joinTruncatedLines(lines []string, max int) string {
	return strings.Join(truncateLines(lines, max), "\n")
}

func evidenceMark(it groupedItem) string {
	switch {
	case it.critical:
		return critStyle.Render("✓ ")
	case it.partial:
		return warnStyle.Render("◐ ")
	default:
		return okStyle.Render("✓ ")
	}
}

func countSuffix(n int) string {
	if n > 1 {
		return dimStyle.Render(fmt.Sprintf(" ×%d", n))
	}
	return ""
}

func collectEvidenceGroups(st model.InvestigationState) map[string][]groupedItem {
	type acc struct {
		count    int
		critical bool
		partial  bool
	}
	buckets := map[string]map[string]*acc{
		"Events": {}, "Metrics": {}, "Objects": {}, "Reasoning": {},
	}

	add := func(group, label string, count int, sev model.Severity) {
		if label == "" {
			return
		}
		key := strings.ToLower(label)
		if buckets[group][key] == nil {
			buckets[group][key] = &acc{}
		}
		a := buckets[group][key]
		a.count += count
		if sev == model.SeverityCritical {
			a.critical = true
		} else if sev == model.SeverityHigh || sev == model.SeverityWarning {
			a.partial = true
		}
	}

	for _, e := range st.LiveEvidence {
		if e.SourceType == model.SourceSystem || e.SourceType == model.SourceLog {
			continue
		}
		c := e.Count
		if c < 1 {
			c = 1
		}
		switch e.SourceType {
		case model.SourceK8sEvent:
			add("Events", eventGroupLabel(e), c, e.Severity)
		case model.SourceMetric:
			add("Metrics", metricGroupLabel(e.Message), c, e.Severity)
		case model.SourceObjectChange:
			add("Objects", objectGroupLabel(e), c, e.Severity)
		}
	}
	for _, e := range st.Snapshot.Events {
		c := int(e.Count)
		if c < 1 {
			c = 1
		}
		add("Events", e.Reason, c, reportEventSeverity(e.Reason))
	}
	for _, e := range st.Timeline {
		switch e.Type {
		case "deploy", "rs":
			add("Objects", timelineObjectLabel(e), 1, e.Severity)
		case "metric":
			add("Metrics", metricGroupLabel(e.Message), 1, e.Severity)
		case "event", "k8s_event":
			add("Events", firstNonEmpty(e.Reason, shortMsg(e.Message)), 1, e.Severity)
		default:
			if e.Reason != "" && e.Type != "log" && e.Type != "verdict" {
				add("Events", firstNonEmpty(e.Reason, shortMsg(e.Message)), 1, e.Severity)
			}
		}
	}
	for _, rs := range st.Snapshot.ReplicaSets {
		if rs.DeploymentOwner != "" {
			add("Objects", fmt.Sprintf("ReplicaSet %s", rs.Name), 1, model.SeverityInfo)
		}
	}
	if rc := st.RecentChange; rc != nil {
		if rc.RevisionTo != "" {
			add("Objects", fmt.Sprintf("Deployment revision %s", rc.RevisionTo), 1, model.SeverityInfo)
		}
	}
	for _, s := range rankedSignals(st.Verdict) {
		grp := "Reasoning"
		switch strings.ToUpper(s.Source) {
		case "EVENT":
			grp = "Events"
		case "METRIC":
			grp = "Metrics"
		case "OBJECT":
			grp = "Objects"
		case "LOG":
			// logs are already distilled into verdict signals — show under Reasoning
			grp = "Reasoning"
		}
		c := s.Count
		if c < 1 {
			c = 1
		}
		add(grp, s.Label, c, s.Severity)
	}
	for _, c := range st.Correlation {
		label := strings.TrimPrefix(c, "✓ ")
		label = strings.TrimSpace(label)
		add("Reasoning", label, 1, model.SeverityInfo)
	}
	for _, r := range st.HypothesisReasons {
		add("Reasoning", r, 1, model.SeverityInfo)
	}
	for _, e := range st.LiveEvidence {
		if e.SourceType == model.SourceSystem && strings.Contains(e.Reason, "Hypothesis") {
			add("Reasoning", "Hypothesis confidence updated", 1, model.SeverityInfo)
		}
	}

	out := map[string][]groupedItem{}
	for group, m := range buckets {
		var items []groupedItem
		for label, a := range m {
			items = append(items, groupedItem{label: label, count: a.count, critical: a.critical, partial: a.partial})
		}
		sort.SliceStable(items, func(i, j int) bool {
			si := groupSortScore(items[i])
			sj := groupSortScore(items[j])
			if si != sj {
				return si > sj
			}
			return items[i].label < items[j].label
		})
		out[group] = items
	}
	return out
}

func groupSortScore(it groupedItem) int {
	score := it.count
	if it.critical {
		score += 100
	} else if it.partial {
		score += 40
	}
	return score
}

func reportEventSeverity(reason string) model.Severity {
	switch reason {
	case "OOMKilled", "OOMKilling", "Failed", "FailedMount", "ErrImagePull", "ImagePullBackOff", "CrashLoopBackOff":
		return model.SeverityCritical
	case "BackOff", "Unhealthy", "NodeNotReady":
		return model.SeverityHigh
	default:
		return model.SeverityWarning
	}
}

func eventGroupLabel(e model.EvidenceEvent) string {
	if e.Reason != "" {
		return e.Reason
	}
	return shortMsg(e.Message)
}

func objectGroupLabel(e model.EvidenceEvent) string {
	kind := firstNonEmpty(e.SourceKind, "Object")
	name := firstNonEmpty(e.SourceName, e.Pod)
	msg := shortMsg(e.Message)
	if msg != "" {
		return fmt.Sprintf("%s %s", kind, msg)
	}
	return fmt.Sprintf("%s %s", kind, name)
}

func timelineObjectLabel(e model.TimelineEvent) string {
	switch e.Type {
	case "deploy":
		return "Deployment updated"
	case "rs":
		return firstNonEmpty(e.Message, "ReplicaSet created")
	default:
		return shortMsg(e.Message)
	}
}

func metricGroupLabel(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "memory"):
		if strings.Contains(lower, "spike") || strings.Contains(lower, "2.") || strings.Contains(lower, "high") {
			return "Memory spike"
		}
		return "Memory elevated"
	case strings.Contains(lower, "cpu"):
		return "CPU stable"
	default:
		return shortMsg(msg)
	}
}

func dedupeKey(e model.EvidenceEvent) string {
	return string(e.SourceType) + "|" + firstNonEmpty(e.Reason, e.Message, e.Raw)
}

func shortMsg(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return truncVisual(s, 48)
}

// ── Claims ───────────────────────────────────────────────────────────────────

type finalClaim struct {
	text       string
	confidence float64
	evidence   []string
}

func claimsBody(st model.InvestigationState, half, rows int) string {
	claims := buildFinalClaims(st)
	if len(claims) == 0 {
		return dimStyle.Render("  no claims — insufficient evidence")
	}

	var lines []string
	for i, c := range claims {
		if i > 0 {
			lines = append(lines, dimStyle.Render("  "+strings.Repeat("─", clampInt(half-6, 8, 24))))
		}
		lines = append(lines, headStyle.Render("Claim"))
		lines = append(lines, "  "+truncVisual(c.text, half-4))
		if c.confidence > 0 {
			lines = append(lines, "  "+kv("Confidence", confidencePhrase(c.confidence, "")))
		}
		lines = append(lines, "  "+headStyle.Render("Supporting Evidence"))
		for _, ev := range c.evidence {
			lines = append(lines, "  "+okStyle.Render("✓")+" "+truncVisual(ev, half-6))
		}
	}
	return joinTruncatedLines(lines, rows)
}

func buildFinalClaims(st model.InvestigationState) []finalClaim {
	var claims []finalClaim

	primary := buildPrimaryClaim(st)
	if primary.text != "" {
		claims = append(claims, primary)
	}
	if rollout := buildRolloutClaim(st); rollout.text != "" {
		claims = append(claims, rollout)
	}
	return claims
}

func buildPrimaryClaim(st model.InvestigationState) finalClaim {
	label := firstNonEmpty(st.HypothesisLabel, st.Verdict.LikelyTrigger)
	if label == "" || st.Verdict.Status == model.VerdictHealthy {
		return finalClaim{}
	}

	var evidence []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		for _, e := range evidence {
			if e == s {
				return
			}
		}
		evidence = append(evidence, s)
	}

	signal := st.Verdict.LeadingSignal
	if signal != "" {
		add(signal + " (Event)")
	}
	for _, s := range rankedSignals(st.Verdict) {
		if s.Strength == "strong" || s.Severity == model.SeverityCritical {
			src := s.Source
			if src == "" {
				src = "Evidence"
			}
			if strings.EqualFold(src, "LOG") {
				src = "Reasoning"
			}
			add(fmt.Sprintf("%s (%s)", s.Label, src))
		}
	}
	for _, r := range st.HypothesisReasons {
		add(r)
	}
	for _, p := range st.Snapshot.Pods {
		for _, c := range p.Containers {
			if c.LastExitCode == 137 {
				add("Exit Code 137")
			}
			if strings.Contains(strings.ToLower(c.LastReason), "oom") {
				add("OOMKilled (Container)")
			}
		}
	}
	if st.Snapshot.Metrics.Available && st.Snapshot.Metrics.MemLimitMi > 0 &&
		st.Snapshot.Metrics.MemUsageMi > st.Snapshot.Metrics.MemRequestMi {
		add("Memory usage spike (Metric)")
	}

	conf := st.Verdict.Confidence
	if conf <= 0 {
		conf = 0.5
	}
	return finalClaim{text: label, confidence: conf, evidence: evidence}
}

func buildRolloutClaim(st model.InvestigationState) finalClaim {
	rc := st.RecentChange
	hasDeploy := rc != nil && (rc.RevisionTo != "" || rc.RevisionFrom != "")
	for _, e := range st.Timeline {
		if e.Type == "deploy" || e.Type == "rs" {
			hasDeploy = true
			break
		}
	}
	if !hasDeploy {
		return finalClaim{}
	}

	var evidence []string
	add := func(s string) {
		for _, e := range evidence {
			if e == s {
				return
			}
		}
		evidence = append(evidence, s)
	}

	if rc != nil && rc.RevisionFrom != "" && rc.RevisionTo != "" {
		add(fmt.Sprintf("Deployment revision %s → %s", rc.RevisionFrom, rc.RevisionTo))
	} else if rc != nil && rc.RevisionTo != "" {
		add(fmt.Sprintf("Deployment revision %s", rc.RevisionTo))
	}
	for _, e := range st.Timeline {
		if e.Type == "rs" {
			add("ReplicaSet created")
			break
		}
	}
	if chain := st.CausalChain; len(chain) >= 2 {
		add("Failure began after rollout")
	} else {
		for _, c := range st.Correlation {
			if strings.Contains(strings.ToLower(c), "rollout") || strings.Contains(strings.ToLower(c), "preceded") {
				add(strings.TrimPrefix(c, "✓ "))
				break
			}
		}
	}
	restarts := 0
	for _, p := range st.Snapshot.Pods {
		if p.RestartCount > 0 {
			restarts++
		}
	}
	if restarts >= 2 {
		add("Restart frequency increased")
	}
	if len(evidence) == 0 {
		return finalClaim{}
	}

	conf := st.Verdict.Confidence * 0.92
	if conf <= 0 {
		conf = 0.55
	}
	if conf > 0.95 {
		conf = 0.95
	}
	return finalClaim{
		text:       "Recent rollout likely introduced regression",
		confidence: conf,
		evidence:   evidence,
	}
}

// ── Cross Correlation ────────────────────────────────────────────────────────

type correlationRow struct {
	signal string
	event  bool
	metric bool
	object bool
}

func crossCorrelationBody(st model.InvestigationState, width int) string {
	rows := buildCorrelationRows(st)
	if len(rows) == 0 {
		return dimStyle.Render("  no cross-source correlation yet")
	}

	cw := width - 4
	var lines []string
	header := fmt.Sprintf("  %s %s %s %s",
		padRight("Signal", clampInt(cw/3, 14, 22)),
		padRight("EVENT", 7), padRight("METRIC", 8), "OBJECT")
	lines = append(lines, dimStyle.Render(header))

	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("  %s %s %s %s",
			padRight(truncVisual(r.signal, clampInt(cw/3, 14, 22)), clampInt(cw/3, 14, 22)),
			corrMark(r.event), corrMark(r.metric), corrMark(r.object)))
	}
	return strings.Join(lines, "\n")
}

func corrMark(ok bool) string {
	if ok {
		return padRight(okStyle.Render("✓"), 6)
	}
	return padRight(dimStyle.Render("—"), 6)
}

func buildCorrelationRows(st model.InvestigationState) []correlationRow {
	leading := firstNonEmpty(st.Verdict.LeadingSignal, "Primary signal")
	rows := []correlationRow{{signal: leading}}
	rows[0] = correlateSignal(st, leading)

	if restarts := totalRestarts(st.Snapshot.Pods); restarts > 0 {
		rows = append(rows, correlateSignal(st, "Restart Count"))
		rows[len(rows)-1].signal = fmt.Sprintf("Restart Count (%d)", restarts)
	}
	return rows
}

func correlateSignal(st model.InvestigationState, signal string) correlationRow {
	row := correlationRow{signal: signal}
	lower := strings.ToLower(signal)

	match := func(s string) bool {
		s = strings.ToLower(s)
		return strings.Contains(s, lower) || strings.Contains(lower, s) ||
			(strings.Contains(lower, "oom") && strings.Contains(s, "oom")) ||
			(strings.Contains(lower, "restart") && (strings.Contains(s, "restart") || strings.Contains(s, "backoff"))) ||
			(strings.Contains(lower, "memory") && strings.Contains(s, "memory"))
	}

	for _, e := range st.LiveEvidence {
		if e.SourceType == model.SourceLog || e.SourceType == model.SourceSystem {
			continue
		}
		hay := strings.ToLower(firstNonEmpty(e.Reason, e.Message, plainMsg(e)))
		if !match(hay) && !signalMatchesEvent(signal, e) {
			continue
		}
		switch e.SourceType {
		case model.SourceK8sEvent:
			row.event = true
		case model.SourceMetric:
			row.metric = true
		case model.SourceObjectChange:
			row.object = true
		}
	}
	for _, e := range st.Snapshot.Events {
		if match(strings.ToLower(e.Reason + " " + e.Message)) {
			row.event = true
		}
	}
	for _, p := range st.Snapshot.Pods {
		if strings.Contains(lower, "restart") && p.RestartCount > 0 {
			row.object = true
		}
		if strings.Contains(lower, "oom") {
			for _, c := range p.Containers {
				if strings.Contains(strings.ToLower(c.LastReason), "oom") {
					row.object = true
					row.event = true
				}
			}
		}
	}
	if strings.Contains(lower, "memory") || strings.Contains(lower, "oom") {
		if st.Snapshot.Metrics.Available && st.Snapshot.Metrics.MemUsageMi > st.Snapshot.Metrics.MemRequestMi {
			row.metric = true
		}
	}
	return row
}

func signalMatchesEvent(signal string, e model.EvidenceEvent) bool {
	if e.Reason != "" && strings.EqualFold(e.Reason, signal) {
		return true
	}
	lower := strings.ToLower(signal)
	if strings.Contains(lower, "oom") && strings.Contains(strings.ToLower(e.Reason+" "+e.Message), "oom") {
		return true
	}
	return false
}

func totalRestarts(pods []model.PodSummary) int {
	n := 0
	for _, p := range pods {
		n += int(p.RestartCount)
	}
	return n
}

// ── Confidence Timeline ──────────────────────────────────────────────────────

type confStep struct {
	conf  float64
	label string
}

func confidenceTimelineBody(st model.InvestigationState, half, rows int) string {
	steps := buildConfidenceTimeline(st)
	if len(steps) == 0 {
		return dimStyle.Render("  confidence progression not available")
	}

	var lines []string
	lines = append(lines, dimStyle.Render("Confidence progression"))
	for i, s := range steps {
		if i > 0 {
			lines = append(lines, dimStyle.Render("  ↓"))
		}
		pct := fmt.Sprintf("%.0f%%", s.conf*100)
		col := okStyle
		if s.conf >= 0.85 {
			col = critStyle
		} else if s.conf >= 0.6 {
			col = warnStyle
		}
		lines = append(lines, "  "+col.Render(pct))
		if s.label != "" {
			lines = append(lines, "  "+dimStyle.Render(truncVisual(s.label, half-4)))
		}
	}
	return joinTruncatedLines(lines, rows)
}

func buildConfidenceTimeline(st model.InvestigationState) []confStep {
	var steps []confStep
	seen := map[string]bool{}

	add := func(label string, conf float64) {
		if label == "" || conf <= 0 {
			return
		}
		key := strings.ToLower(label)
		if seen[key] {
			return
		}
		seen[key] = true
		steps = append(steps, confStep{conf: conf, label: label})
	}

	// milestone labels from timeline — ordered by time
	type milestone struct {
		t     model.Timestamp
		label string
		conf  float64
	}
	var ms []milestone
	for _, e := range st.Timeline {
		if e.Type == "log" {
			continue
		}
		lbl := confidenceMilestoneLabel(e)
		if lbl == "" {
			continue
		}
		conf := e.Confidence
		if conf <= 0 {
			conf = confidenceForSeverity(e.Severity, e.Reason)
		}
		ms = append(ms, milestone{e.Timestamp, lbl, conf})
	}
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].t.Before(ms[j].t) })

	if len(ms) == 0 {
		// fallback from hypothesis transition + verdict
		if st.LastTransition != nil && st.LastTransition.From != "" {
			add("Earlier: "+st.LastTransition.From, 0.45)
		}
		for _, r := range st.HypothesisReasons {
			add(r, 0.65)
		}
	} else {
		for _, m := range ms {
			add(m.label, m.conf)
		}
	}

	if st.Verdict.Confidence > 0 {
		final := firstNonEmpty(st.Verdict.LeadingSignal, st.HypothesisLabel, "Conclusion confirmed")
		add(final+" confirmed", st.Verdict.Confidence)
	}

	// enforce monotonic confidence
	for i := 1; i < len(steps); i++ {
		if steps[i].conf < steps[i-1].conf {
			steps[i].conf = steps[i-1].conf + 0.05
			if steps[i].conf > 0.98 {
				steps[i].conf = 0.98
			}
		}
	}
	if len(steps) > 6 {
		steps = steps[len(steps)-6:]
	}
	return steps
}

func confidenceMilestoneLabel(e model.TimelineEvent) string {
	reason := strings.ToLower(firstNonEmpty(e.Reason, e.Message))
	switch {
	case strings.Contains(reason, "readiness"):
		return "Readiness failures observed"
	case strings.Contains(reason, "redis") || strings.Contains(reason, "timeout"):
		return "Redis timeout correlated"
	case strings.Contains(reason, "memory") && (strings.Contains(reason, "alloc") || strings.Contains(reason, "fail")):
		return "Memory allocation failures detected"
	case strings.Contains(reason, "oom"):
		return "OOMKilled confirmed"
	case e.Type == "deploy":
		return "Deployment change observed"
	case e.Type == "verdict" || strings.Contains(reason, "hypothesis"):
		return "Hypothesis updated"
	case e.Severity == model.SeverityCritical:
		return firstNonEmpty(e.Reason, shortMsg(e.Message))
	case e.Severity == model.SeverityHigh:
		return firstNonEmpty(e.Reason, shortMsg(e.Message))
	default:
		return ""
	}
}

func confidenceForSeverity(sev model.Severity, reason string) float64 {
	switch sev {
	case model.SeverityCritical:
		if strings.Contains(strings.ToLower(reason), "oom") {
			return 0.92
		}
		return 0.85
	case model.SeverityHigh:
		return 0.65
	case model.SeverityWarning:
		return 0.45
	default:
		return 0.35
	}
}

// ── Gaps & Verification ──────────────────────────────────────────────────────

func gapsAndVerificationBody(st model.InvestigationState, half, rows int) string {
	gaps := realInvestigationGaps(st)
	checks := investigationNextSteps(st)

	var lines []string
	if len(gaps) > 0 {
		lines = append(lines, headStyle.Render("Still Missing"))
		maxG := clampInt(rows/2, 2, 4)
		for i, g := range gaps {
			if i >= maxG {
				break
			}
			lines = append(lines, "  "+dimStyle.Render("○ ")+truncVisual(g, half-6))
		}
		lines = append(lines, "")
	}
	lines = append(lines, headStyle.Render("Suggested next steps"))
	if len(checks) == 0 {
		lines = append(lines, dimStyle.Render("  conclusion is well supported"))
	} else {
		for _, c := range checks {
			lines = append(lines, "  "+dimStyle.Render("→ ")+truncVisual(c, half-6))
		}
	}
	return joinTruncatedLines(lines, rows)
}

func investigationNextSteps(st model.InvestigationState) []string {
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || isShellCommand(s) {
			return
		}
		for _, o := range out {
			if o == s {
				return
			}
		}
		out = append(out, s)
	}
	for _, c := range st.NextChecks {
		add(c)
	}
	for _, c := range st.Verdict.RecommendedNextChecks {
		add(c)
	}
	if !st.Snapshot.Metrics.Available && countSnapshotOOM(st.Snapshot) > 0 {
		add("Enable live metrics to see usage trend vs limits before each OOM kill")
	}
	return out
}

func isShellCommand(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "kubectl") || strings.HasPrefix(s, "klew-labs:")
}

func countSnapshotOOM(b model.EvidenceBundle) int {
	n := 0
	for _, p := range b.Pods {
		for _, c := range p.Containers {
			if strings.EqualFold(c.LastReason, "OOMKilled") || c.LastExitCode == 137 {
				n++
				break
			}
		}
	}
	return n
}

func realInvestigationGaps(st model.InvestigationState) []string {
	var gaps []string
	for _, w := range st.Verdict.MissingDataWarnings {
		gaps = append(gaps, w)
	}
	for _, w := range st.Warnings {
		if strings.Contains(strings.ToLower(w), "permission") || strings.Contains(strings.ToLower(w), "denied") {
			gaps = append(gaps, w)
		}
	}
	if !st.Snapshot.Metrics.Available {
		gaps = append(gaps, "Live metrics unavailable")
	}
	return gaps
}

func investigationGaps(st model.InvestigationState) []string {
	var gaps []string
	add := func(s string) {
		for _, g := range gaps {
			if g == s {
				return
			}
		}
		gaps = append(gaps, s)
	}

	if !nodePressureDetected(st.Snapshot.Nodes) {
		add("No node pressure observed")
	}
	if !hasEvidenceReason(st, "networkpolicy", "network policy") {
		add("No network policy changes")
	}
	if !hasPVCEvidence(st) {
		add("No PVC failures")
	}
	if !hasEvidenceReason(st, "errimagepull", "imagepullbackoff", "image pull") {
		add("No image pull failures")
	}
	if !hasEvidenceReason(st, "failedscheduling") {
		add("No scheduling failures")
	}
	if !st.Snapshot.Metrics.Available {
		add("Live metrics unavailable")
	}
	for _, w := range st.Verdict.MissingDataWarnings {
		add(w)
	}
	for _, w := range st.Warnings {
		if strings.Contains(strings.ToLower(w), "permission") || strings.Contains(strings.ToLower(w), "denied") {
			add(w)
		}
	}
	return gaps
}

func hasEvidenceReason(st model.InvestigationState, needles ...string) bool {
	match := func(s string) bool {
		s = strings.ToLower(s)
		for _, n := range needles {
			if strings.Contains(s, n) {
				return true
			}
		}
		return false
	}
	for _, e := range st.LiveEvidence {
		if match(e.Reason + " " + e.Message) {
			return true
		}
	}
	for _, e := range st.Snapshot.Events {
		if match(e.Reason + " " + e.Message) {
			return true
		}
	}
	return false
}

func hasPVCEvidence(st model.InvestigationState) bool {
	if len(st.Snapshot.PVCRefs) == 0 {
		return false
	}
	return hasEvidenceReason(st, "pvc", "persistentvolume", "failedmount", "volume")
}

func nextVerificationSteps(st model.InvestigationState) []string {
	return investigationNextSteps(st)
}
