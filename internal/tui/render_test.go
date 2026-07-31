package tui

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/engine"
	invmodel "github.com/glnreddy421/klew/internal/model"
)

// TestRenderAllTabs is a smoke test: every tab must render non-empty output
// with the persistent header and tab bar. Tabs 1–5 also show the live stream.
func TestRenderAllTabs(t *testing.T) {
	tabs := []Tab{
		TabIncident, TabTimeline, TabGraph, TabFailures, TabResources, TabEvidence,
	}
	for _, tab := range tabs {
		m := newStaticModel(engine.DemoState())
		m.ui.width = 120
		m.ui.height = 40
		m.ui.tab = tab
		m.ui.streamMode = invmodel.StreamRanked
		out := m.View()
		if strings.TrimSpace(out) == "" {
			t.Fatalf("tab %d rendered empty", tab)
		}
		if !strings.Contains(out, "KLEW") {
			t.Fatalf("tab %d missing header", tab)
		}
		if tab == TabEvidence {
			if strings.Contains(out, "Kubernetes Live Evidence Window") {
				t.Fatalf("evidence tab should not show live evidence panel")
			}
			if !strings.Contains(out, "Evidence —") {
				t.Fatalf("evidence tab missing title banner")
			}
			continue
		}
		if tab == TabGraph {
			if strings.Contains(out, "Kubernetes Live Evidence Window") {
				t.Fatalf("graph tab should not show live evidence panel")
			}
			if !strings.Contains(out, "Graph · Workload Map") {
				t.Fatalf("graph tab missing workload map panel")
			}
			continue
		}
		if !strings.Contains(out, "Kubernetes Live Evidence Window") {
			t.Fatalf("tab %d missing live evidence panel", tab)
		}
	}
}

func TestViewFitsTerminalHeight(t *testing.T) {
	tabs := []Tab{
		TabIncident, TabTimeline, TabGraph, TabFailures, TabResources, TabEvidence,
	}
	for _, h := range []int{24, 32, 40, 48} {
		for _, tab := range tabs {
			m := newStaticModel(engine.DemoState())
			m.ui.width = 120
			m.ui.height = h
			m.ui.tab = tab
			m.ui.streamMode = invmodel.StreamRanked
			out := m.View()
			lines := strings.Split(out, "\n")
			if got := len(lines); got != h {
				t.Fatalf("tab %d height %d: rendered %d lines, want %d", tab, h, got, h)
			}
		}
	}
}

func TestStreamModeToggle(t *testing.T) {
	m := newStaticModel(engine.DemoState())
	m.ui.width = 120
	m.ui.height = 40
	start := m.ui.streamMode
	if m.ui.streamMode.Next() == start {
		t.Fatal("stream mode should cycle")
	}
}
