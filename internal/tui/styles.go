package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	TitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	BannerTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	TabActive   = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("57")).Foreground(lipgloss.Color("230"))
	TabInactive = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	StatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	Critical    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	Warning     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	Healthy     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	Muted       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	headStyleTUI = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	selArrow     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	selRow       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
)

var discoverySources = []string{
	"Deployments", "ReplicaSets", "Pods", "Services", "Events",
	"Logs", "EndpointSlices", "ConfigMaps", "Secrets", "Metrics",
}

var scopeBuildSteps = []string{
	"Deployment", "ReplicaSets", "Pods", "Services", "EndpointSlices",
	"Events", "Logs", "ConfigMaps", "Secrets", "Metrics",
}

// Tab IDs for investigation views (K8s-native, not log-analyzer tabs).
type Tab int

const (
	TabIncident Tab = iota
	TabTimeline
	TabGraph
	TabFailures
	TabResources
	TabEvidence
	tabCount
)

func (t Tab) String() string {
	return tabLabels[t]
}

func RenderTabs(active Tab, width int) string {
	var parts []string
	for i := 0; i < int(tabCount); i++ {
		label := "[" + tabLabels[i] + "]"
		if Tab(i) == active {
			parts = append(parts, TabActive.Render(label))
		} else {
			parts = append(parts, TabInactive.Render(label))
		}
	}
	line := strings.Join(parts, " ")
	if lipgloss.Width(line) > width && width > 0 {
		return truncateWidth(line, width)
	}
	return line
}

func VerdictStyle(status string) lipgloss.Style {
	switch strings.ToLower(status) {
	case "critical":
		return Critical
	case "warning":
		return Warning
	case "healthy":
		return Healthy
	default:
		return Muted
	}
}

func truncateWidth(s string, w int) string {
	if w <= 3 {
		return "..."
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:w-1]) + "…"
}

func HeaderLine(title, ctx string) string {
	return TitleStyle.Render(title) + StatusStyle.Render("  │  "+ctx)
}

// FooterHelp returns key hints.
func FooterHelp() string {
	return Muted.Render("1-6 tabs  j/k scroll  f stream  a refresh  e stream  S settings  p pause  q quit  ? help")
}

func Box(title, body string, width int) string {
	b := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	content := TitleStyle.Render(title) + "\n" + body
	if width > 0 {
		return b.Width(width).Render(content)
	}
	return b.Render(content)
}

func JoinLines(lines ...string) string {
	return strings.Join(lines, "\n")
}

func fmtRow(k, v string) string {
	return fmt.Sprintf("  %-18s %s", k+":", v)
}
