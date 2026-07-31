package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glnreddy421/klew/internal/investigation"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/render"
)

// GraphView — tab 3 Graph: blast radius, propagation chain, and full workload map.
func GraphView(st model.InvestigationState, width, scroll, height int) string {
	if width < 40 {
		width = 40
	}
	cw := width - 4
	innerRows := height - 2 // panel top + bottom border
	if innerRows < 6 {
		innerRows = 6
	}

	layout := GraphLayoutFor(st)
	body := buildGraphScrollBody(st, st.Snapshot, layout, cw)
	visual := expandGraphLines(body, cw)
	scroll = ClampScroll(scroll, len(visual), innerRows)
	end := scroll + innerRows
	if end > len(visual) {
		end = len(visual)
	}
	var visible string
	if scroll >= len(visual) {
		visible = ""
	} else {
		visible = strings.Join(visual[scroll:end], "\n")
	}

	title := "Graph · Workload Map"
	if len(visual) > innerRows {
		title += fmt.Sprintf(" · %d–%d/%d · j/k scroll", scroll+1, end, len(visual))
	}
	return Panel(title, width, visible)
}

// GraphLineCount returns visual line count for the graph body at the given width.
func GraphLineCount(st model.InvestigationState, width int) int {
	if width < 40 {
		width = 40
	}
	cw := width - 4
	layout := GraphLayoutFor(st)
	body := buildGraphScrollBody(st, st.Snapshot, layout, cw)
	return len(expandGraphLines(body, cw))
}

func expandGraphLines(body string, cw int) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		out = append(out, wrapPanelLine(line, cw)...)
	}
	return out
}

func buildGraphScrollBody(st model.InvestigationState, b model.EvidenceBundle, layout render.GraphLayout, cw int) string {
	if cw < 20 {
		cw = 20
	}
	var parts []string

	c := affectedCounts(b)
	parts = append(parts, headStyle.Render("Blast Radius"))
	parts = append(parts, fmt.Sprintf("  %s   %s   %s   %s",
		critStyle.Render(fmt.Sprintf("✖ %d critical", c.critical)),
		warnStyle.Render(fmt.Sprintf("⚠ %d warning", c.warning)),
		okStyle.Render(fmt.Sprintf("✓ %d healthy", c.healthy)),
		markStyle.Render(fmt.Sprintf("· %d pods · %d failing", c.pods, c.podsFailing))))
	parts = append(parts, "")

	parts = append(parts, headStyle.Render("Propagation Chain"))
	parts = append(parts, PropagationFlow(layout.Steps, cw))
	parts = append(parts, "")

	parts = append(parts, headStyle.Render("Workload Map"))
	parts = append(parts, dimStyle.Render("  ✖ critical   ⚠ warning   ✓ healthy   · unknown"))
	parts = append(parts, WorkloadMap(st, layout, cw))
	parts = append(parts, "")

	parts = append(parts, headStyle.Render("Associated Resources"))
	parts = append(parts, workloadAssociations(st, b, cw))

	return strings.Join(parts, "\n")
}

// GraphReport renders the full graph body for CLI output (same content as the Graph tab).
func GraphReport(st model.InvestigationState, width int) string {
	if width < 40 {
		width = 40
	}
	layout := GraphLayoutFor(st)
	return buildGraphScrollBody(st, st.Snapshot, layout, width)
}

// PropagationFlow renders a vertical propagation chain that wraps within width.
func PropagationFlow(steps []render.GraphStep, width int) string {
	if len(steps) == 0 {
		return "  " + dimStyle.Render("(propagation path not inferred yet)")
	}
	cw := width
	if cw < 16 {
		cw = 16
	}
	var lines []string
	for i, s := range steps {
		if i > 0 {
			rel := steps[i-1].Relation
			if rel != "" {
				for _, ln := range wrapPlain("│ "+rel, cw-2) {
					lines = append(lines, markStyle.Render("  "+ln))
				}
			} else {
				lines = append(lines, markStyle.Render("  │"))
			}
			lines = append(lines, markStyle.Render("  ▼"))
		}
		line := fmt.Sprintf("%s %s/%s", healthMarker(s.Health), s.Kind, s.Name)
		for _, ln := range wrapPlain(line, cw-2) {
			lines = append(lines, "  "+graphLabel(ln, s.Health))
		}
	}
	return strings.Join(lines, "\n")
}

func healthMarker(health string) string {
	switch strings.ToLower(health) {
	case "critical":
		return "✖"
	case "warning", "degraded":
		return "⚠"
	case "healthy":
		return "✓"
	default:
		return "·"
	}
}

