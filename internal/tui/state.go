package tui

import (
	invmodel "github.com/glnreddy421/klew/internal/model"
)

// uiState holds TUI-specific presentation state.
type uiState struct {
	tab          Tab
	scroll       int
	streamScroll  int
	streamFocused bool // when true, j/k scroll the live stream panel
	streamFollow  bool // pinned to newest evidence (scroll=0)
	filter        string
	searching    bool
	streamMode   invmodel.StreamViewMode
	paused       bool
	showHelp     bool
	showSettings bool // full-height settings overlay (S)
	status       string
	width        int
	height       int

	// Timeline-tab-only filters. Kept separate from the shared stream `filter`
	// so timeline filtering never affects the persistent Live Evidence Stream.
	tlCat       string // all | logs | events | metrics | objects | klew
	tlSearch    string
	tlSearching bool
	tlExpand    bool // expand folded repeated events
}

func defaultUIState() uiState {
	return uiState{
		tab:          TabIncident,
		streamMode:   invmodel.StreamRanked,
		streamFollow: true,
		status:       "Ready",
		tlCat:        "all",
	}
}
