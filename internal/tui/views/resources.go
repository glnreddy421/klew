package views

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/render"
)

// ResourcesView — tab 5 Resources: Can I tell whether CPU/memory contributed?
func ResourcesView(st model.InvestigationState, width, height int) string {
	b := st.Snapshot
	m := resolveMetrics(b)

	half := (width - 1) / 2
	if half < 24 {
		half = 24
	}

	chrome := 6
	contentH := height - chrome
	if contentH < 14 {
		contentH = 14
	}
	topH := clampInt(contentH*28/100, 8, 11)
	bottomH := clampInt(contentH*34/100, 6, 14)
	midH := contentH - topH - bottomH
	if midH < 4 {
		midH = 4
	}

	topRow := TwoCol(
		PanelH("Workload Capacity", half, topH, workloadCapacityBody(st, st.Snapshot, m, half)),
		PanelH("Node Footprint", half, topH, nodeFootprintBody(b, m, half)),
	)
	midRow := TwoCol(
		PanelH("Investigation Pods", half, midH, investigationPodsBody(b, m, half, midH)),
		PanelH("Co-located on Node", half, midH, colocatedPodsBody(b, m, half, midH)),
	)
	bottom := PanelH("Resource Investigation", width, bottomH, resourceAnalysisBody(st, b, m, width, bottomH))

	return topRow + "\n" + midRow + "\n" + bottom
}

// ── Resource Health ──────────────────────────────────────────────────────────

func resourceHealthBody(st model.InvestigationState, b model.EvidenceBundle, m model.MetricsSummary, half int) string {
	cw := half - 4
	if cw < 16 {
		cw = 16
	}
	var out []string

	cpuDenom := maxI64(m.CPULimitM, m.CPURequestM)
	if cpuDenom > 0 || m.CPURequestM > 0 {
		out = append(out, headStyle.Render("CPU"))
		if m.Available && m.CPUUsageM > 0 {
			pct := int(frac(m.CPUUsageM, cpuDenom) * 100)
			out = append(out, fmt.Sprintf("  %s / %s", cpuStr(m.CPUUsageM), cpuStr(cpuDenom)))
			out = append(out, "  "+utilBar(frac(m.CPUUsageM, cpuDenom), 12)+" "+utilPctLabel(pct, cpuDenom))
		} else if m.CPURequestM > 0 {
			out = append(out, "  "+dimStyle.Render(fmt.Sprintf("request %s · limit %s", cpuStr(m.CPURequestM), cpuStr(m.CPULimitM))))
		} else {
			out = append(out, "  "+dimStyle.Render("no cpu data"))
		}
		if sp := metricSparkline(st, "cpu", 12); sp != "" {
			out = append(out, "  trend "+sp)
		}
	}

	memDenom := maxI64(m.MemLimitMi, m.MemRequestMi)
	if memDenom > 0 || m.MemRequestMi > 0 {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, headStyle.Render("Memory"))
		if m.Available && m.MemUsageMi > 0 {
			pct := int(frac(m.MemUsageMi, memDenom) * 100)
			out = append(out, fmt.Sprintf("  %s / %s", memStr(m.MemUsageMi), memStr(memDenom)))
			line := "  " + utilBar(frac(m.MemUsageMi, memDenom), 12) + " " + utilPctLabel(pct, memDenom)
			if pct >= 80 {
				line += " " + warnStyle.Render("⚠ High")
			}
			out = append(out, line)
		} else if m.MemLimitMi > 0 || m.MemRequestMi > 0 {
			line := fmt.Sprintf("request %s · limit %s", memStr(m.MemRequestMi), memStr(m.MemLimitMi))
			if perPod := typicalPodMemLimitMi(b); perPod > 0 && len(b.Pods) > 1 {
				line = fmt.Sprintf("%s · %s per pod", line, memStr(perPod))
			}
			out = append(out, "  "+dimStyle.Render(line))
		}
		if sp := metricSparkline(st, "mem", 12); sp != "" {
			out = append(out, "  trend "+sp)
		}
	}

	if m.CPURequestM > 0 || m.MemRequestMi > 0 {
		out = append(out, "")
		out = append(out, headStyle.Render("Requests"))
		if m.CPURequestM > 0 {
			out = append(out, "  "+kv("CPU", cpuStr(m.CPURequestM)))
		}
		if m.MemRequestMi > 0 {
			out = append(out, "  "+kv("Memory", memStr(m.MemRequestMi)))
		}
	}
	if m.CPULimitM > 0 || m.MemLimitMi > 0 {
		out = append(out, headStyle.Render("Limits"))
		if m.CPULimitM > 0 {
			out = append(out, "  "+kv("CPU", cpuStr(m.CPULimitM)))
		}
		if m.MemLimitMi > 0 {
			out = append(out, "  "+kv("Memory", memStr(m.MemLimitMi)))
		}
	}

	out = append(out, "")
	if m.Available {
		out = append(out, kv("Metrics API", okStyle.Render("✓ Available")))
	} else {
		out = append(out, kv("Metrics API", warnStyle.Render("✗ Unavailable")))
		if oom := countPodsOOMKilled(b); oom > 0 {
			out = append(out, dimStyle.Render(fmt.Sprintf("  %d pod(s) OOMKilled — limits from spec, usage unknown", oom)))
		} else {
			out = append(out, dimStyle.Render("  showing requests/limits from pod spec only"))
		}
		if m.Note != "" {
			out = append(out, dimStyle.Render("  "+truncVisual(m.Note, cw)))
		}
	}
	return strings.Join(out, "\n")
}

