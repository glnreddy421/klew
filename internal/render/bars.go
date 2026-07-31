package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	BarFill   = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	BarEmpty  = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	Label     = lipgloss.NewStyle().Bold(true)
	Muted     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	Critical  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	Warning   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	Healthy   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

// HorizontalBar renders a labeled bar chart row.
func HorizontalBar(label string, value, max, width int) string {
	if max <= 0 {
		max = 1
	}
	if width < 8 {
		width = 8
	}
	filled := int(float64(value) / float64(max) * float64(width))
	if value > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	bar := BarFill.Render(strings.Repeat("█", filled)) + BarEmpty.Render(strings.Repeat("░", width-filled))
	return fmt.Sprintf("  %-22s %s %d", truncate(label, 22), bar, value)
}

// SeverityBar colors by severity name.
func SeverityBar(severity string, value, max, width int) string {
	switch strings.ToLower(severity) {
	case "critical":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(HorizontalBar(severity, value, max, width))
	case "error", "high":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(HorizontalBar(severity, value, max, width))
	case "warning", "medium":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(HorizontalBar(severity, value, max, width))
	default:
		return HorizontalBar(severity, value, max, width)
	}
}

// ConfidenceMeter shows confidence 0-1.
func ConfidenceMeter(conf float64, width int) string {
	if width < 10 {
		width = 10
	}
	filled := int(conf * float64(width))
	if filled > width {
		filled = width
	}
	return BarFill.Render(strings.Repeat("█", filled)) + BarEmpty.Render(strings.Repeat("░", width-filled)) + fmt.Sprintf(" %.0f%%", conf*100)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
