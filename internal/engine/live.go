package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// LiveOptions configures live investigation.
type LiveOptions struct {
	Query       string
	Namespace   string
	PollEvery   time.Duration
	AutoRefresh bool
	Window      time.Duration
	Tail        int
	// DisableMetrics skips the metrics-server probe (requests/limits only).
	DisableMetrics bool
	// CheckAdmissionPerimeter lists admission webhooks when workloads expect pods but none exist.
	CheckAdmissionPerimeter bool
	// NoLogs skips container logs in snapshots, and the investigation accepts
	// no streamed log lines (see IngestLog).
	NoLogs bool
}

// LiveSession runs collectors and exposes investigation state.
type LiveSession struct {
	Reducer *Reducer
	Bus     *Bus
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	refreshMu         sync.Mutex
	autoRefresh       bool
	pollEvery         time.Duration
	lastStructRefresh time.Time
	client            *kube.Client
	refreshNS         string
	refreshQuery      string

	logMu sync.Mutex
	tail  int
	logNS string
	// Pods whose streamed lines the investigation accepts, refreshed from the
	// snapshot at most once a second (see IngestLog).
	ingestPods   map[string]bool
	ingestPodsAt time.Time

	checkAdmissionPerimeter bool
	noLogs                  bool

	watcher *kube.LiveWatcher
}

// StartLive creates snapshot, starts bus/reducer, watchers, and metrics poll.
// Live log lines arrive only through IngestLog, from the desktop log stream.
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
	scope := kube.ScopeFromFlags(opts.Namespace, client.Namespace)
	ns := scope.Primary

	ctx, cancel := context.WithCancel(ctx)
	bus := NewBus(1024)

	bundle, _, err := CollectSnapshot(ctx, client, SnapshotOptions{
		Namespace:               ns,
		Query:                   opts.Query,
		Tail:                    opts.Tail,
		CheckAdmissionPerimeter: opts.CheckAdmissionPerimeter,
		NoLogs:                  opts.NoLogs,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	state := BootstrapState(bundle, scope, opts.Query, model.ModeLive)
	state.Window = model.DurationMS(opts.Window)
	state.TailLines = opts.Tail
	reducer := NewReducer(state)

	session := &LiveSession{
		Reducer:                 reducer,
		Bus:                     bus,
		ctx:                     ctx,
		cancel:                  cancel,
		autoRefresh:             opts.AutoRefresh,
		pollEvery:               opts.PollEvery,
		client:                  client,
		refreshNS:               ns,
		refreshQuery:            opts.Query,
		tail:                    opts.Tail,
		logNS:                   ns,
		checkAdmissionPerimeter: opts.CheckAdmissionPerimeter,
		noLogs:                  opts.NoLogs,
	}

	sink := func(e model.EvidenceEvent) {
		bus.Publish(e)
		session.noteStructuralChange(e)
	}

	// Consumer must run before snapshot publish so seed events are not dropped
	// on a full bus while nobody is draining.
	go bus.RunConsumer(ctx, reducer.ApplyEvent)
	PublishSnapshotEvents(bus, bundle)

	session.wg.Add(1)
	go session.runRefreshLoop(ctx)

	watcher := &kube.LiveWatcher{Client: client, Sink: sink, Namespace: ns, Query: opts.Query}
	watcher.SetTarget(bundle.Target)
	watcher.SeedEventUIDs(eventUIDs(bundle.Events))
	scopePods := podSummaryNames(bundle.Pods)
	// Set before Start: the sink reads it from watch goroutines.
	session.watcher = watcher
	watches := watcher.Start(ctx, &session.wg, scopePods)

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

func (s *LiveSession) noteStructuralChange(e model.EvidenceEvent) {
	if s == nil || !shouldRefreshOnStructuralChange(e) {
		return
	}
	s.refreshMu.Lock()
	if time.Since(s.lastStructRefresh) < 2*time.Second {
		s.refreshMu.Unlock()
		return
	}
	s.lastStructRefresh = time.Now()
	enabled := s.autoRefresh
	client := s.client
	reducer := s.Reducer
	ns := s.refreshNS
	query := s.refreshQuery
	watcher := s.watcher
	s.refreshMu.Unlock()
	if !enabled || client == nil || reducer == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := RefreshSnapshot(ctx, client, reducer, ns, query, s.snapshotCollectOptions()); err != nil {
			slog.Debug("structural snapshot refresh", "err", err)
			return
		}
		if watcher != nil {
			watcher.SetScopePodNames(podSummaryNames(reducer.State().Snapshot.Pods))
			watcher.SetTarget(reducer.State().Snapshot.Target)
		}
	}()
}

func shouldRefreshOnStructuralChange(e model.EvidenceEvent) bool {
	if structuralObjectReason(e.Reason) || severityRank(e.Severity) >= severityRank(model.SeverityHigh) {
		return true
	}
	if e.SourceType != model.SourceObjectChange {
		return false
	}
	switch e.SourceKind {
	case "Container", "Deployment", "ReplicaSet":
		return true
	case "Pod":
		return strings.Contains(e.Message, "phase=Failed") ||
			strings.Contains(e.Message, "phase=Pending") ||
			strings.Contains(e.Message, "phase=Unknown")
	default:
		return false
	}
}

func structuralObjectReason(reason string) bool {
	switch reason {
	case "BackOff", "CrashLoopBackOff", "OOMKilled", "Error", "Failed",
		"FailedScheduling", "FailedMount", "Unhealthy", "Killing":
		return true
	default:
		return false
	}
}

// RefreshSnapshotNow re-collects workload state immediately (manual refresh).
func (s *LiveSession) RefreshSnapshotNow(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("no live session")
	}
	s.refreshMu.Lock()
	client := s.client
	reducer := s.Reducer
	ns := s.refreshNS
	query := s.refreshQuery
	watcher := s.watcher
	s.lastStructRefresh = time.Now()
	s.refreshMu.Unlock()
	if client == nil || reducer == nil {
		return fmt.Errorf("live session not ready")
	}
	if err := RefreshSnapshot(ctx, client, reducer, ns, query, s.snapshotCollectOptions()); err != nil {
		return err
	}
	if watcher != nil {
		watcher.SetScopePodNames(podSummaryNames(reducer.State().Snapshot.Pods))
		watcher.SetTarget(reducer.State().Snapshot.Target)
	}
	return nil
}