// ── Resource Pressure ────────────────────────────────────────────────────────

func resourcePressureBody(b model.EvidenceBundle, m model.MetricsSummary) string {
	total, ready, notReady, restarting, oom := pressureCounts(b)

	cpuPress := "No"
	memPress := "No"
	nodePress := "No"

	if m.Available && m.CPULimitM > 0 && frac(m.CPUUsageM, m.CPULimitM) >= 0.8 {
		cpuPress = highStyle.Render("Yes")
	}
	if memoryPressureDetected(b, m) {
		memPress = warnStyle.Render("Yes")
	}
	if nodePressureDetected(b.Nodes) {
		nodePress = warnStyle.Render("Yes")
	}

	var out []string
	out = append(out, headStyle.Render("Current Pressure"))
	out = append(out, "")
	out = append(out, kv("Pods", fmt.Sprintf("%d", total)))
	out = append(out, kv("Ready", fmt.Sprintf("%d", ready)))
	if notReady > 0 {
		out = append(out, kv("Not Ready", critStyle.Render(fmt.Sprintf("%d", notReady))))
	} else {
		out = append(out, kv("Not Ready", fmt.Sprintf("%d", notReady)))
	}
	if restarting > 0 {
		out = append(out, kv("Restarting", warnStyle.Render(fmt.Sprintf("%d", restarting))))
	} else {
		out = append(out, kv("Restarting", fmt.Sprintf("%d", restarting)))
	}
	if oom > 0 {
		out = append(out, kv("OOMKilled", critStyle.Render(fmt.Sprintf("%d", oom))))
	} else {
		out = append(out, kv("OOMKilled", fmt.Sprintf("%d", oom)))
	}
	out = append(out, "")
	out = append(out, kv("CPU Pressure", cpuPress))
	out = append(out, kv("Memory Pressure", memPress))
	out = append(out, kv("Node Pressure", nodePress))
	return strings.Join(out, "\n")
}

func pressureCounts(b model.EvidenceBundle) (total, ready, notReady, restarting, oom int) {
	for _, p := range b.Pods {
		total++
		if p.Ready {
			ready++
		} else {
			notReady++
		}
		if podRestarting(p) {
			restarting++
		}
		if podOOMKilled(p) {
			oom++
		}
	}
	return
}

func podRestarting(p model.PodSummary) bool {
	if p.RestartCount > 0 && !p.Ready {
		return true
	}
	for _, c := range p.Containers {
		if c.State == "waiting" || c.Reason == "CrashLoopBackOff" || c.Reason == "BackOff" {
			return true
		}
	}
	return false
}

func podOOMKilled(p model.PodSummary) bool {
	for _, c := range p.Containers {
		if strings.EqualFold(c.LastReason, "OOMKilled") || strings.EqualFold(c.Reason, "OOMKilled") {
			return true
		}
	}
	return false
}

func memoryPressureDetected(b model.EvidenceBundle, m model.MetricsSummary) bool {
	if m.Available && m.MemLimitMi > 0 && frac(m.MemUsageMi, m.MemLimitMi) >= 0.75 {
		return true
	}
	if m.Available && m.MemRequestMi > 0 && m.MemUsageMi > m.MemRequestMi {
		return true
	}
	for _, p := range b.Pods {
		if podOOMKilled(p) {
			return true
		}
	}
	for _, n := range b.Nodes {
		if n.MemoryPressure {
			return true
		}
	}
	return false
}

func nodePressureDetected(nodes []model.NodeSummary) bool {
	for _, n := range nodes {
		if !n.Ready || n.MemoryPressure || n.DiskPressure || n.PIDPressure {
			return true
		}
	}
	return false
}

// ── Pod Resource Ranking ───────────────────────────────────────────────────

type podResourceRow struct {
	name         string
	cpuM         int64
	memMi        int64
	memLimitMi   int64
	cpuLimitM    int64
	oomKilled    bool
	restarts     int32
	severity     model.Severity
	sortScore    int
}

