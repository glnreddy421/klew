package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	invmodel "github.com/glnreddy421/klew/internal/model"
)

type liveSession interface {
	State() invmodel.InvestigationState
	Pause(bool)
	Stop()
	SetAutoRefresh(bool)
	AutoRefresh() bool
	SetPollEvery(time.Duration)
	PollInterval() time.Duration
}

type tickMsg struct{}
type stateMsg struct{ st invmodel.InvestigationState }

// RunLive opens the TUI bound to a live investigation session.
func RunLive(ctx context.Context, session liveSession) error {
	m := newLiveModel(ctx, session, invmodel.ModeLive)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	session.Stop()
	return err
}

// RunBundle opens the TUI for offline bundle review.
func RunBundle(st invmodel.InvestigationState) error {
	m := newStaticModel(st)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Run legacy static investigation (compat).
func Run(inv invmodel.Investigation) error {
	st := invmodel.InvestigationState{
		Mode:          invmodel.ModeBundle,
		Snapshot:      inv.Bundle,
		Timeline:      inv.Timeline,
		WorkloadGraph: inv.Graph,
		Verdict:       inv.Verdict,
		Query:         inv.Bundle.Query,
		KubeContext:   inv.Bundle.KubeContext,
		NamespaceScope: invmodel.NamespaceScope{Primary: inv.Bundle.Namespace},
	}
	return RunBundle(st)
}

func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}