func (s *LiveSession) snapshotCollectOptions() SnapshotOptions {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return SnapshotOptions{
		Namespace:               s.refreshNS,
		Query:                   s.refreshQuery,
		Tail:                    s.tail,
		CheckAdmissionPerimeter: s.checkAdmissionPerimeter,
		NoLogs:                  s.noLogs,
	}
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
			if err := RefreshSnapshot(ctx, client, reducer, ns, query, s.snapshotCollectOptions()); err != nil {
				slog.Debug("snapshot refresh", "err", err)
			} else if s.watcher != nil {
				s.watcher.SetScopePodNames(podSummaryNames(reducer.State().Snapshot.Pods))
				s.watcher.SetTarget(reducer.State().Snapshot.Target)
			}
		}
	}
}

func (s *LiveSession) Stop() {
	s.cancel()
	s.Bus.Close()
	s.wg.Wait()
}

// IngestLog accepts a log line streamed outside the investigation (the
// desktop log stream) when it comes from a pod in the investigation's
// snapshot, so Patterns and signals see it. It reports whether it was used.
// Investigations started without logs accept none.
func (s *LiveSession) IngestLog(e model.EvidenceEvent) bool {
	if s == nil || s.Bus == nil || s.noLogs || e.SourceType != model.SourceLog {
		return false
	}
	if e.Namespace != s.logNS || e.Pod == "" {
		return false
	}
	s.logMu.Lock()
	if s.ingestPods == nil || time.Since(s.ingestPodsAt) > time.Second {
		pods := s.Reducer.State().Snapshot.Pods
		s.ingestPods = make(map[string]bool, len(pods))
		for _, p := range pods {
			s.ingestPods[p.Name] = true
		}
		s.ingestPodsAt = time.Now()
	}
	ok := s.ingestPods[e.Pod]
	s.logMu.Unlock()
	if ok {
		s.Bus.Publish(e)
	}
	return ok
}

func podSummaryNames(pods []model.PodSummary) []string {
	out := make([]string, 0, len(pods))
	for _, p := range pods {
		if p.Name != "" {
			out = append(out, p.Name)
		}
	}
	return out
}

// settleTimeout bounds how long RunFor waits for queued evidence to be applied.
const settleTimeout = 5 * time.Second

// RunFor watches for duration, then returns the state once queued evidence has
// been applied. A zero duration returns the snapshot's verdict without watching.
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
	}
	err = session.settle(ctx)
	return session.State(), err
}

// settle waits until the bus is idle. Watches keep publishing, so a busy
// session may never be; that is reported rather than waited out.
func (s *LiveSession) settle(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, settleTimeout)
	defer cancel()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for !s.Bus.Idle() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("evidence still being processed: %w", ctx.Err())
		case <-tick.C:
		}
	}
	return nil
}

func eventUIDs(events []model.EventRecord) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		if e.UID != "" {
			out = append(out, e.UID)
		}
	}
	return out
}