func podRankingBody(b model.EvidenceBundle, m model.MetricsSummary, half, rows int) string {
	ranked := rankPodsByResource(b, m)
	if len(ranked) == 0 {
		return dimStyle.Render("  no pods in scope")
	}

	nameW := clampInt(half-28, 10, 18)
	header := fmt.Sprintf("  %s %s %s %s",
		padRight("Pod", nameW), padRight("CPU", 7), padRight("Memory", 8), "Restarts")
	lines := []string{dimStyle.Render(header)}

	maxShow := rows - 1
	if maxShow < 1 {
		maxShow = 1
	}
	for i, r := range ranked {
		if i >= maxShow {
			break
		}
		cpuBar := utilBar(frac(r.cpuM, maxI64(m.CPULimitM, m.CPURequestM)), 6)
		memBar := utilBar(frac(r.memMi, maxI64(m.MemLimitMi, m.MemRequestMi)), 6)
		icon := ""
		switch r.severity {
		case model.SeverityCritical:
			icon = " 🔴"
		case model.SeverityHigh:
			icon = " 🟠"
		case model.SeverityWarning:
			icon = " 🟡"
		}
		cpu := dimStyle.Render("—")
		mem := dimStyle.Render("—")
		if r.oomKilled && r.memLimitMi > 0 {
			mem = critStyle.Render(memStr(r.memLimitMi) + " at limit")
		} else if r.memMi > 0 {
			mem = memStr(r.memMi)
			if r.severity >= model.SeverityHigh {
				mem = highStyle.Render(mem)
			}
		} else if r.memLimitMi > 0 {
			mem = dimStyle.Render(memStr(r.memLimitMi) + " limit")
		}
		if r.cpuM > 0 {
			cpu = cpuStr(r.cpuM)
			if r.severity >= model.SeverityHigh {
				cpu = highStyle.Render(cpu)
			}
		} else if r.cpuLimitM > 0 && !m.Available {
			cpu = dimStyle.Render(cpuStr(r.cpuLimitM) + " limit")
		}
		rest := fmt.Sprintf("%d", r.restarts)
		if r.restarts >= 3 {
			rest = critStyle.Render(rest)
		} else if r.restarts > 0 {
			rest = warnStyle.Render(rest)
		}
		lines = append(lines, fmt.Sprintf("  %s %s %s %s%s",
			padRight(truncVisual(r.name, nameW), nameW),
			padRight(cpu, 7), padRight(mem, 8), rest, icon))
		if r.cpuM > 0 || r.memMi > 0 {
			lines = append(lines, fmt.Sprintf("      %s cpu  %s mem", cpuBar, memBar))
		}
	}
	return strings.Join(lines, "\n")
}

func rankPodsByResource(b model.EvidenceBundle, m model.MetricsSummary) []podResourceRow {
	var rows []podResourceRow
	for _, p := range b.Pods {
		cpu, mem := estimatePodUsage(p, b.Pods, m)
		sev, score := podResourceSeverity(p, cpu, mem, m)
		podLimCPU, podLimMem := podResourceLimits(p)
		rows = append(rows, podResourceRow{
			name: p.Name, cpuM: cpu, memMi: mem, memLimitMi: podLimMem, cpuLimitM: podLimCPU,
			oomKilled: podOOMKilled(p), restarts: p.RestartCount,
			severity: sev, sortScore: score,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].sortScore != rows[j].sortScore {
			return rows[i].sortScore > rows[j].sortScore
		}
		if rows[i].memMi != rows[j].memMi {
			return rows[i].memMi > rows[j].memMi
		}
		if rows[i].cpuM != rows[j].cpuM {
			return rows[i].cpuM > rows[j].cpuM
		}
		return rows[i].name < rows[j].name
	})
	return rows
}

func podResourceLimits(p model.PodSummary) (cpuM, memMi int64) {
	for _, c := range p.Containers {
		limCPU := parseCPU(c.LimitsCPU)
		limMem := parseMem(c.LimitsMem)
		if limCPU == 0 {
			limCPU = parseCPU(c.RequestsCPU)
		}
		if limMem == 0 {
			limMem = parseMem(c.RequestsMem)
		}
		cpuM += limCPU
		memMi += limMem
	}
	return cpuM, memMi
}

func typicalPodMemLimitMi(b model.EvidenceBundle) int64 {
	for _, p := range b.Pods {
		_, lim := podResourceLimits(p)
		if lim > 0 {
			return lim
		}
	}
	return 0
}

func countPodsOOMKilled(b model.EvidenceBundle) int {
	n := 0
	for _, p := range b.Pods {
		if podOOMKilled(p) {
			n++
		}
	}
	return n
}

func estimatePodUsage(p model.PodSummary, pods []model.PodSummary, m model.MetricsSummary) (cpuM, memMi int64) {
	var podLimitCPU, podLimitMem int64
	for _, c := range p.Containers {
		podLimitCPU += parseCPU(c.LimitsCPU)
		podLimitMem += parseMem(c.LimitsMem)
		if podLimitCPU == 0 {
			podLimitCPU += parseCPU(c.RequestsCPU)
		}
		if podLimitMem == 0 {
			podLimitMem += parseMem(c.RequestsMem)
		}
	}

	if !m.Available {
		return 0, 0
	}

	var totalLimitCPU, totalLimitMem int64
	for _, pod := range pods {
		for _, c := range pod.Containers {
			limCPU := parseCPU(c.LimitsCPU)
			limMem := parseMem(c.LimitsMem)
			if limCPU == 0 {
				limCPU = parseCPU(c.RequestsCPU)
			}
			if limMem == 0 {
				limMem = parseMem(c.RequestsMem)
			}
			totalLimitCPU += limCPU
			totalLimitMem += limMem
		}
	}
	if totalLimitMem > 0 && podLimitMem > 0 {
		memMi = m.MemUsageMi * podLimitMem / totalLimitMem
	}
	if totalLimitCPU > 0 && podLimitCPU > 0 {
		cpuM = m.CPUUsageM * podLimitCPU / totalLimitCPU
	}

	// pods that OOM'd or restart heavily likely consumed at/near limit
	if podOOMKilled(p) && podLimitMem > 0 && memMi < podLimitMem {
		memMi = podLimitMem
	}
	if p.RestartCount >= 2 && podLimitMem > 0 && memMi < podLimitMem*9/10 {
		memMi = podLimitMem * 9 / 10
	}
	return cpuM, memMi
}

