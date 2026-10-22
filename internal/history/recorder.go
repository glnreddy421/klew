package history

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// Recorder states reported to the UI.
const (
	StateStarting      = "starting"
	StateRecording     = "recording"
	StateReconnecting  = "reconnecting"
	StateNotRecordable = "not_recordable"
	StatePaused        = "paused"
	StateOff           = "off"
	StateOtherWindow   = "other_window"
)

// Recording scopes.
const (
	ScopeCluster    = "cluster"
	ScopeNamespaces = "namespaces"
)

// RecorderStatus is a recorder's current condition.
type RecorderStatus struct {
	ClusterKey string `json:"clusterKey"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
	API        string `json:"api,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Namespaces int    `json:"namespaces,omitempty"`
	// HighVolume means only Warning events are being kept.
	HighVolume bool      `json:"highVolume,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// RecorderConfig configures one cluster's recorder.
type RecorderConfig struct {
	ClusterKey string
	Client     kubernetes.Interface
	Store      *Store
	// FallbackNamespaces are tried when the namespace list itself is denied
	// (typically the kubeconfig context namespace and the selected namespace).
	FallbackNamespaces []string
	MaxNamespaces      int
	FlushInterval      time.Duration
	FlushRows          int
	// HighVolumePerSec switches to Warnings-only above this sustained rate.
	HighVolumePerSec int
	VolumeWindow     time.Duration
	MinBackoff       time.Duration
	MaxBackoff       time.Duration
	// DeniedRetry is how long to wait before re-probing after RBAC denial.
	DeniedRetry       time.Duration
	CoverageHeartbeat time.Duration
	OnStatus          func(RecorderStatus)
	Now               func() time.Time
	// SleptSince reports whether the system slept after t (default: macOS
	// kernel wake time). Used to label gaps caused by sleep.
	SleptSince func(time.Time) bool
}

func (c *RecorderConfig) defaults() {
	if c.MaxNamespaces <= 0 {
		c.MaxNamespaces = 50
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = time.Second
	}
	if c.FlushRows <= 0 {
		c.FlushRows = 500
	}
	if c.HighVolumePerSec <= 0 {
		c.HighVolumePerSec = 200
	}
	if c.VolumeWindow <= 0 {
		c.VolumeWindow = 10 * time.Second
	}
	if c.MinBackoff <= 0 {
		c.MinBackoff = time.Second
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 30 * time.Second
	}
	if c.DeniedRetry <= 0 {
		c.DeniedRetry = 5 * time.Minute
	}
	if c.CoverageHeartbeat <= 0 {
		c.CoverageHeartbeat = 30 * time.Second
	}
	if c.Now == nil {
		c.Now = func() time.Time { return time.Now().UTC() }
	}
	if c.SleptSince == nil {
		c.SleptSince = systemSleptSince
	}
}

// Recorder lists and watches Events for one cluster and writes them to the
// Store. It only ever calls List and Watch.
type Recorder struct {
	cfg RecorderConfig

	mu        sync.Mutex
	status    RecorderStatus
	live      int // established watch streams
	sessionID int64
	// sessionStart/sessionBackfilled describe the open coverage session.
	sessionStart      time.Time
	sessionBackfilled bool
	lastTouch         time.Time
	// stopReason is recorded as the session's end reason when Run exits.
	stopReason string
	// resync asks runPlan to drop and re-establish every stream.
	resync     chan struct{}
	writes     chan writeItem
	volume     volumeGuard
	streamErrs map[string]error
}

type writeItem struct {
	row   *EventRow
	state *WatchState
}

// NewRecorder builds a recorder; call Run to start it.
func NewRecorder(cfg RecorderConfig) *Recorder {
	cfg.defaults()
	r := &Recorder{cfg: cfg, resync: make(chan struct{}, 1)}
	r.status = RecorderStatus{ClusterKey: cfg.ClusterKey, State: StateStarting, UpdatedAt: cfg.Now()}
	r.volume = volumeGuard{window: cfg.VolumeWindow, high: cfg.HighVolumePerSec}
	return r
}

// Status returns the current status.
func (r *Recorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *Recorder) setStatus(mut func(*RecorderStatus)) {
	r.mu.Lock()
	prev := r.status
	mut(&r.status)
	changed := r.status.State != prev.State || r.status.Reason != prev.Reason ||
		r.status.API != prev.API || r.status.Scope != prev.Scope ||
		r.status.Namespaces != prev.Namespaces || r.status.HighVolume != prev.HighVolume
	if changed {
		r.status.UpdatedAt = r.cfg.Now()
	}
	st := r.status
	r.mu.Unlock()
	if changed && r.cfg.OnStatus != nil {
		r.cfg.OnStatus(st)
	}
}

// Run records until ctx is cancelled. Pending writes are flushed on exit.
func (r *Recorder) Run(ctx context.Context) {
	r.writes = make(chan writeItem, 4096)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); r.writer(ctx) }()
	go func() { defer wg.Done(); r.coverageLoop(ctx) }()

	backoff := r.cfg.MinBackoff
	for ctx.Err() == nil {
		plan, err := r.probe(ctx)
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			if isDenied(err) {
				r.setStatus(func(s *RecorderStatus) {
					s.State, s.Reason = StateNotRecordable, deniedReason(err)
				})
				sleepCtx(ctx, r.cfg.DeniedRetry)
				continue
			}
			r.setStatus(func(s *RecorderStatus) { s.State, s.Reason = StateReconnecting, connReason(err) })
			sleepCtx(ctx, backoff)
			backoff = nextBackoff(backoff, r.cfg.MaxBackoff)
			continue
		}
		backoff = r.cfg.MinBackoff
		r.setStatus(func(s *RecorderStatus) {
			s.API, s.Scope, s.Namespaces = plan.api, plan.scope, len(plan.namespaces)
			if plan.scope == ScopeNamespaces {
				s.Reason = fmt.Sprintf("cluster-wide events denied; recording %d namespace(s)", len(plan.namespaces))
			} else {
				s.Reason = ""
			}
		})
		if err := r.runPlan(ctx, plan); err != nil && ctx.Err() == nil {
			// Every stream was denied after a successful probe (e.g. list is
			// allowed but watch is not). Re-probing at once would relist the
			// whole cluster in a tight loop.
			r.setStatus(func(s *RecorderStatus) {
				s.State, s.Reason = StateNotRecordable, deniedReason(err)
			})
			sleepCtx(ctx, r.cfg.DeniedRetry)
		}
	}
	r.finishCoverage()
	close(r.writes)
	wg.Wait()
}

type recordPlan struct {
	api        string
	scope      string
	namespaces []string // "" = cluster-wide
}

// probe picks the API and scope: events.k8s.io/v1 cluster-wide, then core/v1
// cluster-wide, then per-namespace for namespaces the user can list.
func (r *Recorder) probe(ctx context.Context) (recordPlan, error) {
	var firstDenied error
	for _, api := range []string{APIEventsV1, APICoreV1} {
		err := r.probeList(ctx, api, "")
		if err == nil {
			return recordPlan{api: api, scope: ScopeCluster, namespaces: []string{""}}, nil
		}
		if apierrors.IsNotFound(err) || isDenied(err) {
			if firstDenied == nil && isDenied(err) {
				firstDenied = err
			}
			continue
		}
		return recordPlan{}, err
	}

	nsList, err := r.candidateNamespaces(ctx)
	if err != nil && !isDenied(err) {
		return recordPlan{}, err
	}
	for _, api := range []string{APIEventsV1, APICoreV1} {
		var allowed []string
		for _, ns := range nsList {
			if err := r.probeList(ctx, api, ns); err == nil {
				allowed = append(allowed, ns)
			} else if !isDenied(err) && !apierrors.IsNotFound(err) {
				return recordPlan{}, err
			}
		}
		if len(allowed) > 0 {
			return recordPlan{api: api, scope: ScopeNamespaces, namespaces: allowed}, nil
		}
	}
	if firstDenied == nil {
		firstDenied = apierrors.NewForbidden(eventsv1.Resource("events"), "", errors.New("no namespaces available"))
	}
	return recordPlan{}, firstDenied
}

func (r *Recorder) probeList(ctx context.Context, api, ns string) error {
	opts := metav1.ListOptions{Limit: 1}
	var err error
	if api == APIEventsV1 {
		_, err = r.cfg.Client.EventsV1().Events(ns).List(ctx, opts)
	} else {
		_, err = r.cfg.Client.CoreV1().Events(ns).List(ctx, opts)
	}
	return err
}

func (r *Recorder) candidateNamespaces(ctx context.Context) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(ns string) {
		ns = strings.TrimSpace(ns)
		if ns == "" || seen[ns] || len(out) >= r.cfg.MaxNamespaces {
			return
		}
		seen[ns] = true
		out = append(out, ns)
	}
	for _, ns := range r.cfg.FallbackNamespaces {
		add(ns)
	}
	list, err := r.cfg.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return out, err
	}
	names := make([]string, 0, len(list.Items))
	for _, n := range list.Items {
		names = append(names, n.Name)
	}
	sort.Strings(names)
	for _, n := range names {
		add(n)
	}
	return out, nil
}

// runPlan streams every namespace in the plan until ctx ends or every stream
// gives up with a permission error (then the caller re-probes).
// Returns the permission error when every stream was denied.
func (r *Recorder) runPlan(ctx context.Context, plan recordPlan) error {
	planCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var deniedMu sync.Mutex
	denied := 0
	var deniedErr error
	for _, ns := range plan.namespaces {
		wg.Add(1)
		go func(ns string) {
			defer wg.Done()
			if err := r.stream(planCtx, plan.api, ns); err != nil && isDenied(err) {
				deniedMu.Lock()
				denied++
				deniedErr = err
				all := denied == len(plan.namespaces)
				deniedMu.Unlock()
				if all {
					cancel()
				}
			}
		}(ns)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-r.resync:
		// Clock jump (sleep): the connection may be stale. Re-establish so the
		// list or resumed watch backfills what the API server still holds.
		cancel()
		<-done
		return nil
	}
	if denied == len(plan.namespaces) {
		return deniedErr
	}
	return nil
}

// stream keeps one list+watch alive. Returns only on ctx end or a permission
// error (RBAC changed under us).
func (r *Recorder) stream(ctx context.Context, api, ns string) error {
	rv := ""
	needList := true
	if st, ok, err := r.cfg.Store.LoadWatchState(ctx, r.cfg.ClusterKey, ns); err == nil && ok && st.API == api {
		rv = st.ResourceVersion
		needList = rv == ""
	}
	backoff := r.cfg.MinBackoff
	for ctx.Err() == nil {
		var err error
		listed := needList
		if needList {
			rv, err = r.relist(ctx, api, ns)
			if err == nil {
				needList = false
			}
		}
		established := false
		if err == nil {
			established, rv, err = r.watch(ctx, api, ns, rv)
		}
		if ctx.Err() != nil {
			return nil
		}
		if isExpired(err) {
			// Resume point compacted away: relist (idempotent upsert) and rewatch.
			needList = true
			if !listed {
				continue
			}
		}
		if isDenied(err) {
			return err
		}
		if established {
			backoff = r.cfg.MinBackoff
		}
		if err != nil && r.liveCount() == 0 {
			r.setStatus(func(s *RecorderStatus) { s.State, s.Reason = StateReconnecting, connReason(err) })
		}
		sleepCtx(ctx, backoff)
		if !established {
			backoff = nextBackoff(backoff, r.cfg.MaxBackoff)
		}
	}
	return nil
}

func (r *Recorder) relist(ctx context.Context, api, ns string) (string, error) {
	opts := metav1.ListOptions{Limit: 500}
	now := r.cfg.Now()
	var rv string
	for {
		var next string
		if api == APIEventsV1 {
			list, err := r.cfg.Client.EventsV1().Events(ns).List(ctx, opts)
			if err != nil {
				return "", err
			}
			for i := range list.Items {
				row := rowFromEventsV1(r.cfg.ClusterKey, &list.Items[i], now)
				r.enqueue(writeItem{row: &row})
			}
			rv, next = list.ResourceVersion, list.Continue
		} else {
			list, err := r.cfg.Client.CoreV1().Events(ns).List(ctx, opts)
			if err != nil {
				return "", err
			}
			for i := range list.Items {
				row := rowFromCore(r.cfg.ClusterKey, &list.Items[i], now)
				r.enqueue(writeItem{row: &row})
			}
			rv, next = list.ResourceVersion, list.Continue
		}
		if next == "" {
			break
		}
		opts.Continue = next
	}
	r.saveRV(api, ns, rv)
	return rv, nil
}

func (r *Recorder) watch(ctx context.Context, api, ns, rv string) (bool, string, error) {
	opts := metav1.ListOptions{ResourceVersion: rv, AllowWatchBookmarks: true}
	var wi watch.Interface
	var err error
	if api == APIEventsV1 {
		wi, err = r.cfg.Client.EventsV1().Events(ns).Watch(ctx, opts)
	} else {
		wi, err = r.cfg.Client.CoreV1().Events(ns).Watch(ctx, opts)
	}
	if err != nil {
		return false, rv, err
	}
	defer wi.Stop()
	r.streamUp()
	defer r.streamDown()

	for {
		select {
		case <-ctx.Done():
			return true, rv, nil
		case ev, ok := <-wi.ResultChan():
			if !ok {
				return true, rv, errors.New("event watch closed")
			}
			switch ev.Type {
			case watch.Error:
				if st, ok := ev.Object.(*metav1.Status); ok {
					return true, rv, apierrors.FromObject(st)
				}
				return true, rv, errors.New("event watch error")
			case watch.Deleted:
				// TTL expiry on the server; history keeps the event.
				if m, err := metaRV(ev.Object); err == nil && m != "" {
					rv = m
				}
				continue
			case watch.Bookmark:
				if m, err := metaRV(ev.Object); err == nil && m != "" {
					rv = m
					r.saveRV(api, ns, rv)
				}
				continue
			}
			now := r.cfg.Now()
			warningsOnly := r.observeVolume(now)
			var row EventRow
			switch e := ev.Object.(type) {
			case *eventsv1.Event:
				row = rowFromEventsV1(r.cfg.ClusterKey, e, now)
				rv = e.ResourceVersion
			case *corev1.Event:
				row = rowFromCore(r.cfg.ClusterKey, e, now)
				rv = e.ResourceVersion
			default:
				continue
			}
			if warningsOnly && row.Type != corev1.EventTypeWarning {
				r.saveRV(api, ns, rv)
				continue
			}
			r.enqueue(writeItem{row: &row})
			r.saveRV(api, ns, rv)
		}
	}
}

func metaRV(obj any) (string, error) {
	type rvGetter interface{ GetResourceVersion() string }
	if g, ok := obj.(rvGetter); ok {
		return g.GetResourceVersion(), nil
	}
	return "", errors.New("no resourceVersion")
}

func (r *Recorder) saveRV(api, ns, rv string) {
	if rv == "" {
		return
	}
	r.enqueue(writeItem{state: &WatchState{
		ClusterKey: r.cfg.ClusterKey, Namespace: ns, API: api, ResourceVersion: rv, UpdatedAt: r.cfg.Now(),
	}})
}

func (r *Recorder) enqueue(it writeItem) {
	r.writes <- it
}

// maxPendingRows bounds memory while the database is unwritable.
const maxPendingRows = 50000

// writer batches rows: flush every FlushInterval or FlushRows, whichever first.
// A failed batch stays pending and is retried: the watch has already moved
// past those events, so dropping them would lose them for good. Rows and the
// resume point are written in one transaction, so the saved resourceVersion
// never runs ahead of stored events.
func (r *Recorder) writer(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.FlushInterval)
	defer ticker.Stop()
	rows := map[string]EventRow{}
	states := map[string]WatchState{}
	var order []string
	failing := false
	reset := func() {
		rows = map[string]EventRow{}
		states = map[string]WatchState{}
		order = order[:0]
	}
	flush := func(fctx context.Context) {
		if len(rows) == 0 && len(states) == 0 {
			return
		}
		batch := make([]EventRow, 0, len(order))
		for _, uid := range order {
			batch = append(batch, rows[uid])
		}
		ws := make([]WatchState, 0, len(states))
		for _, st := range states {
			ws = append(ws, st)
		}
		if err := r.cfg.Store.WriteBatch(fctx, batch, ws); err != nil {
			failing = true
			if len(rows) > maxPendingRows {
				// Bound memory: this backlog is lost. Only reached after the
				// database has rejected writes for a long time.
				reset()
			}
			return
		}
		failing = false
		reset()
	}
	for {
		select {
		case it, ok := <-r.writes:
			if !ok {
				fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				flush(fctx)
				cancel()
				return
			}
			if it.row != nil {
				if _, exists := rows[it.row.UID]; !exists {
					order = append(order, it.row.UID)
				}
				rows[it.row.UID] = *it.row
			}
			if it.state != nil {
				states[it.state.Namespace] = *it.state
			}
			// While the database is failing, retry only on the ticker.
			if len(rows) >= r.cfg.FlushRows && !failing {
				flush(context.Background())
			}
		case <-ticker.C:
			flush(context.Background())
		}
	}
}

func (r *Recorder) liveCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.live
}

func (r *Recorder) streamUp() {
	r.mu.Lock()
	r.live++
	r.mu.Unlock()
	r.coverageEstablished()
	r.setStatus(func(s *RecorderStatus) {
		s.State = StateRecording
		if s.Scope == ScopeCluster {
			s.Reason = ""
		}
	})
}

func (r *Recorder) streamDown() {
	r.mu.Lock()
	r.live--
	if r.live < 0 {
		r.live = 0
	}
	r.mu.Unlock()
}

// SetStopReason sets the end reason recorded when Run exits (EndKlewClosed, …).
func (r *Recorder) SetStopReason(reason string) {
	r.mu.Lock()
	r.stopReason = reason
	r.mu.Unlock()
}

func (r *Recorder) coverageLoop(ctx context.Context) {
	t := time.NewTicker(r.cfg.CoverageHeartbeat)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.coverageTick()
		}
	}
}

func storeCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// coverageEstablished runs whenever a stream (re)establishes. Every
// establishment follows a successful list or a watch resumed from a known
// resourceVersion, so the API server's retained events were captured: a new
// session is backfilled, and a session opened moments ago (after a clock jump)
// earns the backfill now.
func (r *Recorder) coverageEstablished() {
	now := r.cfg.Now()
	r.mu.Lock()
	id, last, started, backfilled := r.sessionID, r.lastTouch, r.sessionStart, r.sessionBackfilled
	scope := r.status.Scope
	r.mu.Unlock()
	ctx, cancel := storeCtx()
	defer cancel()

	if id != 0 && now.Sub(last) <= GapTolerance {
		if err := r.cfg.Store.TouchSession(ctx, id, now); err == nil {
			if !backfilled && now.Sub(started) <= GapTolerance {
				if r.cfg.Store.MarkBackfilled(ctx, id) == nil {
					backfilled = true
				}
			}
			r.mu.Lock()
			r.lastTouch, r.sessionBackfilled = now, backfilled
			r.mu.Unlock()
			return
		}
	}
	r.beginSession(ctx, scope, now, true)
}

// coverageTick is the heartbeat. Time is wall-clock (Now strips the monotonic
// reading), so a sleep shows up as a late tick.
func (r *Recorder) coverageTick() {
	now := r.cfg.Now()
	r.mu.Lock()
	live, id, last := r.live, r.sessionID, r.lastTouch
	scope, state := r.status.Scope, r.status.State
	r.mu.Unlock()
	ctx, cancel := storeCtx()
	defer cancel()

	if live == 0 {
		// Streams down past the tolerance: the session ended at its last touch.
		if id != 0 && now.Sub(last) > GapTolerance {
			reason := EndUnreachable
			if state == StateNotRecordable {
				reason = EndRBACDenied
			}
			_ = r.cfg.Store.SetEndReason(ctx, id, reason)
			r.mu.Lock()
			r.sessionID = 0
			r.mu.Unlock()
		}
		return
	}
	if id == 0 {
		r.beginSession(ctx, scope, now, false)
		return
	}
	if now.Sub(last) > GapTolerance {
		// Clock jump while "live": the old session ends at its last touch. It is
		// labelled sleep only when the OS confirms a wake since then (a stalled
		// or throttled process stays unknown). Start a new session and force a
		// resync; the re-establishment earns the backfill.
		if r.cfg.SleptSince(last) {
			_ = r.cfg.Store.SetEndReason(ctx, id, EndSystemSleep)
		}
		r.beginSession(ctx, scope, now, false)
		select {
		case r.resync <- struct{}{}:
		default:
		}
		return
	}
	if err := r.cfg.Store.TouchSession(ctx, id, now); errors.Is(err, ErrNoSession) {
		r.beginSession(ctx, scope, now, false) // history was cleared
	} else if err == nil {
		r.mu.Lock()
		r.lastTouch = now
		r.mu.Unlock()
	}
}

func (r *Recorder) beginSession(ctx context.Context, scope string, now time.Time, backfilled bool) {
	id, err := r.cfg.Store.BeginSession(ctx, r.cfg.ClusterKey, scope, now, backfilled)
	if err != nil {
		return
	}
	r.mu.Lock()
	r.sessionID, r.sessionStart, r.sessionBackfilled, r.lastTouch = id, now, backfilled, now
	r.mu.Unlock()
}

// finishCoverage closes the open session with the stop reason. A session that
// already stalled keeps its end time and reason.
func (r *Recorder) finishCoverage() {
	r.mu.Lock()
	id, last, reason := r.sessionID, r.lastTouch, r.stopReason
	r.sessionID = 0
	r.mu.Unlock()
	if id == 0 {
		return
	}
	now := r.cfg.Now()
	if now.Sub(last) > GapTolerance {
		return
	}
	ctx, cancel := storeCtx()
	defer cancel()
	_ = r.cfg.Store.EndSession(ctx, id, now, reason)
}

func (r *Recorder) observeVolume(now time.Time) bool {
	r.mu.Lock()
	changed, high := r.volume.observe(now)
	r.mu.Unlock()
	if changed {
		r.setStatus(func(s *RecorderStatus) { s.HighVolume = high })
	}
	return high
}

// volumeGuard tracks watch event rate in fixed windows. Above `high` events/sec
// it switches to Warnings-only; it switches back below half that rate.
type volumeGuard struct {
	window      time.Duration
	high        int
	windowStart time.Time
	count       int
	warnOnly    bool
}

func (v *volumeGuard) observe(now time.Time) (changed bool, warnOnly bool) {
	if v.windowStart.IsZero() {
		v.windowStart = now
	}
	v.count++
	if el := now.Sub(v.windowStart); el >= v.window {
		rate := float64(v.count) / el.Seconds()
		prev := v.warnOnly
		switch {
		case rate > float64(v.high):
			v.warnOnly = true
		case rate < float64(v.high)/2:
			v.warnOnly = false
		}
		v.windowStart, v.count = now, 0
		return prev != v.warnOnly, v.warnOnly
	}
	return false, v.warnOnly
}

func rowFromEventsV1(key string, e *eventsv1.Event, now time.Time) EventRow {
	first := firstNonZero(e.DeprecatedFirstTimestamp.Time, e.EventTime.Time, e.CreationTimestamp.Time)
	var seriesLast time.Time
	var seriesCount int32
	if e.Series != nil {
		seriesLast = e.Series.LastObservedTime.Time
		seriesCount = e.Series.Count
	}
	last := firstNonZero(seriesLast, e.DeprecatedLastTimestamp.Time, e.EventTime.Time, e.CreationTimestamp.Time)
	count := e.DeprecatedCount
	if count <= 0 {
		count = seriesCount
	}
	ctrl := e.ReportingController
	if ctrl == "" {
		ctrl = e.DeprecatedSource.Component
	}
	return EventRow{
		ClusterKey: key, UID: string(e.UID), Namespace: e.Namespace,
		RegardingKind: e.Regarding.Kind, RegardingName: e.Regarding.Name, RegardingUID: string(e.Regarding.UID),
		Type: e.Type, Reason: e.Reason, Message: e.Note, ReportingController: ctrl, Action: e.Action,
		Count: count, SeriesCount: seriesCount, FirstSeen: first, LastSeen: last, RecordedAt: now,
	}
}

func rowFromCore(key string, e *corev1.Event, now time.Time) EventRow {
	first := firstNonZero(e.FirstTimestamp.Time, e.EventTime.Time, e.CreationTimestamp.Time)
	var seriesLast time.Time
	var seriesCount int32
	if e.Series != nil {
		seriesLast = e.Series.LastObservedTime.Time
		seriesCount = e.Series.Count
	}
	last := firstNonZero(e.LastTimestamp.Time, seriesLast, e.EventTime.Time, e.CreationTimestamp.Time)
	count := e.Count
	if count <= 0 {
		count = seriesCount
	}
	ctrl := e.ReportingController
	if ctrl == "" {
		ctrl = e.Source.Component
	}
	return EventRow{
		ClusterKey: key, UID: string(e.UID), Namespace: e.Namespace,
		RegardingKind: e.InvolvedObject.Kind, RegardingName: e.InvolvedObject.Name, RegardingUID: string(e.InvolvedObject.UID),
		Type: e.Type, Reason: e.Reason, Message: e.Message, ReportingController: ctrl, Action: e.Action,
		Count: count, SeriesCount: seriesCount, FirstSeen: first, LastSeen: last, RecordedAt: now,
	}
}

func firstNonZero(ts ...time.Time) time.Time {
	for _, t := range ts {
		if !t.IsZero() {
			return t.UTC()
		}
	}
	return time.Time{}
}

func isDenied(err error) bool {
	return err != nil && (apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err))
}

func isExpired(err error) bool {
	return err != nil && (apierrors.IsResourceExpired(err) || apierrors.IsGone(err))
}

// deniedReason is a short, message-free reason for the UI.
func deniedReason(err error) string {
	if apierrors.IsUnauthorized(err) {
		return "credentials rejected (401)"
	}
	return "RBAC denies listing events cluster-wide and in every known namespace"
}

// connReason never includes event content; it only classifies the error.
func connReason(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"),
		strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "network is unreachable"),
		strings.Contains(msg, "dial tcp"):
		return "cluster unreachable"
	case strings.Contains(msg, "certificate"), strings.Contains(msg, "tls"):
		return "TLS error"
	case strings.Contains(msg, "watch closed"):
		return "watch interrupted"
	default:
		return "connection error"
	}
}

func nextBackoff(cur, max time.Duration) time.Duration {
	cur *= 2
	if cur > max {
		return max
	}
	return cur
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
