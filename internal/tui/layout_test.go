package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glnreddy421/klew/internal/engine"
)

func TestSplitPanelHeights(t *testing.T) {
	t.Parallel()
	tests := []struct {
		contentH      int
		wantUpperMin  int
		wantStreamMin int
	}{
		{contentH: 20, wantUpperMin: 10, wantStreamMin: 7},
		{contentH: 12, wantUpperMin: 6, wantStreamMin: 4},
		{contentH: 6, wantUpperMin: 2, wantStreamMin: 3},
	}
	for _, tc := range tests {
		upper, stream := splitPanelHeights(tc.contentH, true)
		if upper < tc.wantUpperMin {
			t.Fatalf("contentH=%d: upper=%d want >= %d", tc.contentH, upper, tc.wantUpperMin)
		}
		if stream < tc.wantStreamMin {
			t.Fatalf("contentH=%d: stream=%d want >= %d", tc.contentH, stream, tc.wantStreamMin)
		}
		if upper+stream+1 != tc.contentH {
			t.Fatalf("contentH=%d: upper(%d)+stream(%d)+divider != contentH", tc.contentH, upper, stream)
		}
	}
}

func TestStreamFocusScroll(t *testing.T) {
	m := newStaticModel(engine.DemoState())
	m.ui.width = 120
	m.ui.height = 40
	m.ui.streamFocused = true
	before := m.ui.streamScroll
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m2 := nm.(appModel)
	if m2.ui.streamFollow {
		t.Fatal("expected stream follow disabled after manual scroll")
	}
	if m2.ui.streamScroll <= before {
		t.Fatalf("expected stream scroll to increase, got %d from %d", m2.ui.streamScroll, before)
	}
}
