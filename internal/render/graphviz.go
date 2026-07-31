package render

import (
	"regexp"
	"sort"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// GraphStep is one node in the propagation flow shown in TUI and web.
type GraphStep struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	Health   string `json:"health"`
	Relation string `json:"relation,omitempty"`
}

// LayoutNode is a graph node positioned for SVG or ASCII layout.
type LayoutNode struct {
	model.GraphNode
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// GraphLayout is a positioned workload graph for interactive views.
type GraphLayout struct {
	Nodes  []LayoutNode      `json:"nodes"`
	Edges  []model.GraphEdge `json:"edges"`
	Steps  []GraphStep       `json:"steps"`
	Width  float64           `json:"width"`
	Height float64           `json:"height"`
}

var kindLayer = map[string]int{
	"Ingress": 0, "Service": 1, "HPA": 2,
	"Deployment": 3, "StatefulSet": 3, "DaemonSet": 3, "Job": 3, "CronJob": 3,
	"ReplicaSet": 4, "Pod": 5, "Container": 6,
	"ConfigMap": 7, "Secret": 7, "PVC": 7,
	"Node": 8,
}

func layerForKind(kind string) int {
	if l, ok := kindLayer[kind]; ok {
		return l
	}
	return 9 // operators / CRD extensions
}

var pathEdgeRe = regexp.MustCompile(`^(.+?)\s*->\s*(.+?)(?:\s*\((.+)\))?$`)

// PropagationSteps derives an ordered node chain from the graph propagation path.
func PropagationSteps(g model.WorkloadGraph) []GraphStep {
	if len(g.PropagationPath) == 0 {
		return inferPropagationSteps(g)
	}
	var steps []GraphStep
	seen := map[string]bool{}
	appendStep := func(id, relation string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		kind, name := splitGraphID(id)
		health := g.HealthByNode[id]
		if health == "" {
			health = nodeHealthFromGraph(g, id)
		}
		steps = append(steps, GraphStep{
			ID: id, Kind: kind, Name: name,
			Label: shortGraphLabel(kind, name),
			Health: health, Relation: relation,
		})
	}
	for _, p := range g.PropagationPath {
		from, to, rel := parsePathEdge(p)
		if len(steps) == 0 {
			appendStep(from, "")
		}
		if rel != "" && len(steps) > 0 {
			steps[len(steps)-1].Relation = rel
		}
		appendStep(to, "")
	}
	return steps
}

func inferPropagationSteps(g model.WorkloadGraph) []GraphStep {
	if len(g.Nodes) == 0 {
		return nil
	}
	order := []string{"Ingress", "Service", "HPA", "Deployment", "StatefulSet", "ReplicaSet", "Pod", "Container"}
	var steps []GraphStep
	for _, kind := range order {
		for _, n := range g.Nodes {
			if n.Kind != kind {
				continue
			}
			steps = append(steps, GraphStep{
				ID: n.ID, Kind: n.Kind, Name: n.Name,
				Label: shortGraphLabel(n.Kind, n.Name), Health: n.Health,
			})
		}
	}
	if len(steps) > 8 {
		steps = steps[:8]
	}
	return steps
}

func parsePathEdge(p string) (from, to, rel string) {
	m := pathEdgeRe.FindStringSubmatch(strings.TrimSpace(p))
	if len(m) < 3 {
		return "", "", ""
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), strings.TrimSpace(m[3])
}

func splitGraphID(id string) (kind, name string) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", id
}

func shortGraphLabel(kind, name string) string {
	if kind == "" {
		return name
	}
	if len(name) > 22 {
		name = name[:19] + "…"
	}
	return kind + "/" + name
}

func nodeHealthFromGraph(g model.WorkloadGraph, id string) string {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n.Health
		}
	}
	return "unknown"
}

// LayoutGraph assigns coordinates to nodes for web SVG and TUI ASCII layout.
func LayoutGraph(g model.WorkloadGraph) GraphLayout {
	if len(g.Nodes) == 0 {
		return GraphLayout{Steps: PropagationSteps(g)}
	}

	layers := map[int][]model.GraphNode{}
	maxLayer := 0
	for _, n := range g.Nodes {
		l := layerForKind(n.Kind)
		layers[l] = append(layers[l], n)
		if l > maxLayer {
			maxLayer = l
		}
	}
	for l := range layers {
		sort.Slice(layers[l], func(i, j int) bool {
			return layers[l][i].Name < layers[l][j].Name
		})
	}

	const (
		// Spacing tuned for circular icon nodes in the desktop graph.
		nodeW   = 110.0
		padX    = 48.0
		padY    = 40.0
		layerDY = 100.0
	)
	maxInLayer := 1
	for _, nodes := range layers {
		if len(nodes) > maxInLayer {
			maxInLayer = len(nodes)
		}
	}
	width := padX*2 + float64(maxInLayer)*nodeW
	if width < 320 {
		width = 320
	}
	height := padY*2 + float64(maxLayer+1)*layerDY

	var layoutNodes []LayoutNode
	for l := 0; l <= maxLayer; l++ {
		nodes := layers[l]
		rowW := float64(len(nodes)) * nodeW
		startX := (width - rowW) / 2
		if startX < padX {
			startX = padX
		}
		for i, n := range nodes {
			layoutNodes = append(layoutNodes, LayoutNode{
				GraphNode: n,
				X:         startX + float64(i)*nodeW,
				Y:         padY + float64(l)*layerDY,
			})
		}
	}

	return GraphLayout{
		Nodes:  layoutNodes,
		Edges:  g.Edges,
		Steps:  PropagationSteps(g),
		Width:  width,
		Height: height,
	}
}
