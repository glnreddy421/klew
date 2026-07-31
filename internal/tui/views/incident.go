package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/model"
)

// IncidentView — tab 1 Incident: Does it answer "What broke?" in under 5 seconds?
func IncidentView(st model.InvestigationState, width, scroll, height int) string {
	s := engine.BuildIncidentSummary(st)
	cw := width - 4
	if cw < 20 {
		cw = 20
	}

	issuePanel := Panel("Issue", width, " "+issueLine(st, s, cw-1))
	issueLines := 3

	half := (width - 1) / 2
	if half < 24 {
		half = 24
	}
	rows := height - issueLines - 2
	if rows < 10 {
		rows = 10
	}
	inner := rows - 2
	if inner < 4 {
		inner = 4
	}

	leftBody := paginate(investigationBody(st, s, half), scroll, inner)
	rightBody := paginate(evidenceBody(st, half, rows), scroll, inner)
	leftPanel := PanelH("Current Investigation", half, rows, leftBody)
	rightPanel := PanelH("Investigation Intelligence", half, rows, rightBody)
	return issuePanel + "\n" + TwoCol(leftPanel, rightPanel)
}

func investigationBody(st model.InvestigationState, s engine.IncidentSummary, half int) string {
	cw := half - 4
	if cw < 12 {
		cw = 12
	}

	app := firstNonEmpty(primaryWorkloadName(st.Snapshot), s.Query, "workload")
	leading, hypo, conf := investigationBelief(st, s, cw)

	var out []string
	appendKV := func(k, v string) {
		out = append(out, kvLines(k, v, cw)...)
	}
	appendKV("Application", app)
	appendKV("Status", formatOperationalStatus(st))
	appendKV("Leading Signal", signalStyled(leading))
	appendKV("Hypothesis", hypo)
	appendKV("Confidence", conf)

	if rc := recentChangeLines(st, cw); len(rc) > 0 {
		out = append(out, sectionGap(), sectionRule(cw), sectionGap())
		out = append(out, headStyle.Render("Recent Change"))
		out = append(out, rc...)
	}

	out = append(out, sectionGap(), sectionRule(cw), sectionGap())
	out = append(out, headStyle.Render("Current Impact"))
	for _, line := range impactLines(st, s, cw) {
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func evidenceBody(st model.InvestigationState, half, rows int) string {
	sigs := rankedSignals(st.Verdict)
	maxCount := 1
	for _, s := range sigs {
		if s.Count > maxCount {
			maxCount = s.Count
		}
	}
	labelW := half - 24
	if labelW < 10 {
		labelW = 10
	}
	maxSigs := rows - 10
	if maxSigs < 3 {
		maxSigs = 3
	}
	if maxSigs > 6 {
		maxSigs = 6
	}

	var out []string
	for i, s := range sigs {
		if i >= maxSigs {
			break
		}
		obs := s.Count
		if obs <= 0 {
			obs = 1
		}
		out = append(out, fmt.Sprintf("%s %s %s",
			signalBar(obs, maxCount, 10), padRight(truncVisual(s.Label, labelW), labelW),
			dimStyle.Render(fmt.Sprintf("%d obs", obs))))
	}
	if len(out) == 0 {
		out = append(out, dimStyle.Render("no evidence in window yet"))
	}

	w := half - 4
	if w < 12 {
		w = 12
	}
	out = append(out, sectionGap(), sectionRule(w), sectionGap())
	out = append(out, hypothesisSection(st, w)...)

	if chain := causalChainLines(st.CausalChain, w); len(chain) > 0 {
		out = append(out, sectionGap(), headStyle.Render("Causal chain"))
		out = append(out, chain...)
	}
	if len(st.Correlation) > 0 {
		out = append(out, "")
		out = append(out, headStyle.Render("Correlation"))
		for _, c := range st.Correlation {
			out = append(out, "  "+okStyle.Render(truncVisual(c, w-2)))
		}
	}
	if len(st.NextChecks) > 0 {
		out = append(out, "")
		out = append(out, headStyle.Render("Next steps"))
		for _, c := range st.NextChecks {
			out = append(out, "  "+dimStyle.Render("→ ")+truncVisual(c, w-4))
		}
	}
	if len(st.FixActions) > 0 {
		out = append(out, "")
		out = append(out, headStyle.Render("Suggested fix"))
		for _, c := range st.FixActions {
			out = append(out, "  "+okStyle.Render("→ ")+truncVisual(c, w-4))
		}
	}
	return strings.Join(out, "\n")
}

// issueLine is the first thing the eye should land on — what is broken right now.
func issueLine(st model.InvestigationState, s engine.IncidentSummary, cw int) string {
	if s.Status == model.VerdictHealthy {
		app := firstNonEmpty(primaryWorkloadName(st.Snapshot), s.Query, "workload")
		return issueStyle.Render("Issue: ") + okStyle.Render(app+" is operating normally")
	}

	subject := issueSubject(st, s)
	detail := issueDetail(st, s)
	line := "Issue: " + subject
	if detail != "" {
		line += " — " + detail
	}
	return issueStyle.Render(truncVisual(line, cw))
}

func issueSubject(st model.InvestigationState, s engine.IncidentSummary) string {
	if pod := worstPodName(st.Snapshot.Pods); pod != "" {
		return "Pod " + pod
	}
	return firstNonEmpty(primaryWorkloadName(st.Snapshot), s.Query, "Workload")
}

func issueDetail(st model.InvestigationState, s engine.IncidentSummary) string {
	signal := strings.ToLower(firstNonEmpty(s.LeadingSignal, st.Verdict.LeadingSignal))
	switch {
	case strings.Contains(signal, "oom"):
		return "exceeded memory limit"
	case strings.Contains(signal, "crash"), strings.Contains(signal, "backoff"):
		return "is repeatedly restarting"
	case strings.Contains(signal, "image"):
		return "cannot pull container image"
	case strings.Contains(signal, "readiness"), strings.Contains(signal, "unhealthy"):
		return "is failing readiness checks"
	case strings.Contains(signal, "endpoint"):
		return "has no ready endpoints"
	case signal != "" && signal != "none":
		return strings.ToLower(s.LeadingSignal)
	}
	if s.UnreadyPods > 0 {
		return fmt.Sprintf("%d pods not ready", s.UnreadyPods)
	}
	return firstNonEmpty(st.HypothesisLabel, s.LikelyTrigger)
}

func worstPodName(pods []model.PodSummary) string {
	ranked := rankPodsForTriage(pods)
	for _, p := range ranked {
		if podHealthLabel(p) != "healthy" {
			return p.Name
		}
	}
	return ""
}

// investigationBelief renders leading signal, hypothesis and confidence.
func investigationBelief(st model.InvestigationState, s engine.IncidentSummary, cw int) (leading, hypo, conf string) {
	if s.Status == model.VerdictHealthy {
		return "None", okStyle.Render("No active incident"), dimStyle.Render("--")
	}
	leading = firstNonEmpty(s.LeadingSignal, "None")
	hypo = truncVisual(firstNonEmpty(st.HypothesisLabel, s.LikelyTrigger, "Collecting evidence…"), cw-13)
	conf = dimStyle.Render("--")
	if s.Confidence > 0 {
		conf = confidencePhrase(s.Confidence, st.ConfidenceTrend)
	}
	return leading, hypo, conf
}

func recentChangeLines(st model.InvestigationState, cw int) []string {
	var out []string
	add := func(k, v string) {
		if strings.TrimSpace(v) == "" {
			return
		}
		out = append(out, "  "+metaKV(k, truncVisual(v, cw-visualWidth(k)-6)))
	}
	if rc := st.RecentChange; rc != nil {
		switch {
		case rc.RevisionFrom != "" && rc.RevisionTo != "":
			add("Revision", rc.RevisionFrom+" → "+rc.RevisionTo)
		case rc.RevisionTo != "":
			add("Revision", rc.RevisionTo)
		}
		if !rc.DeployedAt.IsZero() {
			add("Deployed", humanizeAgo(time.Since(rc.DeployedAt.Time())))
		}
		add("Image", rc.Image)
		return out
	}
	if rev := workloadRevision(st.Snapshot); rev != "" {
		add("Revision", rev)
	}
	add("Image", firstPodImage(st.Snapshot))
	return out
}

func impactLines(st model.InvestigationState, s engine.IncidentSummary, width int) []string {
	var out []string
	appendKV := func(k, v string) {
		for _, line := range kvLines(k, v, width) {
			out = append(out, "  "+line)
		}
	}
	appendKV("Ready Pods", fmt.Sprintf("%d / %d", s.ReadyPods, s.ReadyPods+s.UnreadyPods))
	appendKV("Affected Pods", fmt.Sprintf("%d", s.AffectedPods))
	appendKV("Restarts", fmt.Sprintf("%d", s.Restarts))
	appendKV("Ready Endpoints", fmt.Sprintf("%d / %d", s.EndpointsReady, s.EndpointsTotal))
	if s.EndpointsTotal > 0 {
		appendKV("Service Availability", fmt.Sprintf("%d%%", s.EndpointsReady*100/s.EndpointsTotal))
	}
	m := st.Snapshot.Metrics
	if v := resourceValue(m.Available, m.CPUUsageM, m.CPURequestM, m.CPULimitM, cpuStr); v != "" {
		appendKV("CPU", v)
	}
	if v := resourceValue(m.Available, m.MemUsageMi, m.MemRequestMi, m.MemLimitMi, memStr); v != "" {
		appendKV("Memory", v)
	}
	return out
}

func resourceValue(available bool, usage, req, limit int64, fmtVal func(int64) string) string {
	if available && usage > 0 {
		denom, suffix := limit, ""
		if denom <= 0 {
			denom, suffix = req, " (req)"
		}
		if denom <= 0 {
			return fmtVal(usage)
		}
		bar := meter(float64(usage)/float64(denom), 10)
		return fmt.Sprintf("%s %s / %s%s", bar, fmtVal(usage), fmtVal(denom), suffix)
	}
	switch {
	case req > 0 && limit > 0:
		return dimStyle.Render(fmt.Sprintf("%s req · %s limit", fmtVal(req), fmtVal(limit)))
	case req > 0:
		return dimStyle.Render(fmtVal(req) + " req")
	case limit > 0:
		return dimStyle.Render(fmtVal(limit) + " limit")
	default:
		return ""
	}
}

func resourceLine(name string, available bool, usage, req, limit int64, fmtVal func(int64) string) string {
	if v := resourceValue(available, usage, req, limit, fmtVal); v != "" {
		return kv(name, v)
	}
	return ""
}

func hypothesisSection(st model.InvestigationState, w int) []string {
	var out []string
	hdr := headStyle.Render("Current hypothesis")
	if st.HypothesisChanges > 0 {
		hdr += warnStyle.Render(fmt.Sprintf("  ⟳ x%d", st.HypothesisChanges))
	}
	out = append(out, hdr)

	label := firstNonEmpty(st.HypothesisLabel, "collecting…")
	conf := confidencePhrase(st.Verdict.Confidence, st.ConfidenceTrend)
	room := w - 2 - visualWidth(conf) - 1
	if room < 8 {
		room = 8
	}
	line := "  " + truncVisual(label, room)
	if conf != "" {
		line += " " + conf
	}
	out = append(out, line)

	if t := st.LastTransition; t != nil && t.From != "" {
		delta := ""
		if t.ConfDelta != 0 {
			delta = fmt.Sprintf("  (%+.0f%%)", t.ConfDelta*100)
		}
		out = append(out, "  "+dimStyle.Render(truncVisual(fmt.Sprintf("was %s → now %s%s", t.From, t.To, delta), w-2)))
	}
	if len(st.HypothesisReasons) > 0 {
		out = append(out, "  "+kv("Reason", truncVisual(strings.Join(st.HypothesisReasons, " · "), w-10)))
	}
	return out
}

func causalChainLines(chain []string, w int) []string {
	if len(chain) == 0 {
		return nil
	}
	joined := strings.Join(chain, " → ")
	wrapped := wrap(joined, w-2, 3)
	var out []string
	for _, ln := range strings.Split(wrapped, "\n") {
		out = append(out, "  "+ln)
	}
	return out
}

func cpuStr(m int64) string { return fmt.Sprintf("%dm", m) }

func memStr(mi int64) string { return humanizeMem(mi) }

func primaryWorkloadName(b model.EvidenceBundle) string {
	name, _ := primaryWorkload(b)
	if name == "unknown" {
		return ""
	}
	return name
}

func firstPodImage(b model.EvidenceBundle) string {
	for _, p := range b.Pods {
		for _, c := range p.Containers {
			if c.Image != "" {
				return c.Image
			}
		}
	}
	return ""
}

func workloadRevision(b model.EvidenceBundle) string {
	if len(b.Workloads) > 0 && b.Workloads[0].Generation > 0 {
		return fmt.Sprintf("%d", b.Workloads[0].Generation)
	}
	return ""
}

func humanizeAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

func humanizeMem(mi int64) string {
	if mi >= 1024 {
		return fmt.Sprintf("%.1fGi", float64(mi)/1024)
	}
	return fmt.Sprintf("%dMi", mi)
}

func primaryWorkload(b model.EvidenceBundle) (name, kind string) {
	if len(b.Workloads) > 0 {
		return b.Workloads[0].Name, b.Workloads[0].Kind
	}
	if len(b.MatchedObjects) > 0 {
		return b.MatchedObjects[0].Ref.Name, b.MatchedObjects[0].Ref.Kind
	}
	return firstNonEmpty(b.Query, "unknown"), "Query"
}

func rankedSignals(v model.Verdict) []model.Signal {
	all := append([]model.Signal{}, v.StrongSignals...)
	all = append(all, v.MediumSignals...)
	all = append(all, v.WeakSignals...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	return all
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func nsLabel(st model.InvestigationState) string {
	if st.NamespaceScope.AllNamespaces {
		return "all namespaces"
	}
	if st.NamespaceScope.Primary != "" {
		return st.NamespaceScope.Primary
	}
	return st.Snapshot.Namespace
}

func wrap(s string, width, maxLines int) string {
	if width < 10 {
		width = 10
	}
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if visualWidth(cur)+1+visualWidth(w) <= width {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = truncVisual(lines[maxLines-1]+" …", width)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}