func wrapPlain(s string, w int) []string {
	if w <= 0 || len(s) <= w {
		return []string{s}
	}
	var out []string
	for len(s) > w {
		cut := w
		if sp := strings.LastIndex(s[:cut], " "); sp > w/3 {
			cut = sp
		}
		out = append(out, strings.TrimRight(s[:cut], " "))
		s = strings.TrimLeft(s[cut:], " ")
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

// WorkloadMap renders the full layered workload topology with issue highlighting.
func WorkloadMap(st model.InvestigationState, layout render.GraphLayout, width int) string {
	if len(layout.Nodes) == 0 {
		return "  " + dimStyle.Render("(no graph nodes — waiting for snapshot)")
	}

	layers := groupMapLayers(layout.Nodes)
	order := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	titles := map[int]string{
		0: "Traffic", 1: "Service", 2: "Autoscaling", 3: "Workload", 4: "Rollout",
		5: "Pods", 6: "Containers", 7: "Config & Storage", 8: "Infrastructure", 9: "Operators / CRD",
	}

	edgeByFrom := map[string][]model.GraphEdge{}
	for _, e := range layout.Edges {
		edgeByFrom[e.From] = append(edgeByFrom[e.From], e)
	}

	var lines []string
	firstLayer := true
	for _, layer := range order {
		nodes := layers[layer]
		if len(nodes) == 0 {
			continue
		}
		if !firstLayer {
			lines = append(lines, markStyle.Render("  ╎"))
		}
		firstLayer = false
		title := titles[layer]
		if title == "" {
			title = fmt.Sprintf("Layer %d", layer)
		}
		lines = append(lines, headStyle.Render("  ▼ "+title))

		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
		for ni, n := range nodes {
			if ni > 0 {
				lines = append(lines, dimStyle.Render("  ·"))
			}
			fact := nodeFact(st, n)
			label := fmt.Sprintf("%s %s/%s", healthMarker(n.Health), n.Kind, n.Name)
			for _, ln := range wrapPlain(label, width-4) {
				lines = append(lines, "    "+graphLabel(ln, n.Health))
			}
			if fact != "" {
				for _, ln := range wrapPlain(fact, width-6) {
					lines = append(lines, "      "+graphNote(n.Health, ln))
				}
			}
			for _, e := range edgeByFrom[n.ID] {
				rel := e.Relation
				if e.Annotation != "" && rel == "owns" {
					rel = e.Annotation
				}
				toKind, toName := splitNodeID(e.To)
				edgeLine := fmt.Sprintf("└─[%s]─▶ %s/%s", rel, toKind, toName)
				for _, ln := range wrapPlain(edgeLine, width-6) {
					lines = append(lines, dimStyle.Render("      "+ln))
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

func nodeFact(st model.InvestigationState, n model.GraphNode) string {
	b := st.Snapshot
	switch n.Kind {
	case "Service":
		for _, svc := range b.Services {
			if svc.Name == n.Name {
				return fmt.Sprintf("endpoints %d/%d", svc.ReadyEndpoints, svc.TotalEndpoints)
			}
		}
	case "Deployment", "StatefulSet", "DaemonSet":
		for _, w := range b.Workloads {
			if w.Name == n.Name {
				return fmt.Sprintf("ready %d/%d · gen %d", w.Ready, w.Replicas, w.Generation)
			}
		}
	case "ReplicaSet":
		for _, rs := range b.ReplicaSets {
			if rs.Name == n.Name {
				return fmt.Sprintf("ready %d/%d", rs.Ready, rs.Replicas)
			}
		}
	case "Pod":
		for _, p := range b.Pods {
			if p.Name != n.Name {
				continue
			}
			base := podScopeTag(b, p.Name)
			if podOOMKilled(p) {
				return base + "OOMKilled exit 137"
			}
			for _, c := range p.Containers {
				if c.Reason == "CrashLoopBackOff" || strings.Contains(strings.ToLower(c.LastReason), "crash") {
					return base + fmt.Sprintf("container %s · restarts %d", c.Name, p.RestartCount)
				}
			}
			if !p.Ready {
				return base + fmt.Sprintf("not ready · restarts %d", p.RestartCount)
			}
			return base + "ready"
		}
		if colocated := colocatedPod(b, n.Name); colocated != nil {
			tag := "co-located · "
			if podOOMKilled(*colocated) {
				return tag + "OOMKilled"
			}
			if !colocated.Ready {
				return tag + "not ready"
			}
			return tag + "running"
		}
	case "Container":
		for _, p := range b.Pods {
			for _, c := range p.Containers {
				id := p.Name + "/" + c.Name
				if id != n.Name && c.Name != n.Name {
					continue
				}
				if c.LastReason == "OOMKilled" {
					return "OOMKilled · limit " + c.LimitsMem
				}
				if c.LastReason != "" {
					return fmt.Sprintf("%s exit %d", c.LastReason, c.LastExitCode)
				}
				return truncVisual(c.Image, 40)
			}
		}
	case "HPA":
		for _, h := range b.HPAs {
			if h.Name == n.Name {
				return fmt.Sprintf("replicas %d/%d (max %d)", h.CurrentReplicas, h.DesiredReplicas, h.MaxReplicas)
			}
		}
	case "Node":
		for _, node := range b.Nodes {
			if node.Name == n.Name {
				if node.AllocatableCPUM > 0 {
					return fmt.Sprintf("alloc %s CPU · %s mem", cpuStr(node.AllocatableCPUM), memStr(node.AllocatableMemMi))
				}
				return nodePressureNote(node)
			}
		}
	}
	if ext := crdExtension(st, n.Kind); ext != "" {
		return "operator · " + ext
	}
	return ""
}

func crdExtension(st model.InvestigationState, kind string) string {
	if st.Scope == nil {
		return ""
	}
	for _, crd := range st.Scope.RelatedCRDs {
		if crd.Kind == kind {
			return crd.Extension
		}
	}
	return ""
}

func splitNodeID(id string) (kind, name string) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", id
}

var mapLayer = map[string]int{
	"Ingress": 0, "Service": 1, "HPA": 2,
	"Deployment": 3, "StatefulSet": 3, "DaemonSet": 3, "Job": 3, "CronJob": 3,
	"ReplicaSet": 4, "Pod": 5, "Container": 6,
	"ConfigMap": 7, "Secret": 7, "PVC": 7,
	"Node": 8,
}

var stdKinds = map[string]bool{
	"Ingress": true, "Service": true, "HPA": true,
	"Deployment": true, "StatefulSet": true, "DaemonSet": true, "Job": true, "CronJob": true,
	"ReplicaSet": true, "Pod": true, "Container": true,
	"ConfigMap": true, "Secret": true, "PVC": true, "Node": true,
}

func groupMapLayers(nodes []render.LayoutNode) map[int][]model.GraphNode {
	out := map[int][]model.GraphNode{}
	for _, ln := range nodes {
		l := mapLayer[ln.Kind]
		if !stdKinds[ln.Kind] {
			l = 9
		}
		out[l] = append(out[l], ln.GraphNode)
	}
	return out
}

func workloadAssociations(st model.InvestigationState, b model.EvidenceBundle, width int) string {
	var lines []string
	add := func(label, detail string) {
		for _, ln := range wrapPlain(label+": "+detail, width-2) {
			lines = append(lines, "  "+ln)
		}
	}

	if st.Scope != nil && len(st.Scope.RelatedCRDs) > 0 {
		for _, crd := range st.Scope.RelatedCRDs {
			names := make([]string, 0, len(crd.Refs))
			for _, r := range crd.Refs {
				names = append(names, r.String())
			}
			health := "healthy"
			add(fmt.Sprintf("%s (%s)", crd.Kind, crd.Extension), strings.Join(names, ", "))
			_ = health
		}
	} else if len(b.DetectedCRDKinds) > 0 {
		add("Detected API groups", strings.Join(b.DetectedCRDKinds[:minInt(5, len(b.DetectedCRDKinds))], ", "))
	}

	if len(b.ConfigRefs) > 0 {
		names := make([]string, len(b.ConfigRefs))
		for i, r := range b.ConfigRefs {
			names[i] = r.Name
		}
		add("ConfigMaps", strings.Join(names, ", "))
	}
	if len(b.SecretRefs) > 0 {
		names := make([]string, len(b.SecretRefs))
		for i, r := range b.SecretRefs {
			names[i] = r.Name
		}
		add("Secrets", strings.Join(names, ", "))
	}
	if len(b.PVCRefs) > 0 {
		names := make([]string, len(b.PVCRefs))
		for i, r := range b.PVCRefs {
			names[i] = r.Name
		}
		add("PVCs", strings.Join(names, ", "))
	}

	if len(lines) == 0 {
		return "  " + dimStyle.Render("no extensions or mounted resources beyond workload graph")
	}
	return strings.Join(lines, "\n")
}

func podScopeTag(b model.EvidenceBundle, name string) string {
	for _, p := range b.Pods {
		if p.Name == name {
			return "in scope · "
		}
	}
	return ""
}

func colocatedPod(b model.EvidenceBundle, name string) *model.PodSummary {
	for i := range b.NodePods {
		if b.NodePods[i].Name == name {
			return &b.NodePods[i]
		}
	}
	return nil
}

func enrichWorkloadGraph(g model.WorkloadGraph, st model.InvestigationState) model.WorkloadGraph {
	index := map[string]bool{}
	for _, n := range g.Nodes {
		index[n.ID] = true
	}
	addNode := func(kind, name string) string {
		id := kind + "/" + name
		if index[id] {
			return id
		}
		index[id] = true
		health := "unknown"
		if g.HealthByNode != nil {
			health = g.HealthByNode[id]
		}
		g.Nodes = append(g.Nodes, model.GraphNode{ID: id, Kind: kind, Name: name, Health: health})
		return id
	}
	addEdge := func(from, to, rel, ann string) {
		g.Edges = append(g.Edges, model.GraphEdge{From: from, To: to, Relation: rel, Annotation: ann})
	}

	for _, p := range st.Snapshot.Pods {
		podID := addNode("Pod", p.Name)
		for _, c := range p.Containers {
			cID := addNode("Container", p.Name+"/"+c.Name)
			addEdge(podID, cID, "runs", c.Image)
		}
		if p.Node != "" {
			addEdge(podID, addNode("Node", p.Node), "scheduledOn", p.Node)
		}
	}
	for _, rs := range st.Snapshot.ReplicaSets {
		rsID := addNode("ReplicaSet", rs.Name)
		if rs.DeploymentOwner != "" {
			addEdge(addNode("Deployment", rs.DeploymentOwner), rsID, "owns", "replicaSet")
		}
	}
	for _, hpa := range st.Snapshot.HPAs {
		hpaID := addNode("HPA", hpa.Name)
		if hpa.TargetKind != "" && hpa.TargetName != "" {
			addEdge(hpaID, addNode(hpa.TargetKind, hpa.TargetName), "scales", "targetRef")
		}
	}
	for _, p := range st.Snapshot.NodePods {
		id := "Pod/" + p.Name
		if index[id] {
			continue
		}
		index[id] = true
		g.Nodes = append(g.Nodes, model.GraphNode{
			ID: id, Kind: "Pod", Name: p.Name, Health: podHealthLabel(p),
		})
		if p.Node != "" {
			addEdge(id, addNode("Node", p.Node), "scheduledOn", "co-located")
		}
	}

	if st.Scope == nil {
		if g.HealthByNode == nil {
			g.HealthByNode = map[string]string{}
		}
		for _, n := range g.Nodes {
			if g.HealthByNode[n.ID] == "" {
				g.HealthByNode[n.ID] = n.Health
			}
		}
		return g
	}

	rootID := st.Scope.RootKind + "/" + st.Scope.RootName
	if !index[rootID] && st.Scope.RootName != "" {
		g.Nodes = append(g.Nodes, model.GraphNode{
			ID: rootID, Kind: st.Scope.RootKind, Name: st.Scope.RootName, Health: g.HealthByNode[rootID],
		})
		index[rootID] = true
	}
	for _, rel := range st.Scope.Relationships {
		from := addNode(rel.From.Kind, rel.From.Name)
		to := addNode(rel.To.Kind, rel.To.Name)
		addEdge(from, to, string(rel.Kind), string(rel.Kind))
	}
	for _, crd := range st.Scope.RelatedCRDs {
		for _, ref := range crd.Refs {
			id := ref.Kind + "/" + ref.Name
			if index[id] {
				continue
			}
			index[id] = true
			health := "healthy"
			g.Nodes = append(g.Nodes, model.GraphNode{ID: id, Kind: ref.Kind, Name: ref.Name, Health: health})
			if rootID != "" {
				g.Edges = append(g.Edges, model.GraphEdge{
					From: rootID, To: id, Relation: string(investigation.RelRelated), Annotation: crd.Extension,
				})
			}
		}
	}
	if g.HealthByNode == nil {
		g.HealthByNode = map[string]string{}
	}
	for _, n := range g.Nodes {
		if g.HealthByNode[n.ID] == "" {
			g.HealthByNode[n.ID] = n.Health
		}
	}
	return g
}

// GraphLayoutFor returns the layout used by TUI and web, enriched with scope CRDs.
func GraphLayoutFor(st model.InvestigationState) render.GraphLayout {
	g := enrichWorkloadGraph(st.WorkloadGraph, st)
	return render.LayoutGraph(g)
}

// RelationshipGraph kept for compatibility — prefer WorkloadMap in GraphView.
func RelationshipGraph(layout render.GraphLayout, width, maxLines int) string {
	return WorkloadMap(model.InvestigationState{}, layout, width)
}
