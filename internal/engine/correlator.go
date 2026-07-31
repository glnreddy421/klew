package engine

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// buildCausalChain distills the timeline into a compact, time-ordered story of
// distinct milestones with relative offsets from the trigger, e.g.
// ["Rollout", "Readiness ✗ +82s", "OOMKilled +2m0s", "Endpoints dropped +2m9s"].
// It is evidence-backed (real timestamps), deterministic, and deduplicated.
func buildCausalChain(tl []model.TimelineEvent) []string {
	type step struct {
		t     time.Time
		label string
	}
	var steps []step
	seen := map[string]bool{}
	for _, e := range SortTimeline(tl) {
		if e.Severity == model.SeverityInfo && e.Type != "deploy" && e.Type != "rs" {
			continue
		}
		lbl := chainLabel(e)
		if lbl == "" || seen[lbl] {
			continue
		}
		seen[lbl] = true
		steps = append(steps, step{e.Timestamp.Time(), lbl})
	}
	if len(steps) == 0 {
		return nil
	}
	// keep the trigger plus the four most recent milestones
	if len(steps) > 5 {
		trimmed := []step{steps[0]}
		trimmed = append(trimmed, steps[len(steps)-4:]...)
		steps = trimmed
	}
	t0 := steps[0].t
	out := make([]string, 0, len(steps))
	for i, s := range steps {
		if i == 0 {
			out = append(out, s.label)
			continue
		}
		out = append(out, fmt.Sprintf("%s +%s", s.label, shortDur(s.t.Sub(t0))))
	}
	return out
}

func chainLabel(e model.TimelineEvent) string {
	switch e.Type {
	case "deploy":
		return "Rollout"
	case "rs":
		return "New ReplicaSet"
	case "verdict":
		return ""
	}
	if e.Reason != "" {
		return e.Reason
	}
	return firstWords(e.Message, 3)
}

var chainStopwords = map[string]bool{
	"from": true, "to": true, "of": true, "the": true, "a": true, "an": true,
	"on": true, "in": true, "for": true, "with": true, "and": true,
}

func firstWords(s string, n int) string {
	f := strings.Fields(s)
	if len(f) > n {
		f = f[:n]
	}
	for len(f) > 1 && chainStopwords[strings.ToLower(f[len(f)-1])] {
		f = f[:len(f)-1]
	}
	return strings.Join(f, " ")
}

func shortDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// SortTimeline orders events by timestamp ascending.
func SortTimeline(events []model.TimelineEvent) []model.TimelineEvent {
	out := append([]model.TimelineEvent(nil), events...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp.Equal(out[j].Timestamp) {
			return severityRank(out[i].Severity) > severityRank(out[j].Severity)
		}
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

func severityRank(s model.Severity) int {
	switch s {
	case model.SeverityCritical:
		return 4
	case model.SeverityHigh:
		return 3
	case model.SeverityWarning:
		return 2
	default:
		return 1
	}
}

// BuildGraph constructs Deployment -> RS -> Pod -> Container and Service edges.
func BuildGraph(b model.EvidenceBundle) model.WorkloadGraph {
	g := model.WorkloadGraph{Health: "unknown"}
	nodeIndex := map[string]model.GraphNode{}

	addNode := func(kind, name, health string) string {
		id := fmt.Sprintf("%s/%s", kind, name)
		if _, ok := nodeIndex[id]; !ok {
			nodeIndex[id] = model.GraphNode{ID: id, Kind: kind, Name: name, Health: health}
		}
		return id
	}
	addEdge := func(from, to, rel, ann string) {
		g.Edges = append(g.Edges, model.GraphEdge{From: from, To: to, Relation: rel, Annotation: ann})
	}

	for _, w := range b.Workloads {
		h := workloadHealth(w)
		addNode(w.Kind, w.Name, h)
	}
	for _, rs := range b.ReplicaSets {
		h := "healthy"
		if rs.Ready < rs.Replicas {
			h = "warning"
		}
		rsID := addNode("ReplicaSet", rs.Name, h)
		if rs.DeploymentOwner != "" {
			depID := addNode("Deployment", rs.DeploymentOwner, "unknown")
			addEdge(depID, rsID, "owns", "replicaSet")
		}
	}
	for _, p := range b.Pods {
		h := podHealth(p)
		podID := addNode("Pod", p.Name, h)
		for _, o := range p.OwnerRefs {
			ownerID := addNode(o.Kind, o.Name, "unknown")
			addEdge(ownerID, podID, "owns", "ownerReference")
		}
		if p.Node != "" {
			nodeID := addNode("Node", p.Node, nodeHealth(b.Nodes, p.Node))
			addEdge(podID, nodeID, "scheduledOn", p.Node)
		}
		for _, c := range p.Containers {
			cID := addNode("Container", p.Name+"/"+c.Name, containerHealth(c))
			addEdge(podID, cID, "runs", c.Image)
		}
		for _, cm := range p.ConfigMapRefs {
			cmID := addNode("ConfigMap", cm, "unknown")
			addEdge(podID, cmID, "mounts", "configMap")
		}
		for _, sec := range p.SecretRefs {
			sID := addNode("Secret", sec, "unknown")
			addEdge(podID, sID, "mounts", "secret")
		}
		for _, pvc := range p.PVCRefs {
			pvcID := addNode("PVC", pvc, "unknown")
			addEdge(podID, pvcID, "mounts", "pvc")
		}
	}
	for _, svc := range b.Services {
		sh := "healthy"
		if svc.ReadyEndpoints == 0 {
			sh = "critical"
		}
		svcID := addNode("Service", svc.Name, sh)
		for _, p := range b.Pods {
			if serviceSelectsPod(svc.Selector, p.Labels) {
				addEdge(svcID, addNode("Pod", p.Name, podHealth(p)), "routesTo", "selector")
			}
		}
	}
	for _, ing := range b.Ingresses {
		ingID := addNode("Ingress", ing.Name, "unknown")
		for _, backend := range ing.Backends {
			svcID := addNode("Service", backend, "unknown")
			addEdge(ingID, svcID, "routesTo", "ingress")
		}
	}
	for _, hpa := range b.HPAs {
		hh := hpaHealth(hpa)
		hpaID := addNode("HPA", hpa.Name, hh)
		if hpa.TargetKind != "" && hpa.TargetName != "" {
			targetKey := fmt.Sprintf("%s/%s", hpa.TargetKind, hpa.TargetName)
			targetHealth := "unknown"
			if n, ok := nodeIndex[targetKey]; ok {
				targetHealth = n.Health
			}
			targetID := addNode(hpa.TargetKind, hpa.TargetName, targetHealth)
			addEdge(hpaID, targetID, "scales", "targetRef")
		}
	}

	for _, n := range nodeIndex {
		g.Nodes = append(g.Nodes, n)
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })

	g.HealthByNode = map[string]string{}
	for _, n := range g.Nodes {
		g.HealthByNode[n.ID] = n.Health
	}
	g.Health = graphOverallHealth(g.Nodes)
	g.PropagationPath = propagationPath(g)
	return g
}

func serviceSelectsPod(selector string, labels map[string]string) bool {
	if selector == "" || len(labels) == 0 {
		return false
	}
	// selector format from labels.Set.String(): key=value,key2=value2
	parts := strings.Split(selector, ",")
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if labels[kv[0]] != kv[1] {
			return false
		}
	}
	return len(parts) > 0
}

func workloadHealth(w model.WorkloadSummary) string {
	if w.Ready < w.Replicas {
		return "warning"
	}
	if w.Available == 0 && w.Replicas > 0 {
		return "critical"
	}
	return "healthy"
}

func podHealth(p model.PodSummary) string {
	if p.Phase == "Running" && p.Ready {
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

func containerHealth(c model.ContainerStatus) string {
	if c.LastReason == "OOMKilled" || strings.Contains(strings.ToLower(c.LastReason), "crashloop") {
		return "critical"
	}
	if !c.Ready {
		return "warning"
	}
	return "healthy"
}

func nodeHealth(nodes []model.NodeSummary, name string) string {
	for _, n := range nodes {
		if n.Name == name {
			if !n.Ready || n.MemoryPressure || n.DiskPressure || n.PIDPressure {
				return "critical"
			}
			return "healthy"
		}
	}
	return "unknown"
}

func graphOverallHealth(nodes []model.GraphNode) string {
	worst := "healthy"
	for _, n := range nodes {
		if n.Health == "critical" {
			return "critical"
		}
		if n.Health == "warning" {
			worst = "warning"
		}
	}
	return worst
}

func hpaHealth(h model.HPASummary) string {
	if h.AtMax && h.DesiredReplicas >= h.MaxReplicas {
		return "warning"
	}
	if h.DesiredReplicas != h.CurrentReplicas {
		return "warning"
	}
	return "healthy"
}

func propagationPath(g model.WorkloadGraph) []string {
	var path []string
	for _, e := range g.Edges {
		if e.Relation == "owns" || e.Relation == "routesTo" || e.Relation == "scales" || e.Relation == "related" {
			path = append(path, fmt.Sprintf("%s -> %s (%s)", e.From, e.To, e.Annotation))
		}
	}
	return path
}
