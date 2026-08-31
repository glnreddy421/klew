package engine

import (
	"context"
	"fmt"
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

// LogTailOptions configures an on-demand multipod log follow.
type LogTailOptions struct {
	// PodNames limits tailing to these pods. Empty → all pods in the investigation snapshot.
	PodNames []string
}

// LiveSession runs collectors and exposes investigation state.
type LiveSession struct {
	Reducer *Reducer
	Bus     *Bus
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	refreshMu     sync.Mutex
	autoRefresh   bool
	pollEvery     time.Duration
	client        *kube.Client
	refreshNS     string
	refreshQuery  string

	logMu         sync.Mutex
	stopLogTail   func()
	logTailActive bool
	logTailPaused bool
	logTailNames  []string
	tail          int
	window        time.Duration
	maxLogReq     int
	logNS         string

	watcher *kube.LiveWatcher
}

// StartLive creates snapshot, starts bus/reducer, watchers, and metrics poll.
// Live pod tailing starts only when the user requests it via StartLogTail.
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
		ctx:          ctx,
		cancel:       cancel,
		autoRefresh:  opts.AutoRefresh,
		pollEvery:    opts.PollEvery,
		client:       client,
		refreshNS:    ns,
		refreshQuery: opts.Query,
		tail:         opts.Tail,
		window:       opts.Window,
		maxLogReq:    opts.MaxLogRequests,
		logNS:        ns,
	}

	// Consumer must run before snapshot publish so seed events are not dropped
	// on a full bus while nobody is draining.
	go bus.RunConsumer(ctx, reducer.ApplyEvent)
	PublishSnapshotEvents(bus, bundle)

	session.wg.Add(1)
	go session.runRefreshLoop(ctx)

	watcher := &kube.LiveWatcher{Client: client, Sink: sink, Namespace: ns, Query: opts.Query}
	scopePods := podSummaryNames(bundle.Pods)
	watches := watcher.Start(ctx, &session.wg, scopePods)
	session.watcher = watcher

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
			} else if s.watcher != nil {
				s.watcher.SetScopePodNames(podSummaryNames(reducer.State().Snapshot.Pods))
			}
		}
	}
}

func (s *LiveSession) Stop() {
	s.logMu.Lock()
	s.stopLogTailLocked()
	s.logMu.Unlock()
	s.cancel()
	s.Bus.Close()
	s.wg.Wait()
}

// LogTailActive reports whether GetLogs follow streams are open.
func (s *LiveSession) LogTailActive() bool {
	if s == nil {
		return false
	}
	s.logMu.Lock()
	defer s.logMu.Unlock()
	return s.logTailActive
}

// LogTailEngaged reports whether a gather session exists (streaming or paused).
func (s *LiveSession) LogTailEngaged() bool {
	if s == nil || s.Reducer == nil {
		return false
	}
	s.logMu.Lock()
	defer s.logMu.Unlock()
	if s.logTailActive || s.logTailPaused {
		return true
	}
	return len(s.Reducer.State().LogTailPods) > 0
}

// LogTailPaused reports whether log follows are paused with scope retained.
func (s *LiveSession) LogTailPaused() bool {
	if s == nil {
		return false
	}
	s.logMu.Lock()
	defer s.logMu.Unlock()
	return s.logTailPaused
}

// StartLogTail opens GetLogs follows for pods in the investigation snapshot.
func (s *LiveSession) StartLogTail(opts LogTailOptions) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("no active investigation")
	}
	pods := filterSnapshotPods(s.Reducer.State().Snapshot.Pods, opts.PodNames)
	if len(opts.PodNames) == 0 {
		return fmt.Errorf("select at least one pod")
	}
	if len(pods) == 0 {
		return fmt.Errorf("no pods in investigation scope")
	}
	allowNames := podSummaryNames(pods)

	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.stopLogTailLocked()
	s.logTailNames = append([]string(nil), allowNames...)
	return s.startLogStreamsLocked(pods, allowNames)
}

// PauseLogTail closes GetLogs streams but keeps the gather session for resume.
func (s *LiveSession) PauseLogTail() error {
	if s == nil {
		return fmt.Errorf("no active investigation")
	}
	s.logMu.Lock()
	defer s.logMu.Unlock()
	if !s.logTailActive && !s.logTailPaused {
		return fmt.Errorf("no active log gather session")
	}
	if s.logTailPaused {
		return nil
	}
	s.stopStreamsLocked()
	s.logTailPaused = true
	if s.Reducer != nil {
		s.Reducer.SetLogTailPaused(true)
	}
	s.emitLogTailSystem("log_tail_paused", "Log gather paused — GetLogs streams closed to reduce API load")
	return nil
}