func podResourceSeverity(p model.PodSummary, cpuM, memMi int64, m model.MetricsSummary) (model.Severity, int) {
	score := 0
	if podOOMKilled(p) {
		score += 100
		return model.SeverityCritical, score + int(p.RestartCount)*5
	}
	if !p.Ready {
		score += 30
	}
	score += int(p.RestartCount) * 8
	for _, c := range p.Containers {
		limMem := parseMem(c.LimitsMem)
		if limMem > 0 && memMi >= limMem {
			score += 40
			return model.SeverityCritical, score
		}
		if limMem > 0 && memMi >= limMem*8/10 {
			score += 25
		}
	}
	if m.Available && m.CPULimitM > 0 && cpuM > 0 && cpuM >= m.CPULimitM/4 {
		score += 15
	}
	switch {
	case score >= 80:
		return model.SeverityCritical, score
	case score >= 40:
		return model.SeverityHigh, score
	case score >= 15:
		return model.SeverityWarning, score
	default:
		return model.SeverityInfo, score
	}
}

// ── Node Summary ─────────────────────────────────────────────────────────────

type nodeResourceRow struct {
	name   string
	cpuPct int
	memPct int
	pods   int
	status string
	score  int
}

func nodeSummaryBody(b model.EvidenceBundle, m model.MetricsSummary, half, rows int) string {
	nodes := rankNodes(b, m)
	if len(nodes) == 0 {
		return dimStyle.Render("  no node data (nodes get permission?)")
	}

	var lines []string
	maxShow := rows / 5
	if maxShow < 1 {
		maxShow = 1
	}
	if maxShow > len(nodes) {
		maxShow = len(nodes)
	}
	for i, n := range nodes {
		if i >= maxShow {
			break
		}
		if i > 0 {
			lines = append(lines, dimStyle.Render("  "+strings.Repeat("─", clampInt(half-6, 8, 30))))
		}
		lines = append(lines, headStyle.Render("  "+n.name))
		cpuLine := fmt.Sprintf("%d%%", n.cpuPct)
		memLine := fmt.Sprintf("%d%%", n.memPct)
		if n.cpuPct >= 80 {
			cpuLine = highStyle.Render(cpuLine)
		} else if n.cpuPct >= 50 {
			cpuLine = warnStyle.Render(cpuLine)
		} else {
			cpuLine = dimStyle.Render(cpuLine)
		}
		if n.memPct >= 80 {
			memLine = critStyle.Render(memLine)
		} else if n.memPct >= 50 {
			memLine = warnStyle.Render(memLine)
		} else {
			memLine = dimStyle.Render(memLine)
		}
		lines = append(lines, "  "+kv("CPU", cpuLine))
		lines = append(lines, "  "+kv("Memory", memLine))
		lines = append(lines, "  "+kv("Pods", fmt.Sprintf("%d", n.pods)))
		status := okStyle.Render("Ready")
		if n.status != "Ready" {
			status = warnStyle.Render(n.status)
		}
		lines = append(lines, "  "+kv("Status", status))
	}
	return strings.Join(lines, "\n")
}

func rankNodes(b model.EvidenceBundle, m model.MetricsSummary) []nodeResourceRow {
	podByNode := map[string][]model.PodSummary{}
	for _, p := range b.Pods {
		if p.Node != "" {
			podByNode[p.Node] = append(podByNode[p.Node], p)
		}
	}

	var rows []nodeResourceRow
	seen := map[string]bool{}
	for _, n := range b.Nodes {
		seen[n.Name] = true
		rows = append(rows, buildNodeRow(n, podByNode[n.Name]))
	}
	for nodeName, pods := range podByNode {
		if seen[nodeName] {
			continue
		}
		rows = append(rows, buildNodeRow(model.NodeSummary{Name: nodeName, Ready: true}, pods))
	}

	if m.Available {
		for i := range rows {
			rows[i].cpuPct, rows[i].memPct = nodeUtilEstimate(rows[i].name, podByNode[rows[i].name], b.Pods, m)
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].memPct > rows[j].memPct
	})
	return rows
}

func buildNodeRow(n model.NodeSummary, pods []model.PodSummary) nodeResourceRow {
	status := "Ready"
	score := 0
	if !n.Ready {
		status = "NotReady"
		score += 50
	}
	if n.MemoryPressure {
		status = "MemoryPressure"
		score += 80
	}
	if n.DiskPressure {
		score += 40
	}
	if n.PIDPressure {
		score += 30
	}
	for _, p := range pods {
		if podOOMKilled(p) {
			score += 20
		}
		if !p.Ready {
			score += 5
		}
	}
	return nodeResourceRow{name: n.Name, pods: len(pods), status: status, score: score}
}

