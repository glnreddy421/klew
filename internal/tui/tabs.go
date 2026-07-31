package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// TabMeta is the human-facing title and purpose line for each investigation tab.
type TabMeta struct {
	Title    string
	Subtitle string
}

var tabMeta = []TabMeta{
	{
		Title:    "Incident",
		Subtitle: `Does it answer "What broke?" in under 5 seconds?`,
	},
	{
		Title:    "Timeline",
		Subtitle: "Does it explain how the incident unfolded?",
	},
	{
		Title:    "Graph",
		Subtitle: "Does it clearly show blast radius and relationships?",
	},
	{
		Title:    "Failures",
		Subtitle: "Can I immediately identify the failing runtime?",
	},
	{
		Title:    "Resources",
		Subtitle: "Can I tell whether CPU/memory contributed?",
	},
	{
		Title:    "Evidence",
		Subtitle: "Does it convincingly justify Klew's verdict?",
	},
}

// Meta returns title and subtitle for a tab.
func (t Tab) Meta() TabMeta {
	if int(t) >= 0 && int(t) < len(tabMeta) {
		return tabMeta[t]
	}
	return TabMeta{Title: "Investigation"}
}

// RenderTabBanner renders the active tab's title and purpose under the tab bar.
func RenderTabBanner(active Tab, width int) string {
	m := active.Meta()
	title := BannerTitle.Render(m.Title)
	sep := Muted.Render(" — ")
	sub := Muted.Render(m.Subtitle)
	line := title + sep + sub
	if width > 0 && lipgloss.Width(line) > width {
		return truncateWidth(line, width)
	}
	return line
}

// tabLabels are the compact tab-bar labels (number + title).
var tabLabels = []string{
	"1 Incident", "2 Timeline", "3 Graph", "4 Failures", "5 Resources", "6 Evidence",
}
