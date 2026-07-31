package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/model"
)

// TimelineView — tab 2 Timeline: Does it explain how the incident unfolded?
func TimelineView(st model.InvestigationState, width, scroll, height int, cat, search string, expand bool) string {
	rows := height - 2
	if rows < 3 {
		rows = 3
	}

	evs := filterTimeline(st.Timeline, cat, search)
	runs := foldRuns(evs, expand)
	objW := clampInt(width-56, 14, 26)

	// Deterministic investigation moments. Absent ones simply never render — we
	// never invent a recovery/rollback that the evidence doesn't show.
	deployIdx, failureIdx, peakIdx, rollbackIdx, recoveryIdx, lastFailIdx := -1, -1, -1, -1, -1, -1
	peakScore := -1
	hypo := map[int]bool{}
	for i, r := range runs {
		if deployIdx < 0 && isDeployEvent(r.ev) {
			deployIdx = i
		}
		if rollbackIdx < 0 && isRollbackEvent(r.ev) {
			rollbackIdx = i
		}
		if isHypoChange(r.ev) {
			hypo[i] = true
		}
		// the "peak" is the worst genuine failure moment — never an info/warning row
		if rk := severityRankTL(r.ev.Severity); rk >= 3 {
			if failureIdx < 0 {
				failureIdx = i
			}
			lastFailIdx = i
			if sc := rk*1000 + r.count; sc > peakScore {
				peakScore, peakIdx = sc, i
			}
		}
	}
	if peakIdx >= 0 {
		for i := peakIdx + 1; i < len(runs); i++ {
			if isRecoveryEvent(runs[i].ev) {
				recoveryIdx = i
				break
			}
		}
	}
	phase := phaseBoundaries(len(runs), deployIdx, failureIdx)

	// The causal chain (trigger → last failure) is linked with a left connector
	// spine — the deterministic sequence from what started it to the impact.
	chainLo, chainHi := deployIdx, lastFailIdx
	if chainLo < 0 {
		chainLo = 0
	}

	// We record each line's section (and whether it is itself a bookmark header)
	// so the active section can stay pinned at the top while the panel scrolls.
	var lines []string
	var sect []string
	var isHdr []bool
	curSection := ""
	push := func(section string, header bool, ls ...string) {
		for _, l := range ls {
			lines = append(lines, l)
			sect = append(sect, section)
			isHdr = append(isHdr, header)
		}
	}

	curPhase := ""
	var prevTs time.Time
	for i, r := range runs {
		if lbl := phase.at(i); lbl != "" && lbl != curPhase {
			// a hypothesis-change bookmark is more specific; don't stack a phase
			// header directly on top of it, but still advance the phase.
			if !hypo[i] {
				curSection = lbl
				push(curSection, true, bookmarkBlock(lbl, width, false, len(lines) > 0)...)
			}
			curPhase = lbl
		}
		if hypo[i] {
			curSection = "Hypothesis Changed"
			push(curSection, true, bookmarkBlock("Hypothesis Changed", width, true, len(lines) > 0)...)
		}
		ann := ""
		switch i {
		case rollbackIdx:
			ann = "⟵ rollback"
		case deployIdx:
			ann = "⟵ trigger"
		case peakIdx:
			ann = "⟵ peak"
		case recoveryIdx:
			ann = "⟵ recovery"
		}
		connected := chainHi >= 0 && i >= chainLo && i <= chainHi
		push(curSection, false, timelineRows(r, prevTs, expand, objW, ann, connected)...)
		prevTs = r.last
	}
	if len(lines) == 0 {
		push("", false, dimStyle.Render("no timeline events match the current filter"))
	}

	// Current State is always the final section and updates continuously.
	for j, l := range currentStateBlock(st, width) {
		push("Current State", j < 4, l) // leading blank + bar + label + bar
	}

	hint := timelineHint(cat, search, expand, width)
	avail := rows - 1
	start := clampInt(scroll, 0, maxInt(0, len(lines)-1))

	// Pin the active section header when its bookmark has scrolled out of view.
	out := []string{hint}
	if start > 0 && start < len(sect) && !isHdr[start] && sect[start] != "" {
		out = append(out, stickyHeader(sect[start], width))
		avail--
	}
	end := start + avail
	if end > len(lines) {
		end = len(lines)
	}
	if start < end {
		out = append(out, lines[start:end]...)
	}
	return PanelH(timelineTitle(st, len(evs), len(st.Timeline)), width, rows, strings.Join(out, "\n"))
}