func nodeUtilEstimate(nodeName string, nodePods, allPods []model.PodSummary, m model.MetricsSummary) (cpuPct, memPct int) {
	if len(nodePods) == 0 {
		return 0, 0
	}
	var nodeLimitCPU, nodeLimitMem, totalLimitCPU, totalLimitMem int64
	for _, p := range nodePods {
		for _, c := range p.Containers {
			limCPU := parseCPU(c.LimitsCPU)
			limMem := parseMem(c.LimitsMem)
			if limCPU == 0 {
				limCPU = parseCPU(c.RequestsCPU)
			}
			if limMem == 0 {
				limMem = parseMem(c.RequestsMem)
			}
			nodeLimitCPU += limCPU
			nodeLimitMem += limMem
		}
	}
	for _, p := range allPods {
		for _, c := range p.Containers {
			limCPU := parseCPU(c.LimitsCPU)
			limMem := parseMem(c.LimitsMem)
			if limCPU == 0 {
				limCPU = parseCPU(c.RequestsCPU)
			}
			if limMem == 0 {
				limMem = parseMem(c.RequestsMem)
			}
			totalLimitCPU += limCPU
			totalLimitMem += limMem
		}
	}
	if totalLimitMem > 0 && nodeLimitMem > 0 {
		nodeMem := m.MemUsageMi * nodeLimitMem / totalLimitMem
		if nodeLimitMem > 0 {
			memPct = int(frac(nodeMem, nodeLimitMem) * 100)
		}
	}
	if totalLimitCPU > 0 && nodeLimitCPU > 0 {
		nodeCPU := m.CPUUsageM * nodeLimitCPU / totalLimitCPU
		if nodeLimitCPU > 0 {
			cpuPct = int(frac(nodeCPU, nodeLimitCPU) * 100)
		}
	}
	if memPct > 100 {
		memPct = 100
	}
	if cpuPct > 100 {
		cpuPct = 100
	}
	return cpuPct, memPct
}

// ── Findings / Timeline / Recommendations ────────────────────────────────────

type resourceFinding struct {
	text     string
	positive bool // true = ✓ healthy, false = ⚠ issue
}

func resourceAnalysisBody(st model.InvestigationState, b model.EvidenceBundle, m model.MetricsSummary, width, rows int) string {
	findings := generateResourceFindings(st, b, m)
	recs := generateResourceRecommendations(st, b, m, findings)
	timeline := buildResourceTimeline(st, b, m)

	findLines := formatFindings(findings, rows-2)
	recLines := formatRecommendations(recs, rows-2)
	tlLines := formatResourceTimeline(timeline, rows-2)

	if m.Available || len(timeline) > 0 {
		colW := (width - 6) / 3
		if colW < 24 {
			colW = 24
		}
		return joinAnalysisColumns(
			headStyle.Render("Findings")+"\n"+findLines,
			headStyle.Render("Timeline")+"\n"+tlLines,
			headStyle.Render("Recommendations")+"\n"+recLines,
			colW, rows,
		)
	}
	half := (width - 6) / 2
	if half < 28 {
		half = 28
	}
	return TwoCol(
		headStyle.Render("Findings")+"\n"+findLines,
		headStyle.Render("Recommendations")+"\n"+recLines,
	)
}

func joinAnalysisColumns(a, b, c string, colW, rows int) string {
	padCol := func(body string) string {
		lines := strings.Split(body, "\n")
		for len(lines) < rows {
			lines = append(lines, "")
		}
		if len(lines) > rows {
			lines = lines[:rows]
		}
		var out []string
		for _, ln := range lines {
			out = append(out, padRight(ln, colW))
		}
		return strings.Join(out, "\n")
	}
	left := padCol(a)
	mid := padCol(b)
	right := padCol(c)
	var merged []string
	for i, ln := range strings.Split(left, "\n") {
		midLn, rightLn := "", ""
		midLines := strings.Split(mid, "\n")
		rightLines := strings.Split(right, "\n")
		if i < len(midLines) {
			midLn = midLines[i]
		}
		if i < len(rightLines) {
			rightLn = rightLines[i]
		}
		merged = append(merged, ln+"  "+midLn+"  "+rightLn)
	}
	return strings.Join(merged, "\n")
}