// ResumeLogTail reopens GetLogs follows for the last gather selection.
func (s *LiveSession) ResumeLogTail() error {
	if s == nil || s.client == nil {
		return fmt.Errorf("no active investigation")
	}
	s.logMu.Lock()
	defer s.logMu.Unlock()
	if !s.logTailPaused {
		return fmt.Errorf("log gather is not paused")
	}
	names := s.logTailNames
	if len(names) == 0 {
		names = append([]string(nil), s.Reducer.State().LogTailPods...)
	}
	if len(names) == 0 {
		return fmt.Errorf("no pods to resume")
	}
	pods := filterSnapshotPods(s.Reducer.State().Snapshot.Pods, names)
	if len(pods) == 0 {
		return fmt.Errorf("selected pods are no longer in investigation scope")
	}
	allowNames := podSummaryNames(pods)
	s.logTailNames = append([]string(nil), allowNames...)
	if err := s.startLogStreamsLocked(pods, allowNames); err != nil {
		return err
	}
	s.emitLogTailSystem("log_tail_resumed", fmt.Sprintf("Resumed log gather for %d pod(s)", len(pods)))
	return nil
}

// StopLogTail ends active log follows without stopping the investigation.
func (s *LiveSession) StopLogTail() {
	if s == nil {
		return
	}
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.stopLogTailLocked()
}

// ClearLogs wipes log lines for the active tail pods (live panel clean slate).
// Log lines from other pods stay in the buffer for Patterns and other views.
func (s *LiveSession) ClearLogs() {
	if s == nil || s.Reducer == nil {
		return
	}
	pods := s.Reducer.State().LogTailPods
	if len(pods) > 0 {
		s.Reducer.ClearLogsForPods(pods)
		return
	}
	s.Reducer.ClearLogs()
}

func (s *LiveSession) stopLogTailLocked() {
	s.stopStreamsLocked()
	s.logTailPaused = false
	s.logTailNames = nil
	if s.Reducer != nil {
		s.Reducer.SetLogTailPods(nil)
		s.Reducer.SetLogTailPaused(false)
	}
}

func (s *LiveSession) stopStreamsLocked() {
	if s.stopLogTail != nil {
		s.stopLogTail()
		s.stopLogTail = nil
	}
	s.logTailActive = false
}

func (s *LiveSession) startLogStreamsLocked(pods []model.PodSummary, allowNames []string) error {
	if s.client == nil {
		return fmt.Errorf("no cluster client")
	}
	s.stopStreamsLocked()
	s.logTailPaused = false

	s.Reducer.SetLogTailPods(allowNames)
	s.Reducer.SetLogTailPaused(false)

	sink := func(e model.EvidenceEvent) { s.Bus.Publish(e) }
	streamer := &kube.LogStreamer{
		Client:         s.client,
		Sink:           sink,
		Namespace:      s.logNS,
		Query:          "",
		Tail:           s.tail,
		Since:          s.window,
		MaxLogRequests: s.maxLogReq,
	}
	s.stopLogTail = streamer.StartWithStop(s.ctx, pods)
	s.logTailActive = true
	s.emitLogTailSystem("log_tail_started", fmt.Sprintf("Gathering logs from %d pod(s)", len(pods)))
	return nil
}

func (s *LiveSession) emitLogTailSystem(reason, message string) {
	if s.Bus == nil {
		return
	}
	s.Bus.Publish(model.EvidenceEvent{
		Timestamp:  model.TimestampFrom(time.Now().UTC()),
		SourceType: model.SourceSystem,
		Severity:   model.SeverityInfo,
		Reason:     reason,
		Message:    message,
		Confidence: 1,
	})
}

func filterSnapshotPods(pods []model.PodSummary, names []string) []model.PodSummary {
	if len(names) == 0 {
		out := make([]model.PodSummary, len(pods))
		copy(out, pods)
		return out
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		if n != "" {
			want[n] = true
		}
	}
	var out []model.PodSummary
	for _, p := range pods {
		if want[p.Name] {
			out = append(out, p)
		}
	}
	return out
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