// ── run folding ──────────────────────────────────────────────────────────────

type tlRun struct {
	ev    model.TimelineEvent
	count int
	first time.Time
	last  time.Time
	all   []model.TimelineEvent
}

func foldRuns(evs []model.TimelineEvent, expand bool) []tlRun {
	var runs []tlRun
	for _, e := range evs {
		if !expand && len(runs) > 0 {
			last := &runs[len(runs)-1]
			if foldKey(last.ev) == foldKey(e) {
				last.count++
				last.all = append(last.all, e)
				if e.Timestamp.After(last.last) {
					last.last = e.Timestamp
				}
				continue
			}
		}
		runs = append(runs, tlRun{ev: e, count: 1, first: e.Timestamp, last: e.Timestamp, all: []model.TimelineEvent{e}})
	}
	return runs
}

func foldKey(e model.TimelineEvent) string {
	base := e.Reason
	if base == "" {
		base = e.Message
	}
	// strip an existing "×N" suffix so counted rows still fold
	if i := strings.LastIndex(base, "×"); i > 0 {
		base = strings.TrimSpace(base[:i])
	}
	return strings.ToLower(tlCategory(e) + "|" + e.SourceName + "|" + strings.Join(strings.Fields(base), " "))
}

// ── filtering ────────────────────────────────────────────────────────────────