func formatFindings(findings []resourceFinding, maxLines int) string {
	if len(findings) == 0 {
		return dimStyle.Render("  no resource evidence yet")
	}
	var lines []string
	for _, f := range findings {
		prefix := okStyle.Render("✓")
		line := f.text
		if !f.positive {
			prefix = warnStyle.Render("⚠")
			line = warnStyle.Render(f.text)
		}
		lines = append(lines, "  "+prefix+" "+line)
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func formatRecommendations(recs []string, maxLines int) string {
	if len(recs) == 0 {
		return dimStyle.Render("  no recommendations — insufficient evidence")
	}
	var lines []string
	for _, r := range recs {
		lines = append(lines, "  "+dimStyle.Render("→ ")+r)
	}
	if maxLines > 0 && len(recs) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func formatResourceTimeline(snaps []resourceTimelineStep, maxLines int) string {
	if len(snaps) == 0 {
		return dimStyle.Render("  awaiting metric samples")
	}
	var lines []string
	for i, s := range snaps {
		if i > 0 {
			lines = append(lines, dimStyle.Render("  ↓"))
		}
		lines = append(lines, markStyle.Render("  "+s.timeLabel))
		for _, detail := range s.details {
			lines = append(lines, "  "+detail)
		}
		if s.event != "" {
			lines = append(lines, critStyle.Render("  "+s.event))
		}
	}
	if maxLines > 0 {
		lines = truncateLines(lines, maxLines)
	}
	return strings.Join(lines, "\n")
}

func truncateLines(lines []string, max int) []string {
	if len(lines) <= max {
		return lines
	}
	return lines[:max]
}

type resourceTimelineStep struct {
	timeLabel string
	details   []string
	event     string
}

func generateResourceFindings(st model.InvestigationState, b model.EvidenceBundle, m model.MetricsSummary) []resourceFinding {
	var out []resourceFinding

	// CPU stability
	if m.Available && m.CPULimitM > 0 {
		cpuFrac := frac(m.CPUUsageM, m.CPULimitM)
		if cpuFrac < 0.7 {
			out = append(out, resourceFinding{text: "CPU remained stable", positive: true})
		} else if cpuFrac >= 0.9 {
			out = append(out, resourceFinding{text: "CPU usage near limit", positive: false})
		}
	}

	// Memory vs request/limit
	if m.Available && m.MemRequestMi > 0 && m.MemUsageMi > m.MemRequestMi {
		out = append(out, resourceFinding{text: "Memory usage exceeded configured request", positive: false})
	}
	if m.Available && m.MemLimitMi > 0 && m.MemUsageMi >= m.MemLimitMi*9/10 {
		out = append(out, resourceFinding{text: "Aggregate memory near workload limit", positive: false})
	}

	// OOM correlation
	oomPods := 0
	for _, p := range b.Pods {
		if podOOMKilled(p) {
			oomPods++
		}
	}
	if oomPods > 0 {
		out = append(out, resourceFinding{text: "Container exceeded memory limit", positive: false})
		if memorySpikeBeforeOOM(st) {
			out = append(out, resourceFinding{text: "Memory spike preceded OOMKilled", positive: false})
		}
	}

	// Restart trend
	maxRestarts, podsWithRestarts := 0, 0
	for _, p := range b.Pods {
		if p.RestartCount > 0 {
			podsWithRestarts++
		}
		if int(p.RestartCount) > maxRestarts {
			maxRestarts = int(p.RestartCount)
		}
	}
	if podsWithRestarts >= 2 && maxRestarts >= 2 {
		out = append(out, resourceFinding{text: "Restart frequency increasing", positive: false})
	} else if maxRestarts == 0 {
		out = append(out, resourceFinding{text: "No pod restarts observed", positive: true})
	}

	// Node health
	if len(b.Nodes) > 0 {
		if nodePressureDetected(b.Nodes) {
			out = append(out, resourceFinding{text: "Node pressure detected", positive: false})
		} else {
			out = append(out, resourceFinding{text: "Node capacity healthy", positive: true})
		}
	}

	// Request configuration
	if m.MemRequestMi > 0 && m.MemLimitMi > 0 {
		if m.MemRequestMi*2 < m.MemLimitMi && oomPods == 0 {
			out = append(out, resourceFinding{text: "Requests appear correctly configured", positive: true})
		} else if m.MemRequestMi >= m.MemLimitMi && oomPods > 0 {
			out = append(out, resourceFinding{text: "Memory request equals limit — no burst headroom", positive: false})
		}
	}

	// HPA at max with resource pressure
	for _, h := range b.HPAs {
		if h.AtMax && memoryPressureDetected(b, m) {
			out = append(out, resourceFinding{text: "HPA at max replicas under memory pressure", positive: false})
		}
	}

	// Recent deployment + resource symptoms
	if st.RecentChange != nil && (st.RecentChange.RevisionTo != "" || st.RecentChange.Image != "") {
		if oomPods > 0 || memoryPressureDetected(b, m) {
			out = append(out, resourceFinding{text: "Resource symptoms follow recent deployment", positive: false})
		}
	}

	if len(out) == 0 {
		if !m.Available {
			out = append(out, resourceFinding{text: "Live metrics unavailable — limits from pod spec", positive: false})
		}
	}
	return out
}

func generateResourceRecommendations(st model.InvestigationState, b model.EvidenceBundle, m model.MetricsSummary, findings []resourceFinding) []string {
	var recs []string
	add := func(s string) {
		for _, r := range recs {
			if r == s {
				return
			}
		}
		recs = append(recs, s)
	}

	hasFinding := func(substr string) bool {
		for _, f := range findings {
			if strings.Contains(strings.ToLower(f.text), strings.ToLower(substr)) {
				return true
			}
		}
		return false
	}

	if hasFinding("memory limit") || hasFinding("oom") || podOOMCount(b) > 0 {
		add("Investigate memory leak or sudden allocation spike")
		if m.Available && m.MemLimitMi > 0 && m.MemUsageMi >= m.MemLimitMi*9/10 {
			add("Increase memory limit")
		}
	}
	if hasFinding("exceeded configured request") || (m.MemRequestMi > 0 && m.MemUsageMi > m.MemRequestMi*2) {
		add("Reduce memory request imbalance")
	}
	if hasFinding("recent deployment") || st.RecentChange != nil {
		add("Review recent deployment")
	}
	for _, h := range b.HPAs {
		if h.AtMax && memoryPressureDetected(b, m) {
			add("Check HPA configuration")
		}
	}
	if hasFinding("cpu usage near limit") {
		add("Review CPU limits and throttling")
	}
	if hasFinding("node pressure") {
		add("Inspect node capacity and eviction thresholds")
	}
	if !m.Available {
		add("Enable metrics-server for live usage data")
	}

	return recs
}

func podOOMCount(b model.EvidenceBundle) int {
	n := 0
	for _, p := range b.Pods {
		if podOOMKilled(p) {
			n++
		}
	}
	return n
}

func memorySpikeBeforeOOM(st model.InvestigationState) bool {
	var metrics []metricSample
	for _, e := range st.LiveEvidence {
		if e.SourceType == model.SourceMetric {
			if cpu, mem := parseMetricMessage(e.Message); cpu > 0 || mem > 0 {
				metrics = append(metrics, metricSample{ts: e.Timestamp, memMi: mem, cpuM: cpu})
			}
		}
	}
	for _, e := range st.Timeline {
		if e.Type == "metric" {
			if cpu, mem := parseMetricMessage(e.Message); cpu > 0 || mem > 0 {
				metrics = append(metrics, metricSample{ts: e.Timestamp, memMi: mem, cpuM: cpu})
			}
		}
	}
	if len(metrics) < 2 {
		// infer from aggregate if OOM events exist after increasing usage in snapshot
		for _, e := range st.Timeline {
			if e.Reason == "OOMKilled" {
				return st.Snapshot.Metrics.MemUsageMi > st.Snapshot.Metrics.MemRequestMi
			}
		}
		return false
	}
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].ts.Before(metrics[j].ts) })
	first, last := metrics[0], metrics[len(metrics)-1]
	return last.memMi > first.memMi*2 && last.memMi > 0
}

