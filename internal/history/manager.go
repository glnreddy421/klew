package history

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/glnreddy421/klew/internal/model"
)

// Settings control recording. Stored in history/settings.json and shared by
// every Klew window.
type Settings struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retentionDays"`
	MaxMB         int  `json:"maxMB"`
	// EventTTLMinutes is the cluster's --event-ttl: how long the API server
	// keeps Events, so how far back a fresh list backfills.
	EventTTLMinutes int `json:"eventTTLMinutes"`
}

// DefaultSettings: on, 7 days, 500 MB, 60-minute event TTL.
func DefaultSettings() Settings {
	return Settings{Enabled: true, RetentionDays: 7, MaxMB: 500, EventTTLMinutes: int(DefaultEventTTL / time.Minute)}
}

// EventTTL is EventTTLMinutes as a duration.
func (s Settings) EventTTL() time.Duration {
	return time.Duration(s.EventTTLMinutes) * time.Minute
}

func (s Settings) normalized() Settings {
	if s.EventTTLMinutes <= 0 {
		s.EventTTLMinutes = int(DefaultEventTTL / time.Minute)
	}
	if s.EventTTLMinutes < 5 {
		s.EventTTLMinutes = 5
	}
	if s.EventTTLMinutes > 24*60 {
		s.EventTTLMinutes = 24 * 60
	}
	if s.RetentionDays <= 0 {
		s.RetentionDays = 7
	}
	if s.RetentionDays > 90 {
		s.RetentionDays = 90
	}
	if s.MaxMB <= 0 {
		s.MaxMB = 500
	}
	if s.MaxMB < 50 {
		s.MaxMB = 50
	}
	if s.MaxMB > 10240 {
		s.MaxMB = 10240
	}
	return s
}

// ClusterTarget is what the manager needs to record one context.
type ClusterTarget struct {
	Client             kubernetes.Interface
	ServerURL          string
	FallbackNamespaces []string
}

// ClientFactory resolves a kube context name to a client. It must not contact
// the API server (kubeconfig load only).
type ClientFactory func(contextName string) (ClusterTarget, error)

