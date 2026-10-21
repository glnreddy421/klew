package main

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/glnreddy421/klew/internal/history"
	"github.com/glnreddy421/klew/internal/kube"
)

// eventHistory owns this window's local event recorder. Each Klew window is a
// separate process; per-context lock files make sure only one of them records
// a given cluster while all of them read the shared database.
type eventHistory struct {
	mgr *history.Manager

	emitMu      sync.Mutex
	emitPending *time.Timer
}

// EventHistorySettings is the Settings → Investigation → Event history payload.
type EventHistorySettings struct {
	Available     bool   `json:"available"`
	Enabled       bool   `json:"enabled"`
	RetentionDays int    `json:"retentionDays"`
	MaxMB         int    `json:"maxMB"`
	SizeBytes     int64  `json:"sizeBytes"`
	Path          string `json:"path,omitempty"`
	Error         string `json:"error,omitempty"`
}

func (a *App) startEventHistory() {
	dir := kube.DataDir()
	if dir == "" {
		return
	}
	eh := &eventHistory{}
	mgr, err := history.NewManager(history.ManagerConfig{
		Dir:      filepath.Join(dir, "history"),
		Clients:  a.historyClient,
		OnChange: a.emitEventHistorySoon,
	})
	if err != nil {
		a.historyErr = err.Error()
		return
	}
	eh.mgr = mgr
	a.history = eh
}

func (a *App) stopEventHistory() {
	if a.history != nil && a.history.mgr != nil {
		a.history.mgr.Close()
	}
}

func (a *App) historyManager() *history.Manager {
	if a.history == nil {
		return nil
	}
	return a.history.mgr
}

// attachHistory lets investigation and inspector reads include recorded events.
func (a *App) attachHistory(c *kube.Client) {
	if c == nil {
		return
	}
	if m := a.historyManager(); m != nil {
		c.History = m
	}
}

// historyClient builds a recording client for a context from the current
// kubeconfig (multi-file KUBECONFIG and proxy env are honoured by NewFromFlags).
func (a *App) historyClient(contextName string) (history.ClusterTarget, error) {
	a.mu.Lock()
	kcfg := a.cluster.KubeconfigPath
	selected := a.cluster.SelectedContext
	selectedNS := a.cluster.SelectedNamespace
	a.mu.Unlock()
	c, err := kube.NewFromFlags(kcfg, contextName, "")
	if err != nil {
		return history.ClusterTarget{}, err
	}
	fallback := []string{c.ContextNamespace}
	if contextName == selected {
		fallback = append(fallback, selectedNS)
	}
	fallback = append(fallback, "default")
	return history.ClusterTarget{Client: c.Clientset, ServerURL: c.Config.Host, FallbackNamespaces: fallback}, nil
}

// syncHistoryActive tells the recorder which context this window shows.
func (a *App) syncHistoryActive() {
	m := a.historyManager()
	if m == nil {
		return
	}
	a.mu.Lock()
	name := a.cluster.SelectedContext
	if name == "" {
		name = a.cluster.CurrentContext
	}
	a.mu.Unlock()
	m.SetActiveContext(name)
}

func (a *App) emitEventHistorySoon() {
	eh := a.history
	if eh == nil || a.ctx == nil {
		return
	}
	eh.emitMu.Lock()
	defer eh.emitMu.Unlock()
	if eh.emitPending != nil {
		return
	}
	eh.emitPending = time.AfterFunc(500*time.Millisecond, func() {
		eh.emitMu.Lock()
		eh.emitPending = nil
		eh.emitMu.Unlock()
		// Signal only: the UI refetches with its own context list.
		runtime.EventsEmit(a.ctx, "eventHistory")
	})
}

// SetRecordedContexts sets the favorite contexts recorded in the background.
// The frontend calls it on launch and whenever favorites change.
func (a *App) SetRecordedContexts(names []string) []history.ContextStatus {
	if m := a.historyManager(); m != nil {
		m.SetFavoriteContexts(names)
	}
	return a.GetEventHistoryStatus(nil)
}

// SetEventRecordingPaused mirrors the UI's "monitoring paused" state.
func (a *App) SetEventRecordingPaused(paused bool) {
	if m := a.historyManager(); m != nil {
		m.SetPaused(paused)
	}
}

// GetEventHistoryStatus returns per-context recording state for the active
// context, favorites, the given contexts and any context with retained history.
func (a *App) GetEventHistoryStatus(contexts []string) []history.ContextStatus {
	m := a.historyManager()
	if m == nil {
		out := make([]history.ContextStatus, 0, len(contexts))
		for _, c := range contexts {
			out = append(out, history.ContextStatus{Context: c, State: history.StateNotRecordable, Reason: "event history unavailable"})
		}
		return out
	}
	return m.Statuses(contexts)
}

// GetEventHistorySettings returns recording settings and the current size.
func (a *App) GetEventHistorySettings() EventHistorySettings {
	m := a.historyManager()
	if m == nil {
		d := history.DefaultSettings()
		return EventHistorySettings{Enabled: d.Enabled, RetentionDays: d.RetentionDays, MaxMB: d.MaxMB, Error: a.historyErr}
	}
	s := m.Settings()
	return EventHistorySettings{
		Available: true, Enabled: s.Enabled, RetentionDays: s.RetentionDays, MaxMB: s.MaxMB,
		SizeBytes: m.SizeBytes(), Path: m.Store().Path(),
	}
}

// SetEventHistorySettings saves recording settings (shared by all windows).
func (a *App) SetEventHistorySettings(enabled bool, retentionDays, maxMB int) EventHistorySettings {
	if m := a.historyManager(); m != nil {
		_, _ = m.SetSettings(history.Settings{Enabled: enabled, RetentionDays: retentionDays, MaxMB: maxMB})
	}
	return a.GetEventHistorySettings()
}

// ClearEventHistory deletes all locally recorded events and coverage.
func (a *App) ClearEventHistory() (EventHistorySettings, error) {
	m := a.historyManager()
	if m == nil {
		return a.GetEventHistorySettings(), nil
	}
	err := m.Clear()
	return a.GetEventHistorySettings(), err
}
