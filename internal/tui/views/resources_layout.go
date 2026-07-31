package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glnreddy421/klew/internal/model"
)

type resourceTotals struct {
	cpuReq, cpuLim, memReq, memLim int64
}

func aggregatePodResources(pods []model.PodSummary) resourceTotals {
	var t resourceTotals
	for _, p := range pods {
		for _, c := range p.Containers {
			reqCPU := parseCPU(c.RequestsCPU)
			limCPU := parseCPU(c.LimitsCPU)
			if limCPU == 0 {
				limCPU = reqCPU
			}
			if reqCPU == 0 {
				reqCPU = limCPU
			}
			reqMem := parseMem(c.RequestsMem)
			limMem := parseMem(c.LimitsMem)
			if limMem == 0 {
				limMem = reqMem
			}
			if reqMem == 0 {
				reqMem = limMem
			}
			t.cpuReq += reqCPU
			t.cpuLim += limCPU
			t.memReq += reqMem
			t.memLim += limMem
		}
	}
	return t
}

func workloadCapacityBody(st model.InvestigationState, b model.EvidenceBundle, m model.MetricsSummary, half int) string {
	cw := half - 4
	if cw < 16 {
		cw = 16
	}
	agg := aggregatePodResources(b.Pods)
	perPod := typicalPodMemLimitMi(b)
	var out []string

	title := fmt.Sprintf("Workload · %d pod(s)", len(b.Pods))
	if wl := primaryWorkloadName(b); wl != "" {
		title = fmt.Sprintf("%s · %d pod(s)", wl, len(b.Pods))
	}
	out = append(out, headStyle.Render(title))

	nodeFreeCPU, nodeFreeMem := nodeSchedulingHeadroom(b)

	out = append(out, capacityBlock("CPU", agg.cpuReq, agg.cpuLim, m.CPUUsageM, nodeFreeCPU, m.Available, metricSparkline(st, "cpu", 14))...)
	out = append(out, "")
	out = append(out, capacityBlock("Memory", agg.memReq, agg.memLim, m.MemUsageMi, nodeFreeMem, m.Available, metricSparkline(st, "mem", 14))...)

	if perPod > 0 && len(b.Pods) > 1 {
		out = append(out, dimStyle.Render(fmt.Sprintf("  per pod ≈ %s mem limit · %s mem req",
			memStr(perPod), memStr(agg.memReq/int64(len(b.Pods))))))
	}

	out = append(out, "")
	if m.Available {
		out = append(out, kv("Metrics", okStyle.Render("live usage from metrics-server")))
	} else {
		out = append(out, kv("Metrics", warnStyle.Render("usage unknown")))
		out = append(out, dimStyle.Render("  bars show request/limit vs node headroom"))
	}
	if m.Note != "" {
		out = append(out, dimStyle.Render("  "+truncVisual(m.Note, cw)))
	}
	return strings.Join(out, "\n")
}

func capacityBlock(name string, req, lim, usage, nodeFree int64, metricsOK bool, trend string) []string {
	var out []string
	out = append(out, headStyle.Render(name))

	reqBar := specShareBar(req, lim, 14, dimStyle)
	limBar := specShareBar(lim, lim, 14, okStyle)
	out = append(out, fmt.Sprintf("  %-8s %6s  %s req/limit", "request", formatResource(name, req), reqBar))
	out = append(out, fmt.Sprintf("  %-8s %6s  %s", "limit", formatResource(name, lim), limBar))

	if metricsOK && usage > 0 && lim > 0 {
		pct := int(frac(usage, lim) * 100)
		out = append(out, fmt.Sprintf("  %-8s %6s  %s %s",
			"usage", formatResource(name, usage), utilBar(frac(usage, lim), 14), utilPctLabel(pct, lim)))
		headroom := lim - usage
		if headroom < 0 {
			headroom = 0
		}
		out = append(out, fmt.Sprintf("  %-8s %6s  %s to limit",
			"headroom", formatResource(name, headroom), dimStyle.Render("limit − usage")))
	} else if lim > 0 {
		out = append(out, fmt.Sprintf("  %-8s %6s  %s",
			"usage", dimStyle.Render("—"), dimStyle.Render("enable metrics-server")))
		out = append(out, fmt.Sprintf("  %-8s %6s  %s",
			"headroom", formatResource(name, lim), dimStyle.Render("limit − usage (est.)")))
	}

	if nodeFree > 0 {
		out = append(out, fmt.Sprintf("  %-8s %6s  %s on node(s)",
			"node free", formatResource(name, nodeFree), specShareBar(req, nodeFree+req, 14, warnStyle)))
	}

	if trend != "" {
		out = append(out, fmt.Sprintf("  %-8s %s", "trend", trend))
	} else if metricsOK {
		out = append(out, fmt.Sprintf("  %-8s %s", "trend", dimStyle.Render("collecting samples…")))
	}
	return out
}

