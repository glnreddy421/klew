package render

import "fmt"

// RestartMatrix renders pod restart counts as a compact row.
func RestartMatrix(pods []struct{ Name string; Restarts int32 }) string {
	if len(pods) == 0 {
		return "  (no pods)"
	}
	var line string
	for i, p := range pods {
		cell := "·"
		if p.Restarts > 0 {
			cell = fmt.Sprintf("%d", p.Restarts)
		}
		if i > 0 {
			line += " "
		}
		line += fmt.Sprintf("%-12s[%s]", truncate(p.Name, 12), cell)
	}
	return line
}
