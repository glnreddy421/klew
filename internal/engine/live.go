package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// LiveOptions configures live investigation.
type LiveOptions struct {
	Query          string
	Namespace      string
	AllNS          bool
	PollEvery      time.Duration
	AutoRefresh    bool
	Window         time.Duration
	Tail           int
	MaxLogRequests int // concurrent pod/container log follows; 0 → kube default
	// DisableMetrics skips the metrics-server probe (requests/limits only).
	DisableMetrics bool
}

// LiveSession runs collectors and exposes investigation state.
type LiveSession struct {
	Reducer *Reducer
	Bus     *Bus
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	refreshMu     sync.Mutex
	autoRefresh   bool
	pollEvery     time.Duration
	client        *kube.Client
	refreshNS     string
	refreshQuery  string
}

// StartLive creates snapshot, starts bus/reducer, watchers, log streams, metrics poll.
func StartLive(ctx context.Context, client *kube.Client, opts LiveOptions) (*LiveSession, error) {
	if opts.PollEvery <= 0 {
		opts.PollEvery = 10 * time.Second
	}
	if opts.Window <= 0 {
		opts.Window = 15 * time.Minute
	}
	if opts.Window > 15*time.Minute {
		opts.Window = 15 * time.Minute
	}
	if opts.Tail <= 0 {
		opts.Tail = 200
	}
	scope := kube.ScopeFromFlags(opts.Namespace, opts.AllNS, client.Namespace)
	ns := scope.Primary
	if scope.AllNamespaces {
		ns = client.Namespace
	}

	ctx, cancel := context.WithCancel(ctx)
	bus := NewBus(1024)
	sink := func(e model.EvidenceEvent) { bus.Publish(e) }

	bundle, _, err := CollectSnapshot(ctx, client, SnapshotOptions{Namespace: ns, Query: opts.Query, AllNS: opts.AllNS, Tail: opts.Tail})
	if err != nil {
		cancel()
		return nil, err
	}
	state := BootstrapState(bundle, scope, opts.Query, model.ModeLive)
	state.Window = model.DurationMS(opts.Window)
	state.TailLines = opts.Tail
	reducer := NewReducer(state)

	session := &LiveSession{
		Reducer:      reducer,
		Bus:          bus,
		cancel:       cancel,
		autoRefresh:  opts.AutoRefresh,
		pollEvery:    opts.PollEvery,
		client:       client,
		refreshNS:    ns,
		refreshQuery: opts.Query,
	}

	// Consumer must run before snapshot publish so seed events are not dropped
	// on a full bus while nobody is draining.
	go bus.RunConsumer(ctx, reducer.ApplyEvent)
	PublishSnapshotEvents(bus, bundle)

	session.wg.Add(1)
	go session.runRefreshLoop(ctx)

	watcher := &kube.LiveWatcher{Client: client, Sink: sink, Namespace: ns, Query: opts.Query}
	watches := watcher.Start(ctx, &session.wg)

	logStreamer := &kube.LogStreamer{
		Client:         client,
		Sink:           sink,
		Namespace:      ns,
		Query:          opts.Query,
		Tail:           opts.Tail,
		Since:          opts.Window,
		MaxLogRequests: opts.MaxLogRequests,
	}
	logStreamer.Start(ctx, bundle.Pods, &session.wg)

	if opts.DisableMetrics {
		sink(model.EvidenceEvent{
			Timestamp:  model.TimestampFrom(time.Now().UTC()),
			SourceType: model.SourceSystem,
			Severity:   model.SeverityInfo,
			Reason:     "metrics_disabled",
			Message:    "Metrics-server probe disabled in settings — showing pod requests/limits only",
			Confidence: 1,
		})
	} else {
		metrics := &kube.MetricsPoller{Client: client, Sink: sink, Namespace: ns}
		metrics.Start(ctx, bundle.Pods, &session.wg)
	}

	reducer.SetWatches(watches)
	return session, nil
}

func (s *LiveSession) State() model.InvestigationState { return s.Reducer.State() }

// Client returns the live Kubernetes client (nil when no cluster is attached).
func (s *LiveSession) Client() *kube.Client {
	if s == nil {
		return nil
	}
	return s.client
}

func (s *LiveSession) Pause(v bool) { s.Reducer.SetPaused(v) }

// SetAutoRefresh toggles periodic cluster snapshot refresh.
func (s *LiveSession) SetAutoRefresh(v bool) {
	s.refreshMu.Lock()
	s.autoRefresh = v
	s.refreshMu.Unlock()
}

// AutoRefresh reports whether snapshot refresh is enabled.
func (s *LiveSession) AutoRefresh() bool {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.autoRefresh
}

// SetPollEvery changes the snapshot refresh interval.
func (s *LiveSession) SetPollEvery(d time.Duration) {
	if d <= 0 {
		return
	}
	s.refreshMu.Lock()
	s.pollEvery = d
	s.refreshMu.Unlock()
}

// PollInterval returns the configured snapshot refresh interval.
func (s *LiveSession) PollInterval() time.Duration {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.pollEvery
}

func (s *LiveSession) runRefreshLoop(ctx context.Context) {
	defer s.wg.Done()
	var lastRefresh time.Time
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshMu.Lock()
			enabled := s.autoRefresh
			interval := s.pollEvery
			client := s.client
			reducer := s.Reducer
			ns := s.refreshNS
			query := s.refreshQuery
			s.refreshMu.Unlock()
			if !enabled || time.Since(lastRefresh) < interval {
				continue
			}
			lastRefresh = time.Now()
			if err := RefreshSnapshot(ctx, client, reducer, ns, query); err != nil {
				slog.Debug("snapshot refresh", "err", err)
			}
		}
	}
}

func (s *LiveSession) Stop() {
	s.cancel()
	s.Bus.Close()
	s.wg.Wait()
}

func RunFor(ctx context.Context, client *kube.Client, opts LiveOptions, duration time.Duration) (model.InvestigationState, error) {
	session, err := StartLive(ctx, client, opts)
	if err != nil {
		return model.InvestigationState{}, err
	}
	defer session.Stop()
	select {
	case <-ctx.Done():
		return session.State(), ctx.Err()
	case <-time.After(duration):
		return session.State(), nil
	}
}