type metricSample struct {
	ts    time.Time
	cpuM  int64
	memMi int64
}

var metricCPURe = regexp.MustCompile(`(?i)cpu\s+(\d+(?:\.\d+)?)\s*m`)
var metricMemMiRe = regexp.MustCompile(`(?i)memory\s*(\d+(?:\.\d+)?)\s*mi`)
var metricMemGiRe = regexp.MustCompile(`(?i)memory\s*(\d+(?:\.\d+)?)\s*gi`)

func parseMetricMessage(msg string) (cpuM, memMi int64) {
	if m := metricCPURe.FindStringSubmatch(msg); len(m) > 1 {
		f, _ := strconv.ParseFloat(m[1], 64)
		cpuM = int64(f)
	}
	if m := metricMemMiRe.FindStringSubmatch(msg); len(m) > 1 {
		f, _ := strconv.ParseFloat(m[1], 64)
		memMi = int64(f)
	}
	if m := metricMemGiRe.FindStringSubmatch(msg); len(m) > 1 {
		f, _ := strconv.ParseFloat(m[1], 64)
		memMi = int64(f * 1024)
	}
	if cpuM == 0 {
		if i := strings.Index(strings.ToLower(msg), "cpu"); i >= 0 {
			sub := msg[i:]
			if m := metricCPURe.FindStringSubmatch(sub); len(m) > 1 {
				f, _ := strconv.ParseFloat(m[1], 64)
				cpuM = int64(f)
			}
		}
	}
	return cpuM, memMi
}

func buildResourceTimeline(st model.InvestigationState, b model.EvidenceBundle, m model.MetricsSummary) []resourceTimelineStep {
	if !m.Available && len(collectMetricSamples(st)) == 0 {
		return nil
	}

	var steps []resourceTimelineStep
	for _, s := range collectMetricSamples(st) {
		var details []string
		if s.cpuM > 0 {
			details = append(details, "CPU "+cpuStr(s.cpuM))
		}
		if s.memMi > 0 {
			details = append(details, "Memory "+memStr(s.memMi))
		}
		if len(details) == 0 {
			continue
		}
		steps = append(steps, resourceTimelineStep{
			timeLabel: s.ts.Format("15:04"),
			details:   details,
		})
	}

	// fold OOM and restart milestones from timeline
	for _, e := range st.Timeline {
		switch e.Reason {
		case "OOMKilled", "OOMKilling":
			steps = append(steps, resourceTimelineStep{
				timeLabel: e.Timestamp.Format("15:04"),
				event:     "OOMKilled",
			})
		case "BackOff", "CrashLoopBackOff":
			if strings.Contains(strings.ToLower(e.Message), "restart") {
				steps = append(steps, resourceTimelineStep{
					timeLabel: e.Timestamp.Format("15:04"),
					event:     "Restart",
				})
			}
		}
	}

	sort.SliceStable(steps, func(i, j int) bool {
		return steps[i].timeLabel < steps[j].timeLabel
	})

	// dedupe consecutive identical events
	var deduped []resourceTimelineStep
	for _, s := range steps {
		if len(deduped) > 0 {
			last := deduped[len(deduped)-1]
			if last.timeLabel == s.timeLabel && last.event == s.event && len(last.details) == 0 && len(s.details) == 0 {
				continue
			}
		}
		deduped = append(deduped, s)
	}
	return deduped
}