// ContextStatus is one context's recording state for the UI.
type ContextStatus struct {
	Context    string `json:"context"`
	ClusterKey string `json:"clusterKey,omitempty"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
	Scope      string `json:"scope,omitempty"`
	HighVolume bool   `json:"highVolume,omitempty"`
	Active     bool   `json:"active,omitempty"`
	Favorite   bool   `json:"favorite,omitempty"`
	// Since is the start of retained history (empty when none).
	Since model.Timestamp `json:"since,omitempty"`
	// Gaps within retained history when Klew was not recording.
	Gaps []model.TimeRange `json:"gaps,omitempty"`
}

// Recording reports whether events for this context are being recorded by
// this or another Klew window.
func (s ContextStatus) Recording() bool {
	return s.State == StateRecording || s.State == StateOtherWindow
}

// ManagerConfig configures a Manager.
type ManagerConfig struct {
	// Dir is the history directory (…/Klew/history).
	Dir           string
	Clients       ClientFactory
	OnChange      func()
	Now           func() time.Time
	ReconcileTick time.Duration
	PruneTick     time.Duration
	// RecorderDefaults are copied into every recorder (tests shorten timings).
	RecorderDefaults RecorderConfig
}

type runningRecorder struct {
	context string
	key     string
	rec     *Recorder
	cancel  context.CancelFunc
	done    chan struct{}
	lock    *Lock
}

// Manager owns the store and the recorders of one Klew process.
type Manager struct {
	cfg   ManagerConfig
	store *Store

	// reconcileMu serializes reconcile with Clear/Restart so recorders are
	// never started half-way through a clear.
	reconcileMu sync.Mutex

	mu        sync.Mutex
	settings  Settings
	active    string
	favorites []string
	paused    bool
	running   map[string]*runningRecorder // by context name
	blocked   map[string]ContextStatus    // desired but not running (lock, client error)
	keys      map[string]string           // context → cluster key cache
	closed    bool

	loopCancel context.CancelFunc
	loopDone   chan struct{}
	kick       chan struct{}
}

// NewManager opens the store under cfg.Dir and starts reconcile and prune loops.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.Dir == "" {
		return nil, errors.New("history directory unavailable")
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.ReconcileTick <= 0 {
		cfg.ReconcileTick = 30 * time.Second
	}
	if cfg.PruneTick <= 0 {
		cfg.PruneTick = time.Hour
	}
	store, err := Open(filepath.Join(cfg.Dir, "events.db"))
	if err != nil {
		return nil, err
	}
	m := &Manager{
		cfg:      cfg,
		store:    store,
		settings: loadSettings(cfg.Dir),
		running:  map[string]*runningRecorder{},
		blocked:  map[string]ContextStatus{},
		keys:     map[string]string{},
		kick:     make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.loopCancel = cancel
	m.loopDone = make(chan struct{})
	go m.loop(ctx)
	return m, nil
}

// Store exposes the underlying store (read access for callers and tests).
func (m *Manager) Store() *Store { return m.store }

// Close stops all recorders, ends coverage sessions and closes the store.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.mu.Unlock()
	m.loopCancel()
	<-m.loopDone
	m.stopAll(EndKlewClosed)
	_ = m.store.Close()
}

// SetActiveContext sets this window's active context ("" for none).
func (m *Manager) SetActiveContext(name string) {
	m.mu.Lock()
	changed := m.active != name
	m.active = name
	m.mu.Unlock()
	if changed {
		m.Kick()
	}
}

// SetFavoriteContexts replaces the background-recorded favorites.
func (m *Manager) SetFavoriteContexts(names []string) {
	clean := uniqueNonEmpty(names)
	m.mu.Lock()
	m.favorites = clean
	m.mu.Unlock()
	m.Kick()
}

// SetPaused pauses all recording in this process (monitoring paused).
func (m *Manager) SetPaused(paused bool) {
	m.mu.Lock()
	changed := m.paused != paused
	m.paused = paused
	m.mu.Unlock()
	if changed {
		m.Kick()
	}
}

// Settings returns the current settings.
func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// SetSettings saves settings, applies them, and prunes with the new limits.
func (m *Manager) SetSettings(s Settings) (Settings, error) {
	s = s.normalized()
	if err := saveSettings(m.cfg.Dir, s); err != nil {
		return m.Settings(), err
	}
	m.mu.Lock()
	m.settings = s
	m.mu.Unlock()
	m.Kick()
	go m.Prune()
	return s, nil
}

// Clear deletes all recorded history and restarts recording from now.
func (m *Manager) Clear() error {
	m.reconcileMu.Lock()
	m.stopAll("")
	err := m.store.Clear(context.Background())
	m.reconcileMu.Unlock()
	m.Kick()
	return err
}

// SizeBytes is the history database size on disk.
func (m *Manager) SizeBytes() int64 { return m.store.SizeBytes() }

// Prune applies retention now (age, then size).
func (m *Manager) Prune() {
	s := m.Settings()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, _ = m.store.Prune(ctx, m.cfg.Now(), time.Duration(s.RetentionDays)*24*time.Hour, int64(s.MaxMB)*1024*1024)
}

// Kick schedules a reconcile.
func (m *Manager) Kick() {
	select {
	case m.kick <- struct{}{}:
	default:
	}
}

func (m *Manager) loop(ctx context.Context) {
	defer close(m.loopDone)
	m.Prune()
	m.reconcile()
	rt := time.NewTicker(m.cfg.ReconcileTick)
	defer rt.Stop()
	pt := time.NewTicker(m.cfg.PruneTick)
	defer pt.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.kick:
			m.reconcile()
		case <-rt.C:
			m.reconcile()
		case <-pt.C:
			m.Prune()
		}
	}
}

func (m *Manager) desired() (set []string, enabled, paused bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	enabled, paused = m.settings.Enabled, m.paused
	if !enabled || paused {
		return nil, enabled, paused
	}
	all := append([]string{m.active}, m.favorites...)
	return uniqueNonEmpty(all), enabled, paused
}

// reconcile starts and stops recorders to match the desired set and retries
// contexts another window was holding.
func (m *Manager) reconcile() {
	m.reconcileMu.Lock()
	defer m.reconcileMu.Unlock()
	// Another window may have changed settings (e.g. turned recording off).
	s := loadSettings(m.cfg.Dir)
	m.mu.Lock()
	m.settings = s
	m.mu.Unlock()

	want, enabled, paused := m.desired()
	stopReason := EndContextInactive
	switch {
	case !enabled:
		stopReason = EndRecordingOff
	case paused:
		stopReason = EndMonitoringPaused
	}
	wantSet := map[string]bool{}
	for _, c := range want {
		wantSet[c] = true
	}

	m.mu.Lock()
	var stop []*runningRecorder
	for name, rr := range m.running {
		if !wantSet[name] {
			stop = append(stop, rr)
			delete(m.running, name)
		}
	}
	for name := range m.blocked {
		if !wantSet[name] {
			delete(m.blocked, name)
		}
	}
	var start []string
	for _, c := range want {
		if _, ok := m.running[c]; !ok {
			start = append(start, c)
		}
	}
	m.mu.Unlock()

	for _, rr := range stop {
		rr.halt(stopReason)
	}
	for _, c := range start {
		m.startContext(c)
	}
	m.notify()
}

func (m *Manager) startContext(name string) {
	if m.cfg.Clients == nil {
		return
	}
	target, err := m.cfg.Clients(name)
	if err != nil {
		m.setBlocked(name, ContextStatus{Context: name, State: StateNotRecordable, Reason: "kubeconfig: context unavailable"})
		return
	}
	key := ClusterKey(name, target.ServerURL)
	m.mu.Lock()
	m.keys[name] = key
	m.mu.Unlock()

	lock, err := AcquireLock(filepath.Join(m.cfg.Dir, "locks"), key)
	if errors.Is(err, ErrLocked) {
		m.setBlocked(name, ContextStatus{Context: name, ClusterKey: key, State: StateOtherWindow,
			Reason: "recorded by another Klew window"})
		return
	}
	if err != nil {
		m.setBlocked(name, ContextStatus{Context: name, ClusterKey: key, State: StateNotRecordable, Reason: "cannot create lock file"})
		return
	}

	rc := m.cfg.RecorderDefaults
	rc.ClusterKey = key
	rc.Client = target.Client
	rc.Store = m.store
	rc.FallbackNamespaces = target.FallbackNamespaces
	rc.OnStatus = func(RecorderStatus) { m.notify() }
	rec := NewRecorder(rc)
	ctx, cancel := context.WithCancel(context.Background())
	rr := &runningRecorder{context: name, key: key, rec: rec, cancel: cancel, done: make(chan struct{}), lock: lock}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cancel()
		lock.Release()
		return
	}
	delete(m.blocked, name)
	m.running[name] = rr
	m.mu.Unlock()

	go func() {
		defer close(rr.done)
		rec.Run(ctx)
	}()
}

// halt stops the recorder; reason becomes its coverage session's end reason.
func (rr *runningRecorder) halt(reason string) {
	rr.rec.SetStopReason(reason)
	rr.cancel()
	<-rr.done
	rr.lock.Release()
}

func (m *Manager) setBlocked(name string, st ContextStatus) {
	m.mu.Lock()
	m.blocked[name] = st
	m.mu.Unlock()
}

func (m *Manager) stopAll(reason string) {
	m.mu.Lock()
	all := make([]*runningRecorder, 0, len(m.running))
	for name, rr := range m.running {
		all = append(all, rr)
		delete(m.running, name)
	}
	m.mu.Unlock()
	for _, rr := range all {
		rr.halt(reason)
	}
}

func (m *Manager) notify() {
	if m.cfg.OnChange != nil {
		m.cfg.OnChange()
	}
}

// IsRecording reports whether this process currently runs a recorder for name.
func (m *Manager) IsRecording(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.running[name]
	return ok
}

// Statuses returns recording state for the active context, favorites, the
// extra names given, and any context that still has retained history.
func (m *Manager) Statuses(extra []string) []ContextStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m.mu.Lock()
	settings, paused, active := m.settings, m.paused, m.active
	favs := map[string]bool{}
	for _, f := range m.favorites {
		favs[f] = true
	}
	names := uniqueNonEmpty(append(append([]string{active}, m.favorites...), extra...))
	running := map[string]*runningRecorder{}
	for k, v := range m.running {
		running[k] = v
	}
	blocked := map[string]ContextStatus{}
	for k, v := range m.blocked {
		blocked[k] = v
	}
	keys := map[string]string{}
	for k, v := range m.keys {
		keys[k] = v
	}
	m.mu.Unlock()

	// Contexts with retained history but not currently desired.
	historyKeys, _ := m.store.clusterKeysWithCoverage(ctx)
	byContext := map[string]string{}
	for _, k := range historyKeys {
		c := ContextFromKey(k)
		if _, ok := byContext[c]; !ok {
			byContext[c] = k
		}
		if keys[c] == "" {
			names = append(names, c)
		}
	}
	names = uniqueNonEmpty(names)

	out := make([]ContextStatus, 0, len(names))
	now := m.cfg.Now()
	since := now.Add(-time.Duration(settings.RetentionDays) * 24 * time.Hour)
	for _, name := range names {
		st := ContextStatus{Context: name, Active: name == active, Favorite: favs[name]}
		switch {
		case !settings.Enabled:
			st.State, st.Reason = StateOff, "turned off in Settings"
		case paused && (name == active || favs[name]):
			st.State, st.Reason = StatePaused, "monitoring paused"
		case running[name] != nil:
			rs := running[name].rec.Status()
			st.State, st.Reason, st.Scope, st.HighVolume = rs.State, rs.Reason, rs.Scope, rs.HighVolume
			st.ClusterKey = running[name].key
		case blocked[name].State != "":
			b := blocked[name]
			st.State, st.Reason, st.ClusterKey = b.State, b.Reason, b.ClusterKey
		default:
			st.State, st.Reason = StateOff, "not the active context or a favorite"
		}
		if st.ClusterKey == "" {
			st.ClusterKey = keys[name]
		}
		if st.ClusterKey == "" {
			st.ClusterKey = byContext[name]
		}
		if st.ClusterKey != "" {
			if cov, err := m.store.Coverage(ctx, st.ClusterKey, since, now, settings.EventTTL()); err == nil && cov.Recorded() {
				st.Since = model.TimestampFrom(cov.Since)
				st.Gaps = toRanges(cov.Gaps)
			}
		}
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Context < out[j].Context })
	return out
}

// RecordedEvents implements kube.EventHistory: stored events marked Recorded,
// plus coverage from q.Since (or the retention start) until now.
func (m *Manager) RecordedEvents(ctx context.Context, contextName, serverURL string, q model.EventHistoryQuery) ([]model.EventRecord, model.EventHistoryCoverage, error) {
	if m == nil {
		return nil, model.EventHistoryCoverage{}, nil
	}
	return recordedEvents(ctx, m.store, m.Settings(), m.cfg.Now(), contextName, serverURL, q)
}

func recordedEvents(ctx context.Context, store *Store, settings Settings, now time.Time, contextName, serverURL string, q model.EventHistoryQuery) ([]model.EventRecord, model.EventHistoryCoverage, error) {
	key := ClusterKey(contextName, serverURL)
	since := q.Since
	if since.IsZero() {
		since = now.Add(-time.Duration(settings.RetentionDays) * 24 * time.Hour)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 500
	}
	rows, err := store.Events(ctx, EventQuery{
		ClusterKey: key, Namespace: q.Namespace, Since: since,
		RegardingName: q.RegardingName, NamePrefix: q.NamePrefix, Limit: limit,
	})
	if err != nil {
		return nil, model.EventHistoryCoverage{}, err
	}
	cov, err := store.Coverage(ctx, key, since, now, settings.EventTTL())
	if err != nil {
		return nil, model.EventHistoryCoverage{}, err
	}
	out := make([]model.EventRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, RowToRecord(r))
	}
	mc := model.EventHistoryCoverage{Gaps: toRanges(cov.Gaps)}
	if cov.Recorded() {
		mc.Since = model.TimestampFrom(cov.Since)
	}
	return out, mc, nil
}

// RowToRecord converts a stored row to the shared event model.
func RowToRecord(r EventRow) model.EventRecord {
	return model.EventRecord{
		Timestamp: model.TimestampFrom(r.LastSeen),
		Type:      r.Type,
		Reason:    r.Reason,
		Message:   r.Message,
		Count:     r.Count,
		Source:    r.ReportingController,
		InvolvedObject: model.ObjectRef{
			Kind: r.RegardingKind, Name: r.RegardingName, Namespace: r.Namespace, UID: r.RegardingUID,
		},
		UID:       r.UID,
		FirstSeen: model.TimestampFrom(r.FirstSeen),
		LastSeen:  model.TimestampFrom(r.LastSeen),
		Recorded:  true,
	}
}

func toRanges(gaps []Gap) []model.TimeRange {
	if len(gaps) == 0 {
		return nil
	}
	out := make([]model.TimeRange, 0, len(gaps))
	for _, g := range gaps {
		out = append(out, model.TimeRange{From: model.TimestampFrom(g.From), To: model.TimestampFrom(g.To), Reason: g.Reason})
	}
	return out
}

func (s *Store) clusterKeysWithCoverage(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cluster_key FROM coverage GROUP BY cluster_key ORDER BY MAX(ended_at) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func settingsPath(dir string) string { return filepath.Join(dir, "settings.json") }

func loadSettings(dir string) Settings {
	s := DefaultSettings()
	b, err := os.ReadFile(settingsPath(dir))
	if err != nil {
		return s
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return DefaultSettings()
	}
	return s.normalized()
}

func saveSettings(dir string, s Settings) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := settingsPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, settingsPath(dir))
}

// Restart stops every recorder and starts them again with fresh clients
// (kubeconfig or proxy changed).
func (m *Manager) Restart() {
	m.reconcileMu.Lock()
	defer m.reconcileMu.Unlock()
	m.stopAll("")
	m.mu.Lock()
	m.blocked = map[string]ContextStatus{}
	m.keys = map[string]string{}
	m.mu.Unlock()
	m.Kick()
}
