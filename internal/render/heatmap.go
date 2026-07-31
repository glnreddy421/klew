package render

// heatmap.go — v1 uses histogram bars; heatmap reserved for pod readiness matrix in TUI.

import "fmt"

// ReadinessMatrix renders pod ready/not-ready grid.
func ReadinessMatrix(pods []struct{ Name string; Ready bool }) []string {
	if len(pods) == 0 {
		return []string{"  (no pods)"}
	}
	var lines []string
	for _, p := range pods {
		cell := "░"
		if p.Ready {
			cell = BarFill.Render("█")
		} else {
			cell = lipglossErr.Render("▓")
		}
		lines = append(lines, fmt.Sprintf("  %-40s %s", truncate(p.Name, 40), cell))
	}
	return lines
}

// lipgloss inline to avoid circular import in simple helper
var lipglossErr = BarEmpty
