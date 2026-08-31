package service

import (
	"context"
	"fmt"
	"time"

	"github.com/glnreddy421/klew/internal/api"
	"github.com/glnreddy421/klew/internal/details"
	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// Service is the shared investigation backend for the desktop app.
type Service struct {
	session *engine.LiveSession
}

// Start begins a live cluster investigation.
func Start(ctx context.Context, client *kube.Client, opts engine.LiveOptions) (*Service, error) {
	session, err := engine.StartLive(ctx, client, opts)
	if err != nil {
		return nil, err
	}
	return &Service{session: session}, nil
}

// Wrap returns a Service around an existing live session (tests).
func Wrap(session *engine.LiveSession) *Service {
	if session == nil {
		return nil
	}
	return &Service{session: session}
}

func (s *Service) State() model.InvestigationState {
	if s == nil || s.session == nil {
		return model.InvestigationState{}
	}
	return s.session.State()
}

// Client returns the live Kubernetes client for the active session.
func (s *Service) Client() *kube.Client {
	if s == nil || s.session == nil {
		return nil
	}
	return s.session.Client()
}

// ObjectDetails builds a kind-aware inspector payload for one object.
func (s *Service) ObjectDetails(ctx context.Context, kind, name, namespace string) (*details.ObjectDetail, error) {
	if s == nil || s.session == nil {
		return nil, fmt.Errorf("no active investigation")
	}
	st := s.session.State()
	return details.Build(ctx, &details.Request{
		Client:   s.session.Client(),
		Snapshot: st.Snapshot,
		State:    st,
		Ref: model.ObjectRef{
			Kind:      kind,
			Name:      name,
			Namespace: namespace,
		},
	})
}

// View returns the capped API view used by desktop React.
func (s *Service) View() api.View {
	return api.Build(s.State())
}

func (s *Service) Pause(v bool) {
	if s == nil || s.session == nil {
		return
	}
	s.session.Pause(v)
}

func (s *Service) Stop() {
	if s != nil && s.session != nil {
		s.session.Stop()
	}
}

func (s *Service) SetAutoRefresh(v bool) {
	if s == nil || s.session == nil {
		return
	}
	s.session.SetAutoRefresh(v)
}

func (s *Service) AutoRefresh() bool {
	if s == nil || s.session == nil {
		return false
	}
	return s.session.AutoRefresh()
}

func (s *Service) SetPollEvery(d time.Duration) {
	if s == nil || s.session == nil {
		return
	}
	s.session.SetPollEvery(d)
}

func (s *Service) PollInterval() time.Duration {
	if s == nil || s.session == nil {
		return 0
	}
	return s.session.PollInterval()
}

func (s *Service) LogTailActive() bool {
	if s == nil || s.session == nil {
		return false
	}
	return s.session.LogTailActive()
}

func (s *Service) LogTailEngaged() bool {
	if s == nil || s.session == nil {
		return false
	}
	return s.session.LogTailEngaged()
}

func (s *Service) LogTailPaused() bool {
	if s == nil || s.session == nil {
		return false
	}
	return s.session.LogTailPaused()
}

func (s *Service) StartLogTail(opts engine.LogTailOptions) error {
	if s == nil || s.session == nil {
		return fmt.Errorf("no active investigation")
	}
	return s.session.StartLogTail(opts)
}

func (s *Service) StopLogTail() {
	if s == nil || s.session == nil {
		return
	}
	s.session.StopLogTail()
}

func (s *Service) PauseLogTail() error {
	if s == nil || s.session == nil {
		return fmt.Errorf("no active investigation")
	}
	return s.session.PauseLogTail()
}

func (s *Service) ResumeLogTail() error {
	if s == nil || s.session == nil {
		return fmt.Errorf("no active investigation")
	}
	return s.session.ResumeLogTail()
}

func (s *Service) ClearLogs() {
	if s == nil || s.session == nil {
		return
	}
	s.session.ClearLogs()
}

// Watch invokes fn whenever investigation state changes until ctx is cancelled.
// Polls at a UI-friendly cadence — sub-second log bursts must not push a full
// state snapshot to the desktop on every line.
func (s *Service) Watch(ctx context.Context, fn func(model.InvestigationState)) {
	if s == nil || s.session == nil || fn == nil {
		return
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var last model.Timestamp
	emit := func() {
		st := s.State()
		if st.LastUpdatedAt.Equal(last) && !last.IsZero() {
			return
		}
		last = st.LastUpdatedAt
		fn(st)
	}
	emit()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			emit()
		}
	}
}
