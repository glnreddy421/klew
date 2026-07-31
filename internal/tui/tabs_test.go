package tui

import (
	"strings"
	"testing"
)

func TestTabBanner(t *testing.T) {
	line := RenderTabBanner(TabIncident, 120)
	if !strings.Contains(line, "Incident") {
		t.Fatalf("missing title: %q", line)
	}
	if !strings.Contains(line, "What broke") {
		t.Fatalf("missing subtitle: %q", line)
	}
}

func TestTabLabels(t *testing.T) {
	want := []string{"Incident", "Timeline", "Graph", "Failures", "Resources", "Evidence"}
	for i, title := range want {
		if Tab(i).Meta().Title != title {
			t.Fatalf("tab %d title = %q, want %q", i, Tab(i).Meta().Title, title)
		}
	}
}
