package render

import (
	"fmt"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// WorkloadTree renders an ASCII tree from graph edges.
func WorkloadTree(g model.WorkloadGraph, maxEdges int) string {
	if len(g.Edges) == 0 {
		if len(g.Nodes) == 0 {
			return "  (no graph data)"
		}
		var names []string
		for _, n := range g.Nodes {
			names = append(names, fmt.Sprintf("• %s [%s]", n.ID, n.Health))
		}
		return strings.Join(names, "\n")
	}
	var b strings.Builder
	limit := maxEdges
	if limit <= 0 || limit > len(g.Edges) {
		limit = len(g.Edges)
	}
	for i := 0; i < limit; i++ {
		e := g.Edges[i]
		b.WriteString(fmt.Sprintf("  %s\n    │ %s\n    ▼\n  %s\n", e.From, e.Annotation, e.To))
		if i < limit-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// PropagationChain formats path strings.
func PropagationChain(path []string) string {
	if len(path) == 0 {
		return "  (none inferred)"
	}
	var lines []string
	for _, p := range path {
		lines = append(lines, "  • "+p)
	}
	return strings.Join(lines, "\n")
}