func filterTimeline(evs []model.TimelineEvent, cat, search string) []model.TimelineEvent {
	cat = strings.ToLower(strings.TrimSpace(cat))
	if cat == "solid" {
		cat = "klew" // legacy filter alias
	}
	q := strings.ToLower(strings.TrimSpace(search))
	out := make([]model.TimelineEvent, 0, len(evs))
	for _, e := range evs {
		if cat != "" && cat != "all" && tlCategory(e) != cat {
			continue
		}
		if q != "" {
			hay := strings.ToLower(e.Reason + " " + e.Message + " " + e.SourceName + " " + e.SourceKind)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

// ── phases / bookmarks ─────────────────────────────────────────────────────────

type tlPhases struct {
	idx    []int
	labels []string
}

func phaseBoundaries(n, deployIdx, failureIdx int) tlPhases {
	var p tlPhases
	add := func(i int, label string) {
		if i < 0 || i >= n {
			return
		}
		// dedupe by index — later (more specific) label wins
		for k, ix := range p.idx {
			if ix == i {
				p.labels[k] = label
				return
			}
		}
		p.idx = append(p.idx, i)
		p.labels = append(p.labels, label)
	}
	if n > 0 {
		add(0, "Investigation Started")
	}
	add(deployIdx, "Deployment Changes")
	add(failureIdx, "Failure Begins")
	return p
}

func (p tlPhases) at(i int) string {
	label, best := "", -1
	for k, ix := range p.idx {
		if ix <= i && ix > best {
			best, label = ix, p.labels[k]
		}
	}
	return label
}

// bookmarkBlock renders a strong section break: a full-width rule, the label,
// and another rule — with a leading blank line for breathing room. Reasoning
// bookmarks are purple and prefixed with ★.
func bookmarkBlock(label string, width int, reasoning, leadingBlank bool) []string {
	barW := maxInt(4, width-4)
	bar := headStyle.Render(strings.Repeat("━", barW))
	name := "  " + headStyle.Render("▌ "+label)
	if reasoning {
		name = "  " + klewStyle.Render("★ "+label)
	}
	var out []string
	if leadingBlank {
		out = append(out, "", "")
	}
	return append(out, bar, name, bar)
}

// stickyHeader is the compact, pinned version of the active section's bookmark.
// It uses a thin rule so it reads as a pinned header, not a full section break.
func stickyHeader(label string, width int) string {
	tag := headStyle.Render("▸ " + label)
	fill := maxInt(0, width-4-visualWidth("▸ "+label)-1)
	return tag + " " + markStyle.Render(strings.Repeat("─", fill))
}

// ── rows ─────────────────────────────────────────────────────────────────────

// tlSubIndent aligns sub-lines (folded detail, reasoning) under the object
// column — enough to read as attached detail without running off the panel.
const tlSubIndent = 3 + 9 + 6 + 8 // connector+marker+space, time, delta, type

func tlIndent() string { return strings.Repeat(" ", tlSubIndent) }

func timelineRows(r tlRun, prevTs time.Time, expand bool, objW int, ann string, connected bool) []string {
	// Klew's own reasoning events get an explanatory multi-line block.
	if isKlewEvent(r.ev) {
		return reasoningRows(r.ev, prevTs, objW, ann, connected)
	}
	if expand && r.count > 1 {
		var out []string
		p := prevTs
		for i, e := range r.all {
			a := ""
			if i == 0 {
				a = ann
			}
			out = append(out, tlRowMsg(e, p, e.Message, objW, a, connected))
			p = e.Timestamp
		}
		return out
	}
	out := []string{tlRowMsg(r.ev, prevTs, r.ev.Message, objW, ann, connected)}
	if r.count > 1 {
		out = append(out, foldedDetail(r), "")
	}
	return out
}

// tlRowMsg renders one aligned event row with a given message body.
func tlRowMsg(e model.TimelineEvent, prevTs time.Time, msg string, objW int, ann string, connected bool) string {
	cat := tlCategory(e)

	gutter := " "
	if connected {
		gutter = markStyle.Render("│")
	}
	delta := strings.Repeat(" ", 5)
	if !prevTs.IsZero() {
		if d := e.Timestamp.Sub(prevTs); d > 0 {
			delta = dimStyle.Render(padRight("+"+timelineDur(d), 5))
		}
	}
	ts := dimStyle.Render(e.Timestamp.Format("15:04:05"))
	typeStyled := tlTypeStyle(cat, e.Severity).Render(padRight(tlTypeLabel(e, cat), 7))

	obj := padRight(truncVisual(tlObject(e), objW), objW)
	if e.Severity == model.SeverityInfo && cat != "klew" {
		obj = dimStyle.Render(obj)
	}

	body := tlMsgStyle(cat, e.Severity).Render(msg)
	if ann != "" {
		body += "  " + markStyle.Render(ann)
	}
	return gutter + tlMarker(e, cat) + " " + ts + " " + delta + " " + typeStyled + " " + obj + " " + body
}

// foldedDetail gives context for a collapsed run without expanding it.
func foldedDetail(r tlRun) string {
	txt := fmt.Sprintf("×%d occurrences · first %s · last %s · ⏎ expand",
		r.count, r.first.Format("15:04:05"), r.last.Format("15:04:05"))
	return tlIndent() + dimStyle.Render(txt)
}

// reasoningRows renders a Klew reasoning event and, for hypothesis changes,
// explains WHY it changed: the belief transition and the confidence shift.
func reasoningRows(e model.TimelineEvent, prevTs time.Time, objW int, ann string, connected bool) []string {
	from, to, cf, ct, ok := parseHypoChange(e.Message)
	label := e.Message
	if ok {
		label = "Hypothesis Changed"
	}
	out := []string{tlRowMsg(e, prevTs, label, objW, ann, connected)}
	if ok {
		ind := tlIndent()
		out = append(out, ind+klewStyle.Render(from)+dimStyle.Render(" → ")+klewStyle.Render(to))
		if cf != "" && ct != "" {
			out = append(out, ind+dimStyle.Render("confidence ")+klewStyle.Render(cf)+dimStyle.Render(" → ")+klewStyle.Render(ct))
		}
	}
	return out
}

// parseHypoChange extracts "A → B" and "X% → Y%" from a hypothesis-change
// message like "Hypothesis changed: A → B  (confidence X% → Y%)".
func parseHypoChange(msg string) (from, to, confFrom, confTo string, ok bool) {
	m := msg
	// A trailing parenthetical carries the confidence shift, e.g. "(confidence
	// 70% → 90%)" or simply "(70% → 90%)". Split it off before the belief arrow.
	if i := strings.LastIndex(m, "("); i >= 0 {
		conf := strings.TrimRight(strings.TrimSpace(m[i+1:]), ")")
		if j := strings.Index(strings.ToLower(conf), "confidence"); j >= 0 {
			conf = conf[j+len("confidence"):]
		}
		if strings.Contains(conf, "%") {
			if a, b, ok2 := splitArrow(conf); ok2 {
				confFrom, confTo = strings.TrimSpace(a), strings.TrimSpace(b)
			}
			m = m[:i]
		}
	}
	if i := strings.Index(strings.ToLower(m), "changed:"); i >= 0 {
		m = m[i+len("changed:"):]
	}
	if a, b, ok2 := splitArrow(m); ok2 {
		return strings.TrimSpace(a), strings.TrimSpace(b), confFrom, confTo, true
	}
	return "", "", "", "", false
}

func splitArrow(s string) (string, string, bool) {
	for _, sep := range []string{"→", "->"} {
		if i := strings.Index(s, sep); i >= 0 {
			return s[:i], s[i+len(sep):], true
		}
	}
	return "", "", false
}

// currentStateBlock is the always-final section: what Klew currently believes,
// its confidence, and a live indicator that the investigation continues.
func currentStateBlock(st model.InvestigationState, width int) []string {
	out := bookmarkBlock("Current State", width, false, true)
	ind := "  "
	out = append(out, ind+kv("Hypothesis", firstNonEmpty(st.HypothesisLabel, "No active incident")))
	if st.Verdict.Confidence > 0 {
		out = append(out, ind+kv("Confidence", fmt.Sprintf("%s %.0f%%", engine.ConfidenceLabel(st.Verdict.Confidence), st.Verdict.Confidence*100)))
	}
	out = append(out, ind+kv("Status", statusPhrase(InvestigationPhase(st))))
	if st.Mode == model.ModeLive {
		out = append(out, ind+okStyle.Render("● watching live")+dimStyle.Render(fmt.Sprintf(" · %d sources", len(st.ActiveWatches))))
	} else {
		out = append(out, ind+dimStyle.Render("○ historical bundle — not live"))
	}
	return out
}

// ── classification / styling ────────────────────────────────────────────────

func tlCategory(e model.TimelineEvent) string {
	switch {
	case isKlewEvent(e):
		return "klew"
	case e.Type == "log":
		return "logs"
	case e.Type == "metric":
		return "metrics"
	case e.Type == "event" || e.Type == "k8s_event":
		return "events"
	default:
		return "objects"
	}
}

// isKlewEvent reports whether a timeline entry is one of Klew's own reasoning
// events (rendered in purple). Legacy SourceKind "Solid" is still accepted.
func isKlewEvent(e model.TimelineEvent) bool {
	return e.Type == "system" || e.SourceKind == "Klew" || e.SourceKind == "Solid"
}

func isHypoChange(e model.TimelineEvent) bool {
	if !isKlewEvent(e) {
		return false
	}
	s := strings.ToLower(e.Reason + " " + e.Message)
	return strings.Contains(s, "hypothesis")
}

func isDeployEvent(e model.TimelineEvent) bool {
	switch e.Type {
	case "deploy", "rollout", "rs":
		return true
	}
	return e.SourceKind == "Deployment" || e.SourceKind == "ReplicaSet"
}

// isRollbackEvent / isRecoveryEvent detect optional investigation moments from
// deterministic evidence only. They stay dormant unless the signals appear.
func isRollbackEvent(e model.TimelineEvent) bool {
	s := strings.ToLower(e.Reason + " " + e.Message)
	return strings.Contains(s, "rollback") || strings.Contains(s, "rolled back") || strings.Contains(s, "revert")
}

func isRecoveryEvent(e model.TimelineEvent) bool {
	if severityRankTL(e.Severity) >= 2 {
		return false // still a problem, not a recovery
	}
	s := strings.ToLower(e.Reason + " " + e.Message)
	for _, kw := range []string{"recovered", "became ready", "back to", "healthy again", "endpoints restored", "scaled up"} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func tlTypeLabel(e model.TimelineEvent, cat string) string {
	if cat == "klew" {
		return "KLEW"
	}
	switch e.Type {
	case "log":
		return "LOG"
	case "metric":
		return "METRIC"
	case "event", "k8s_event":
		return "EVENT"
	case "object_change":
		return "CHANGE"
	case "deploy", "rollout":
		return "DEPLOY"
	case "rs":
		return "RS"
	case "pod":
		return "POD"
	case "container":
		return "CTR"
	case "service":
		return "SVC"
	}
	switch e.SourceKind {
	case "Deployment":
		return "DEPLOY"
	case "ReplicaSet":
		return "RS"
	case "Pod":
		return "POD"
	case "Service":
		return "SVC"
	case "Container":
		return "CTR"
	}
	return "EVENT"
}

func tlObject(e model.TimelineEvent) string {
	if e.SourceName != "" {
		return e.SourceName
	}
	return e.InvolvedObject.Name
}

func tlSevStyleTL(sev model.Severity) lipgloss.Style {
	switch sev {
	case model.SeverityCritical:
		return critStyle
	case model.SeverityHigh:
		return highStyle
	case model.SeverityWarning:
		return warnStyle
	case model.SeverityInfo:
		return dimStyle
	default:
		return okStyle
	}
}

func tlMarker(e model.TimelineEvent, cat string) string {
	switch cat {
	case "klew":
		return klewStyle.Render("★")
	case "metrics":
		return metricStyle.Render("◆")
	default:
		return tlSevStyleTL(e.Severity).Render("●")
	}
}

func tlTypeStyle(cat string, sev model.Severity) lipgloss.Style {
	switch cat {
	case "klew":
		return klewStyle
	case "metrics":
		return metricStyle
	default:
		return tlSevStyleTL(sev)
	}
}

func tlMsgStyle(cat string, sev model.Severity) lipgloss.Style {
	if cat == "klew" {
		return klewStyle
	}
	if sev == model.SeverityInfo {
		return dimStyle
	}
	return plainStyle
}

// ── header / hint / helpers ────────────────────────────────────────────────────

func timelineTitle(st model.InvestigationState, shown, total int) string {
	app := firstNonEmpty(primaryWorkloadName(st.Snapshot), st.Query, "workload")
	if shown == total {
		return fmt.Sprintf("Timeline · %s · %d events", app, total)
	}
	return fmt.Sprintf("Timeline · %s · %d/%d events", app, shown, total)
}

func timelineHint(cat, search string, expand bool, width int) string {
	active := cat
	if active == "" {
		active = "all"
	}
	seg := func(key, label string) string {
		if active == label {
			return headStyle.Render(key)
		}
		return dimStyle.Render(key)
	}
	parts := []string{
		seg("a", "all"), seg("l", "logs"), seg("e", "events"),
		seg("m", "metrics"), seg("o", "objects"), seg("s", "klew"),
	}
	line := dimStyle.Render("filter ") + strings.Join(parts, dimStyle.Render("·"))
	line += dimStyle.Render("   / search   ⏎ ")
	if expand {
		line += headStyle.Render("expanded")
	} else {
		line += dimStyle.Render("expand")
	}
	if strings.TrimSpace(search) != "" {
		line += dimStyle.Render("   search=") + warnStyle.Render(search)
	}
	return truncVisual(line, width-4)
}

func timelineDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}

func severityRankTL(s model.Severity) int {
	switch s {
	case model.SeverityCritical:
		return 4
	case model.SeverityHigh:
		return 3
	case model.SeverityWarning:
		return 2
	case model.SeverityInfo:
		return 0
	default:
		return 1
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// scopeCounts is the affected-resource tally shown in Blast Radius.
type scopeCounts struct{ critical, warning, healthy, pods, podsFailing int }

func affectedCounts(b model.EvidenceBundle) scopeCounts {
	var c scopeCounts
	tally := func(h string) {
		switch strings.ToLower(h) {
		case "critical":
			c.critical++
		case "warning", "degraded", "unknown":
			c.warning++
		default:
			c.healthy++
		}
	}
	for range b.Ingresses {
		tally("healthy")
	}
	for _, svc := range b.Services {
		tally(serviceHealth(svc))
	}
	for _, w := range b.Workloads {
		tally(workloadHealth(w))
	}
	for _, rs := range b.ReplicaSets {
		if rs.Ready < rs.Replicas {
			tally("warning")
		} else {
			tally("healthy")
		}
	}
	for _, p := range b.Pods {
		h := podHealthLabel(p)
		tally(h)
		c.pods++
		if h != "healthy" {
			c.podsFailing++
		}
	}
	for range b.ConfigRefs {
		tally("healthy")
	}
	for range b.SecretRefs {
		tally("healthy")
	}
	for range b.PVCRefs {
		tally("healthy")
	}
	for _, n := range b.Nodes {
		if !n.Ready || n.MemoryPressure || n.DiskPressure {
			tally("warning")
		} else {
			tally("healthy")
		}
	}
	return c
}

// blastRadiusLines shows how bad and how far: an affected-resource tally plus
// the propagation chain.
func blastRadiusLines(st model.InvestigationState, b model.EvidenceBundle) []string {
	c := affectedCounts(b)
	summary := fmt.Sprintf("%s   %s   %s   %s",
		critStyle.Render(fmt.Sprintf("✖ %d critical", c.critical)),
		warnStyle.Render(fmt.Sprintf("⚠ %d warning", c.warning)),
		okStyle.Render(fmt.Sprintf("✓ %d healthy", c.healthy)),
		markStyle.Render(fmt.Sprintf("· %d pods, %d failing", c.pods, c.podsFailing)))
	out := []string{"  " + summary}
	return out
}

// scopeItem is one resource line in a scope group.
type scopeItem struct{ health, label, fact string }

// scopeGroup is a health-annotated, category-based grouping of scoped
// resources. Grouping is data-driven so collapsible sections (▼/▶) can be added
// later without changing the renderer.
type scopeGroup struct {
	name  string
	items []scopeItem
}

// buildScopeGroups organises the discovered resources into investigation
// categories, each item carrying the single most useful fact.
func buildScopeGroups(b model.EvidenceBundle) []scopeGroup {
	var groups []scopeGroup
	add := func(name string, items []scopeItem) {
		if len(items) > 0 {
			groups = append(groups, scopeGroup{name, items})
		}
	}

	var net []scopeItem
	if len(b.Ingresses) > 0 {
		fact := "admits traffic"
		if len(b.Services) > 0 {
			fact = "routes traffic to " + b.Services[0].Name
		}
		net = append(net, scopeItem{"healthy", "Ingress", fact})
	}
	for _, svc := range b.Services {
		net = append(net, scopeItem{serviceHealth(svc), "Service",
			fmt.Sprintf("endpoints %d/%d", svc.ReadyEndpoints, svc.TotalEndpoints)})
		break
	}
	add("Networking", net)

	var wl []scopeItem
	for _, w := range b.Workloads {
		wl = append(wl, scopeItem{workloadHealth(w), firstNonEmpty(w.Kind, "Workload"),
			fmt.Sprintf("revision %d", w.Generation)})
		break
	}
	if len(b.ReplicaSets) > 0 {
		health := "healthy"
		for _, rs := range b.ReplicaSets {
			if rs.Ready < rs.Replicas {
				health = "warning"
			}
		}
		fact := fmt.Sprintf("%d total", len(b.ReplicaSets))
		if cur, ok := activeReplicaSet(b); ok {
			fact += " · " + cur.Name + " current"
		}
		wl = append(wl, scopeItem{health, "ReplicaSets", fact})
	}
	if len(b.Pods) > 0 {
		c := affectedCounts(b)
		fact := fmt.Sprintf("%d pods", c.pods)
		if c.podsFailing > 0 {
			fact = fmt.Sprintf("%d pods · %d failing", c.pods, c.podsFailing)
			if r := topPodReason(b); r != "" {
				fact += " · " + r
			}
		}
		wl = append(wl, scopeItem{podsOverallHealth(b.Pods), "Pods", fact})
	}
	add("Workloads", wl)

	var cfg []scopeItem
	if len(b.ConfigRefs) > 0 {
		cfg = append(cfg, scopeItem{"healthy", "ConfigMap", "referenced by Deployment"})
	}
	if len(b.SecretRefs) > 0 {
		cfg = append(cfg, scopeItem{"healthy", "Secret", "mounted into Pods"})
	}
	add("Configuration", cfg)

	var stg []scopeItem
	if len(b.PVCRefs) > 0 {
		stg = append(stg, scopeItem{"healthy", "PVC", fmt.Sprintf("%d bound", len(b.PVCRefs))})
	}
	add("Storage", stg)

	var infra []scopeItem
	if len(b.Nodes) > 0 {
		health, fact := "healthy", fmt.Sprintf("%d nodes", len(b.Nodes))
		for _, n := range b.Nodes {
			if !n.Ready || n.MemoryPressure || n.DiskPressure {
				health = "warning"
				fact = fmt.Sprintf("%d nodes · %s %s", len(b.Nodes), n.Name, nodePressureNote(n))
				break
			}
		}
		infra = append(infra, scopeItem{health, "Nodes", fact})
	}
	add("Infrastructure", infra)

	return groups
}

// investigationScopeLines renders the grouped scope report plus a scope summary.
func investigationScopeLines(st model.InvestigationState, b model.EvidenceBundle) []string {
	var out []string
	for gi, g := range buildScopeGroups(b) {
		if gi > 0 {
			out = append(out, "")
		}
		out = append(out, headStyle.Render("▼ "+g.name))
		for _, it := range g.items {
			out = append(out, scopeItemLine(it, g.name))
		}
		// expand failing pods under Workloads
		if g.name == "Workloads" && len(b.Pods) > 0 {
			for _, p := range sortedPodsForGraph(b.Pods) {
				out = append(out, podScopeLine(p))
			}
		}
		out = append(out, "")
	}
	return trimTrailing(out)
}

func scopeItemLine(it scopeItem, group string) string {
	indent := "  "
	label := padRight(it.label, 12)
	// ReplicaSets are secondary — de-emphasize
	if it.label == "ReplicaSets" {
		return indent + dimStyle.Render(scopeMark(it.health)+" "+label+" "+it.fact)
	}
	return indent + fmt.Sprintf("%s %s %s",
		scopeMark(it.health), graphLabel(label, it.health), graphNote(it.health, it.fact))
}

func podScopeLine(p model.PodSummary) string {
	health := podHealthLabel(p)
	c := worstContainer(p)
	fact := podReadinessLabel(p, health)
	if c.LastReason != "" {
		fact = c.LastReason
		if c.LastExitCode > 0 {
			fact += fmt.Sprintf(" · exit %d", c.LastExitCode)
		}
	}
	return "      " + fmt.Sprintf("%s %s %s",
		scopeMark(health), graphLabel(truncVisual(p.Name, 28), health), graphNote(health, fact))
}

func sortedPodsForGraph(pods []model.PodSummary) []model.PodSummary {
	out := append([]model.PodSummary(nil), pods...)
	sort.SliceStable(out, func(i, j int) bool {
		hi, hj := healthRank(podHealthLabel(out[i])), healthRank(podHealthLabel(out[j]))
		if hi != hj {
			return hi > hj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// scopeSummaryLines reassures the engineer what Klew is actively observing.
func scopeSummaryLines(st model.InvestigationState, b model.EvidenceBundle) []string {
	resources := len(b.Workloads) + len(b.ReplicaSets) + len(b.Pods) + len(b.Services) +
		len(b.Ingresses) + len(b.HPAs) + len(b.ConfigRefs) + len(b.SecretRefs) + len(b.PVCRefs) + len(b.Nodes)
	rel := len(st.WorkloadGraph.Edges)
	if rel == 0 {
		rel = len(b.ReplicaSets) + len(b.Pods)*2 + len(b.Services) + len(b.Ingresses) +
			len(b.ConfigRefs) + len(b.SecretRefs) + len(b.PVCRefs)
	}
	metrics := "n/a"
	if st.Snapshot.Metrics.Available || st.Snapshot.Metrics.MemLimitMi > 0 {
		metrics = "live"
	}
	parts := strings.Join([]string{
		fmt.Sprintf("%d resources", resources),
		fmt.Sprintf("%d relationships", rel),
		fmt.Sprintf("%d pods", len(b.Pods)),
		fmt.Sprintf("%d events", st.Counters.EventsIngested),
		fmt.Sprintf("%d logs", st.Counters.LogsIngested),
		"metrics " + metrics,
	}, " · ")
	return []string{
		headStyle.Render("▼ Scope Summary"),
		"  " + markStyle.Render("watching "+parts),
	}
}

// scopeMark returns the health glyph for a scope item (✓ / ⚠ / ✖).
func scopeMark(health string) string {
	switch strings.ToLower(health) {
	case "critical":
		return critStyle.Render("✖")
	case "warning", "degraded", "unknown":
		return warnStyle.Render("⚠")
	default:
		return okStyle.Render("✓")
	}
}

// topPodReason surfaces the most relevant failure reason across scoped pods.
func topPodReason(b model.EvidenceBundle) string {
	for _, p := range b.Pods {
		for _, c := range p.Containers {
			if c.LastReason != "" {
				return c.LastReason
			}
		}
	}
	return ""
}

func workloadHealth(w model.WorkloadSummary) string {
	if w.Replicas > 0 && w.Ready == 0 {
		return "critical"
	}
	if w.Ready < w.Replicas {
		return "warning"
	}
	return "healthy"
}

// activeReplicaSet returns the ReplicaSet with the most replicas (the one the
// Deployment currently rolls onto).
func activeReplicaSet(b model.EvidenceBundle) (model.ReplicaSetSummary, bool) {
	if len(b.ReplicaSets) == 0 {
		return model.ReplicaSetSummary{}, false
	}
	best := b.ReplicaSets[0]
	for _, rs := range b.ReplicaSets[1:] {
		if rs.Replicas > best.Replicas {
			best = rs
		}
	}
	return best, true
}

func podsOverallHealth(pods []model.PodSummary) string {
	crit, warn := false, false
	for _, p := range pods {
		switch podHealthLabel(p) {
		case "critical":
			crit = true
		case "warning", "unknown":
			warn = true
		}
	}
	switch {
	case crit:
		return "critical"
	case warn:
		return "warning"
	default:
		return "healthy"
	}
}

// graphStoryLines renders the propagation path as a vertical causal chain, one
// step per line connected by "↓", or a derived sentence when the engine has not
// inferred a path yet.
func graphStoryLines(st model.InvestigationState, b model.EvidenceBundle) []string {
	p := st.WorkloadGraph.PropagationPath
	if len(p) == 0 {
		return []string{"  " + dimStyle.Render(propagationSentence(b))}
	}
	out := make([]string, 0, len(p))
	for i, s := range p {
		prefix := "  "
		if i > 0 {
			prefix = markStyle.Render("  ↓ ")
		}
		out = append(out, prefix+s)
	}
	return out
}

// graphLabel colours failing resources (red / orange) and leaves healthy ones
// in the default weight — healthy resources are never over-coloured.
func graphLabel(text, health string) string {
	switch strings.ToLower(health) {
	case "critical":
		return critStyle.Render(text)
	case "warning", "degraded", "unknown":
		return highStyle.Render(text)
	default:
		return plainStyle.Render(text)
	}
}

func graphNote(health, note string) string {
	switch strings.ToLower(health) {
	case "critical":
		return critStyle.Render(note)
	case "warning", "degraded":
		return warnStyle.Render(note)
	default:
		return markStyle.Render(note)
	}
}

func nodePressureNote(n model.NodeSummary) string {
	var out []string
	if !n.Ready {
		out = append(out, "NotReady")
	}
	if n.MemoryPressure {
		out = append(out, "MemoryPressure")
	}
	if n.DiskPressure {
		out = append(out, "DiskPressure")
	}
	if len(out) == 0 {
		return "ready"
	}
	return strings.Join(out, " · ")
}

// graphScope folds the old Objects tab into one inventory line.
func graphScope(b model.EvidenceBundle) string {
	var parts []string
	add := func(n int, singular string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, pluralize(n, singular)))
		}
	}
	add(len(b.Workloads), "workload")
	add(len(b.ReplicaSets), "ReplicaSet")
	add(len(b.Pods), "Pod")
	add(len(b.Services), "Service")
	add(len(b.Ingresses), "Ingress")
	add(len(b.HPAs), "HPA")
	add(len(b.ConfigRefs), "ConfigMap")
	add(len(b.SecretRefs), "Secret")
	add(len(b.PVCRefs), "PVC")
	add(len(b.Nodes), "Node")
	if len(parts) == 0 {
		return dimStyle.Render("no objects in scope")
	}
	return markStyle.Render(strings.Join(parts, " · "))
}

func pluralize(n int, s string) string {
	if n == 1 {
		return s
	}
	return s + "s"
}

func serviceHealth(s model.ServiceSummary) string {
	if s.ReadyEndpoints == 0 {
		return "critical"
	}
	if s.ReadyEndpoints < s.TotalEndpoints {
		return "warning"
	}
	return "healthy"
}

func podHealthLabel(p model.PodSummary) string {
	for _, c := range p.Containers {
		if c.LastReason == "OOMKilled" || strings.Contains(strings.ToLower(c.LastReason), "crash") {
			return "critical"
		}
	}
	if p.Ready && p.Phase == "Running" {
		return "healthy"
	}
	if p.RestartCount > 3 {
		return "critical"
	}
	if !p.Ready {
		return "warning"
	}
	return "unknown"
}

func podsForService(b model.EvidenceBundle, svc model.ServiceSummary) []model.PodSummary {
	var pods []model.PodSummary
	for _, p := range b.Pods {
		if svc.Selector == "" || selectorMatches(svc.Selector, p.Labels) {
			pods = append(pods, p)
		}
	}
	if len(pods) == 0 {
		pods = b.Pods
	}
	sort.SliceStable(pods, func(i, j int) bool {
		hi, hj := healthRank(podHealthLabel(pods[i])), healthRank(podHealthLabel(pods[j]))
		if hi != hj {
			return hi > hj
		}
		return pods[i].Name < pods[j].Name
	})
	return pods
}

func healthRank(h string) int {
	switch h {
	case "critical":
		return 3
	case "warning":
		return 2
	case "unknown":
		return 1
	default:
		return 0
	}
}

func selectorMatches(selector string, labels map[string]string) bool {
	if len(labels) == 0 {
		return false
	}
	for _, part := range strings.Split(selector, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if labels[kv[0]] != kv[1] {
			return false
		}
	}
	return true
}

func shortImage(img string) string {
	if i := strings.LastIndex(img, "/"); i >= 0 {
		return img[i+1:]
	}
	return img
}

func propagationSentence(b model.EvidenceBundle) string {
	if len(b.Workloads) > 0 {
		return fmt.Sprintf("rollout revision %d → new pods → failures → endpoints dropped", b.Workloads[0].Generation)
	}
	return "no propagation path inferred yet"
}

func trimTrailing(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