func formatResource(kind string, v int64) string {
	if kind == "CPU" {
		return cpuStr(v)
	}
	return memStr(v)
}

func specShareBar(part, total int64, width int, style lipgloss.Style) string {
	if total <= 0 {
		return style.Render(strings.Repeat("░", width))
	}
	f := frac(part, total)
	if f > 1 {
		f = 1
	}
	n := int(f * float64(width))
	if n > width {
		n = width
	}
	return style.Render(strings.Repeat("█", n)) + renderBarEmpty(f, width, n)
}

func nodeFootprintBody(b model.EvidenceBundle, m model.MetricsSummary, half int) string {
	if len(b.Nodes) == 0 {
		return dimStyle.Render("  no node data — need nodes/list permission")
	}

	var lines []string
	lines = append(lines, headStyle.Render("Node Footprint"))
	lines = append(lines, dimStyle.Render("  scoped vs co-located containers"))

	maxNodes := clampInt(half/8, 1, 3)
	for i, n := range b.Nodes {
		if i >= maxNodes {
			break
		}
		if i > 0 {
			lines = append(lines, "")
		}
		status := okStyle.Render("Ready")
		if !n.Ready {
			status = warnStyle.Render("NotReady")
		}
		if n.MemoryPressure {
			status = critStyle.Render("MemoryPressure")
		}
		lines = append(lines, headStyle.Render("  "+n.Name)+" "+status)

		if n.AllocatableCPUM > 0 || n.AllocatableMemMi > 0 {
			lines = append(lines, "  "+kv("Allocatable", fmt.Sprintf("%s CPU · %s mem",
				cpuStr(n.AllocatableCPUM), memStr(n.AllocatableMemMi))))
		}

		scoped, other := podsOnNode(b, n.Name)
		sAgg := aggregatePodResources(scoped)
		oAgg := aggregatePodResources(other)

		lines = append(lines, "  "+kv("This workload", fmt.Sprintf("%s req · %s lim · %d pod(s)",
			cpuStr(sAgg.cpuReq)+"/"+memStr(sAgg.memReq),
			cpuStr(sAgg.cpuLim)+"/"+memStr(sAgg.memLim),
			len(scoped))))
		if len(other) > 0 {
			lines = append(lines, "  "+kv("Co-located", fmt.Sprintf("%s req · %s lim · %d pod(s)",
				cpuStr(oAgg.cpuReq)+"/"+memStr(oAgg.memReq),
				cpuStr(oAgg.cpuLim)+"/"+memStr(oAgg.memLim),
				len(other))))
		} else if len(b.NodePods) == 0 {
			lines = append(lines, "  "+kv("Co-located", dimStyle.Render("none collected (refresh snapshot)")))
		} else {
			lines = append(lines, "  "+kv("Co-located", dimStyle.Render("none on this node")))
		}

		freeCPU := n.AllocatableCPUM - sAgg.cpuReq - oAgg.cpuReq
		freeMem := n.AllocatableMemMi - sAgg.memReq - oAgg.memReq
		if n.AllocatableCPUM > 0 {
			freeLine := fmt.Sprintf("%s CPU · %s mem", cpuStr(maxI64(freeCPU, 0)), memStr(maxI64(freeMem, 0)))
			if freeCPU < 0 || freeMem < 0 {
				freeLine = warnStyle.Render(freeLine + " ⚠ overcommitted")
			}
			lines = append(lines, "  "+kv("Unclaimed", freeLine))
			lines = append(lines, fmt.Sprintf("      %s cpu  %s mem by requests",
				specShareBar(sAgg.cpuReq+oAgg.cpuReq, n.AllocatableCPUM, 10, warnStyle),
				specShareBar(sAgg.memReq+oAgg.memReq, n.AllocatableMemMi, 10, warnStyle)))
		}

		if m.Available {
			usageCPU, usageMem := nodeLiveUsage(b, n.Name, m)
			if usageCPU > 0 || usageMem > 0 {
				lines = append(lines, "  "+kv("Live usage", fmt.Sprintf("%s CPU · %s mem (scoped pods)",
					cpuStr(usageCPU), memStr(usageMem))))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func podsOnNode(b model.EvidenceBundle, node string) (scoped, other []model.PodSummary) {
	for _, p := range b.Pods {
		if p.Node == node {
			scoped = append(scoped, p)
		}
	}
	for _, p := range b.NodePods {
		if p.Node == node {
			other = append(other, p)
		}
	}
	return scoped, other
}

func nodeSchedulingHeadroom(b model.EvidenceBundle) (cpu, mem int64) {
	for _, n := range b.Nodes {
		scoped, other := podsOnNode(b, n.Name)
		sAgg := aggregatePodResources(scoped)
		oAgg := aggregatePodResources(other)
		if n.AllocatableCPUM > 0 {
			cpu += maxI64(n.AllocatableCPUM-sAgg.cpuReq-oAgg.cpuReq, 0)
		}
		if n.AllocatableMemMi > 0 {
			mem += maxI64(n.AllocatableMemMi-sAgg.memReq-oAgg.memReq, 0)
		}
	}
	return cpu, mem
}

func nodeLiveUsage(b model.EvidenceBundle, node string, m model.MetricsSummary) (cpu, mem int64) {
	if !m.Available {
		return 0, 0
	}
	scoped, _ := podsOnNode(b, node)
	totalLimCPU, totalLimMem := aggregatePodResources(scoped).cpuLim, aggregatePodResources(scoped).memLim
	allLimCPU, allLimMem := aggregatePodResources(b.Pods).cpuLim, aggregatePodResources(b.Pods).memLim
	if allLimCPU > 0 && totalLimCPU > 0 {
		cpu = m.CPUUsageM * totalLimCPU / allLimCPU
	}
	if allLimMem > 0 && totalLimMem > 0 {
		mem = m.MemUsageMi * totalLimMem / allLimMem
	}
	return cpu, mem
}

func investigationPodsBody(b model.EvidenceBundle, m model.MetricsSummary, half, rows int) string {
	return scopedPodsTable(b, m, half, rows, true)
}

func colocatedPodsBody(b model.EvidenceBundle, m model.MetricsSummary, half, rows int) string {
	if len(b.NodePods) == 0 {
		return dimStyle.Render("  no co-located pods collected\n  re-run analyze to peer pods on node(s)")
	}
	return scopedPodsTable(b, m, half, rows, false)
}

func scopedPodsTable(b model.EvidenceBundle, m model.MetricsSummary, half, rows int, scoped bool) string {
	pods := b.Pods
	if !scoped {
		pods = b.NodePods
	}
	if len(pods) == 0 {
		return dimStyle.Render("  none")
	}

	type row struct {
		pod      model.PodSummary
		label    string
		cpuReq   int64
		memLim   int64
		restarts int32
		flags    string
	}
	var rowsData []row
	for _, p := range pods {
		var cpuReq, memLim int64
		for _, c := range p.Containers {
			cpuReq += parseCPU(c.RequestsCPU)
			if cpuReq == 0 {
				cpuReq += parseCPU(c.LimitsCPU)
			}
			lim := parseMem(c.LimitsMem)
			if lim == 0 {
				lim = parseMem(c.RequestsMem)
			}
			memLim += lim
		}
		label := truncVisual(p.Name, clampInt(half-30, 12, 20))
		if !scoped {
			label = truncVisual(p.Namespace+"/"+p.Name, clampInt(half-30, 14, 22))
		}
		flags := ""
		if podOOMKilled(p) {
			flags = critStyle.Render("OOM")
		} else if !p.Ready {
			flags = warnStyle.Render("!")
		}
		rowsData = append(rowsData, row{pod: p, label: label, cpuReq: cpuReq, memLim: memLim, restarts: p.RestartCount, flags: flags})
	}
	sort.Slice(rowsData, func(i, j int) bool { return rowsData[i].memLim > rowsData[j].memLim })

	nameW := clampInt(half-28, 10, 18)
	header := fmt.Sprintf("  %s %s %s %s",
		padRight("Pod", nameW), padRight("CPU req", 8), padRight("Mem", 10), "Rst")
	lines := []string{dimStyle.Render(header)}

	maxShow := rows - 1
	if maxShow < 1 {
		maxShow = 1
	}
	for i, r := range rowsData {
		if i >= maxShow {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("  … +%d more", len(rowsData)-maxShow)))
			break
		}
		memLabel := memStr(r.memLim) + " lim"
		if podOOMKilled(r.pod) {
			memLabel = critStyle.Render(memStr(r.memLim) + " at limit")
		}
		rest := fmt.Sprintf("%d", r.restarts)
		if r.restarts >= 3 {
			rest = critStyle.Render(rest)
		}
		flag := r.flags
		if flag != "" {
			flag = " " + flag
		}
		lines = append(lines, fmt.Sprintf("  %s %s %s %s%s",
			padRight(r.label, nameW), padRight(cpuStr(r.cpuReq), 8), padRight(memLabel, 10), rest, flag))
	}
	return strings.Join(lines, "\n")
}