func collectMetricSamples(st model.InvestigationState) []metricSample {
	var out []metricSample
	for _, e := range st.LiveEvidence {
		if e.SourceType != model.SourceMetric {
			continue
		}
		cpu, mem := parseMetricMessage(e.Message)
		if cpu == 0 && mem == 0 {
			continue
		}
		out = append(out, metricSample{ts: e.Timestamp, cpuM: cpu, memMi: mem})
	}
	for _, e := range st.Timeline {
		if e.Type != "metric" {
			continue
		}
		cpu, mem := parseMetricMessage(e.Message)
		if cpu == 0 && mem == 0 {
			continue
		}
		out = append(out, metricSample{ts: e.Timestamp, cpuM: cpu, memMi: mem})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ts.Before(out[j].ts) })
	return out
}

// ── sparklines ───────────────────────────────────────────────────────────────

func metricSparkline(st model.InvestigationState, kind string, width int) string {
	samples := collectMetricSamples(st)
	if len(samples) < 2 {
		return ""
	}
	vals := make([]int64, 0, len(samples))
	for _, s := range samples {
		switch kind {
		case "cpu":
			if s.cpuM > 0 {
				vals = append(vals, s.cpuM)
			}
		case "mem":
			if s.memMi > 0 {
				vals = append(vals, s.memMi)
			}
		}
	}
	if len(vals) < 2 {
		return ""
	}
	return renderSparkline(vals, width)
}

func renderSparkline(vals []int64, width int) string {
	if width < 4 {
		width = 4
	}
	if len(vals) > width {
		vals = vals[len(vals)-width:]
	}
	minV, maxV := vals[0], vals[0]
	for _, v := range vals {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	span := maxV - minV
	if span <= 0 {
		span = 1
	}
	var b strings.Builder
	for _, v := range vals {
		frac := float64(v-minV) / float64(span)
		ch := "▁▂▃▄▅▆▇█"[clampInt(int(frac*7), 0, 7)]
		col := okStyle
		switch {
		case frac >= 0.85:
			col = critStyle
		case frac >= 0.6:
			col = highStyle
		case frac >= 0.35:
			col = warnStyle
		}
		b.WriteString(col.Render(string(ch)))
	}
	return b.String()
}

// ── shared helpers ───────────────────────────────────────────────────────────

func resolveMetrics(b model.EvidenceBundle) model.MetricsSummary {
	m := b.Metrics
	if m.CPURequestM == 0 && m.MemLimitMi == 0 {
		m = deriveMetrics(b)
	}
	return m
}

func utilBar(f float64, width int) string {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	n := int(f * float64(width))
	col := okStyle
	switch {
	case f >= 0.9:
		col = critStyle
	case f >= 0.75:
		col = highStyle
	case f >= 0.5:
		col = warnStyle
	default:
		col = okStyle
	}
	return col.Render(strings.Repeat("█", n)) + renderBarEmpty(f, width, n)
}

func renderBarEmpty(f float64, width, filled int) string {
	if filled >= width {
		return ""
	}
	return render.BarEmpty.Render(strings.Repeat("░", width-filled))
}

func utilPctLabel(pct int, denom int64) string {
	if denom <= 0 {
		return dimStyle.Render("--")
	}
	s := fmt.Sprintf("%d%%", pct)
	if pct >= 90 {
		return critStyle.Render(s)
	}
	if pct >= 75 {
		return highStyle.Render(s)
	}
	if pct >= 50 {
		return warnStyle.Render(s)
	}
	return dimStyle.Render(s)
}

func nodePressureHealth(n model.NodeSummary) string {
	if !n.Ready || n.MemoryPressure || n.DiskPressure {
		return "warning"
	}
	return "healthy"
}

func frac(a, b int64) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func deriveMetrics(b model.EvidenceBundle) model.MetricsSummary {
	var m model.MetricsSummary
	for _, p := range b.Pods {
		for _, c := range p.Containers {
			m.CPURequestM += parseCPU(c.RequestsCPU)
			m.CPULimitM += parseCPU(c.LimitsCPU)
			m.MemRequestMi += parseMem(c.RequestsMem)
			m.MemLimitMi += parseMem(c.LimitsMem)
		}
	}
	return m
}

func parseCPU(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.HasSuffix(s, "m") {
		n, _ := strconv.ParseInt(strings.TrimSuffix(s, "m"), 10, 64)
		return n
	}
	f, _ := strconv.ParseFloat(s, 64)
	return int64(f * 1000)
}

func parseMem(s string) int64 {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasSuffix(s, "Mi"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(s, "Mi"), 10, 64)
		return n
	case strings.HasSuffix(s, "Gi"):
		f, _ := strconv.ParseFloat(strings.TrimSuffix(s, "Gi"), 64)
		return int64(f * 1024)
	default:
		return 0
	}
}
