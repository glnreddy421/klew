package history

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/internal/model"
)

func fastRecorder() RecorderConfig {
	return RecorderConfig{
		FlushInterval:     10 * time.Millisecond,
		MinBackoff:        5 * time.Millisecond,
		MaxBackoff:        20 * time.Millisecond,
		DeniedRetry:       20 * time.Millisecond,
		CoverageHeartbeat: 20 * time.Millisecond,
	}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func v1Event(ns, name, uid, typ, reason string) *eventsv1.Event {
	now := metav1.NewMicroTime(time.Now().UTC())
	return &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, UID: types.UID(uid)},
		EventTime:  now,
		Regarding:  corev1.ObjectReference{Kind: "Pod", Name: "payment-api-abc", Namespace: ns},
		Type:       typ, Reason: reason, Note: "note " + name,
		ReportingController: "kubelet", ReportingInstance: "node-1", Action: "Pulling",
	}
}

func coreEvent(ns, name, uid string) *corev1.Event {
	t := metav1.NewTime(time.Now().UTC())
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: name, Namespace: ns, UID: types.UID(uid)},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "payment-api-abc", Namespace: ns},
		Type:           corev1.EventTypeWarning, Reason: "BackOff", Message: "back-off",
		FirstTimestamp: t, LastTimestamp: t, Count: 3,
	}
}

func storedUIDs(t *testing.T, s *Store, key string) map[string]EventRow {
	t.Helper()
	rows, err := s.Events(context.Background(), EventQuery{ClusterKey: key, Limit: 10000})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]EventRow{}
	for _, r := range rows {
		out[r.UID] = r
	}
	return out
}

func startRecorder(t *testing.T, cfg RecorderConfig) (*Recorder, context.CancelFunc, chan struct{}) {
	t.Helper()
	r := NewRecorder(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); r.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return r, cancel, done
}

func TestRecorderListThenWatchClusterWide(t *testing.T) {
	store := openTestStore(t)
	cs := fake.NewSimpleClientset(v1Event("klew-lab", "e1", "uid-1", "Warning", "Failed"))
	cfg := fastRecorder()
	cfg.ClusterKey, cfg.Client, cfg.Store = "ctx|h", cs, store
	r, _, _ := startRecorder(t, cfg)

	eventually(t, "recording", func() bool { return r.Status().State == StateRecording })
	if st := r.Status(); st.API != APIEventsV1 || st.Scope != ScopeCluster {
		t.Fatalf("status = %+v", st)
	}
	eventually(t, "listed event stored", func() bool { _, ok := storedUIDs(t, store, "ctx|h")["uid-1"]; return ok })

	if _, err := cs.EventsV1().Events("other").Create(context.Background(), v1Event("other", "e2", "uid-2", "Normal", "Pulled"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "watched event stored", func() bool { _, ok := storedUIDs(t, store, "ctx|h")["uid-2"]; return ok })

	row := storedUIDs(t, store, "ctx|h")["uid-1"]
	if row.Namespace != "klew-lab" || row.RegardingName != "payment-api-abc" || row.ReportingController != "kubelet" || row.Action != "Pulling" {
		t.Fatalf("row = %+v", row)
	}
	eventually(t, "coverage", func() bool {
		start, _ := store.HistoryStart(context.Background(), "ctx|h")
		return !start.IsZero()
	})

	// Only events.k8s.io/v1 is watched; core/v1 is never touched.
	for _, a := range cs.Actions() {
		if a.GetResource().Resource == "events" && a.GetResource().Group == "" {
			t.Fatalf("unexpected core/v1 call: %s", a.GetVerb())
		}
	}
}

func TestRecorderFallsBackToCoreV1WhenEventsV1Missing(t *testing.T) {
	store := openTestStore(t)
	cs := fake.NewSimpleClientset(coreEvent("ns1", "c1", "core-uid"))
	cs.PrependReactor("list", "events", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetResource().Group == "events.k8s.io" {
			return true, nil, apierrors.NewNotFound(eventsv1.Resource("events"), "")
		}
		return false, nil, nil
	})
	cfg := fastRecorder()
	cfg.ClusterKey, cfg.Client, cfg.Store = "k", cs, store
	r, _, _ := startRecorder(t, cfg)
	eventually(t, "recording", func() bool { return r.Status().State == StateRecording })
	if r.Status().API != APICoreV1 {
		t.Fatalf("api = %s", r.Status().API)
	}
	eventually(t, "core event", func() bool { return storedUIDs(t, store, "k")["core-uid"].Count == 3 })
}

func TestRecorderRelistsOnGone(t *testing.T) {
	store := openTestStore(t)
	cs := fake.NewSimpleClientset(v1Event("ns", "e1", "uid-1", "Warning", "Failed"))
	var lists, watches int32
	cs.PrependReactor("list", "events", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if lr, ok := a.(k8stesting.ListAction); ok && lr.GetListRestrictions().Labels.Empty() {
			atomic.AddInt32(&lists, 1)
		}
		return false, nil, nil
	})
	cs.PrependWatchReactor("events", func(a k8stesting.Action) (bool, watch.Interface, error) {
		if atomic.AddInt32(&watches, 1) == 1 {
			fw := watch.NewFakeWithChanSize(1, false)
			fw.Error(&metav1.Status{Status: metav1.StatusFailure, Code: 410, Reason: metav1.StatusReasonExpired, Message: "too old resource version"})
			return true, fw, nil
		}
		return false, nil, nil
	})
	cfg := fastRecorder()
	cfg.ClusterKey, cfg.Client, cfg.Store = "k", cs, store
	r, _, _ := startRecorder(t, cfg)

	eventually(t, "second watch", func() bool { return atomic.LoadInt32(&watches) >= 2 })
	eventually(t, "recording", func() bool { return r.Status().State == StateRecording })
	// probe list + initial list + relist after 410.
	if n := atomic.LoadInt32(&lists); n < 3 {
		t.Fatalf("lists = %d, want >= 3 (relist after 410)", n)
	}
	// Relist is idempotent: exactly one row.
	eventually(t, "row stored", func() bool { return len(storedUIDs(t, store, "k")) == 1 })
	time.Sleep(50 * time.Millisecond)
	if got := storedUIDs(t, store, "k"); len(got) != 1 {
		t.Fatalf("rows = %d", len(got))
	}
}

func TestRecorderResumesFromSavedResourceVersion(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_ = store.WriteBatch(ctx, nil, []WatchState{{ClusterKey: "k", API: APIEventsV1, ResourceVersion: "123", UpdatedAt: time.Now()}})
	cs := fake.NewSimpleClientset()
	var watchRV atomic.Value
	cs.PrependWatchReactor("events", func(a k8stesting.Action) (bool, watch.Interface, error) {
		if wa, ok := a.(k8stesting.WatchActionImpl); ok {
			watchRV.Store(wa.WatchRestrictions.ResourceVersion)
		}
		return false, nil, nil
	})
	cfg := fastRecorder()
	cfg.ClusterKey, cfg.Client, cfg.Store = "k", cs, store
	startRecorder(t, cfg)
	eventually(t, "watch", func() bool { return watchRV.Load() != nil })
	if rv := watchRV.Load().(string); rv != "123" {
		t.Fatalf("watched from rv %q, want 123", rv)
	}
}

func forbidClusterWideEvents(cs *fake.Clientset) {
	cs.PrependReactor("list", "events", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetNamespace() == "" {
			return true, nil, apierrors.NewForbidden(eventsv1.Resource("events"), "", fmt.Errorf("denied"))
		}
		return false, nil, nil
	})
	cs.PrependWatchReactor("events", func(a k8stesting.Action) (bool, watch.Interface, error) {
		if a.GetNamespace() == "" {
			return true, nil, apierrors.NewForbidden(eventsv1.Resource("events"), "", fmt.Errorf("denied"))
		}
		return false, nil, nil
	})
}

func TestRecorderRBACFallbackToNamespaces(t *testing.T) {
	store := openTestStore(t)
	cs := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-b"}},
		v1Event("team-a", "e1", "uid-a", "Warning", "Failed"),
	)
	forbidClusterWideEvents(cs)
	// team-b events are denied too.
	cs.PrependReactor("list", "events", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetNamespace() == "team-b" {
			return true, nil, apierrors.NewForbidden(eventsv1.Resource("events"), "", fmt.Errorf("denied"))
		}
		return false, nil, nil
	})
	cfg := fastRecorder()
	cfg.ClusterKey, cfg.Client, cfg.Store = "k", cs, store
	r, _, _ := startRecorder(t, cfg)

	eventually(t, "recording", func() bool { return r.Status().State == StateRecording })
	st := r.Status()
	if st.Scope != ScopeNamespaces || st.Namespaces != 1 {
		t.Fatalf("status = %+v", st)
	}
	eventually(t, "ns event", func() bool { _, ok := storedUIDs(t, store, "k")["uid-a"]; return ok })
}

func TestRecorderUsesFallbackNamespacesWhenNamespaceListDenied(t *testing.T) {
	store := openTestStore(t)
	cs := fake.NewSimpleClientset(v1Event("mine", "e1", "uid-m", "Warning", "Failed"))
	forbidClusterWideEvents(cs)
	cs.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(corev1.Resource("namespaces"), "", fmt.Errorf("denied"))
	})
	cfg := fastRecorder()
	cfg.ClusterKey, cfg.Client, cfg.Store, cfg.FallbackNamespaces = "k", cs, store, []string{"mine"}
	r, _, _ := startRecorder(t, cfg)
	eventually(t, "recording", func() bool { return r.Status().State == StateRecording })
	eventually(t, "event", func() bool { _, ok := storedUIDs(t, store, "k")["uid-m"]; return ok })
}

func TestRecorderNotRecordableWhenEverythingDenied(t *testing.T) {
	store := openTestStore(t)
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "*", func(a k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(a.GetResource().GroupResource(), "", fmt.Errorf("denied"))
	})
	cfg := fastRecorder()
	cfg.ClusterKey, cfg.Client, cfg.Store, cfg.FallbackNamespaces = "k", cs, store, []string{"default"}
	r, _, _ := startRecorder(t, cfg)
	eventually(t, "not recordable", func() bool { return r.Status().State == StateNotRecordable })
	if r.Status().Reason == "" {
		t.Fatal("missing reason")
	}
	if start, _ := store.HistoryStart(context.Background(), "k"); !start.IsZero() {
		t.Fatal("coverage recorded while not recording")
	}
}

func TestVolumeGuardSwitchesToWarningsOnly(t *testing.T) {
	v := volumeGuard{window: 10 * time.Second, high: 200}
	t0 := time.Unix(0, 0)
	var changed, warn bool
	// 300 events/s for just over one window.
	for i := 0; i <= 3100; i++ {
		c, w := v.observe(t0.Add(time.Duration(i) * (time.Second / 300)))
		changed = changed || c
		warn = w
	}
	if !changed || !warn {
		t.Fatalf("expected warnings-only, changed=%v warn=%v", changed, warn)
	}
	// Then a quiet window: 5 events/s.
	base := t0.Add(11 * time.Second)
	for i := 1; i <= 60; i++ {
		_, warn = v.observe(base.Add(time.Duration(i) * 200 * time.Millisecond))
	}
	if warn {
		t.Fatal("should leave warnings-only once rate drops")
	}
}

// --- Manager: pause/resume, locks, statuses ---

func fakeFactory(cs *fake.Clientset) ClientFactory {
	return func(name string) (ClusterTarget, error) {
		return ClusterTarget{Client: cs, ServerURL: "https://" + name + ":6443"}, nil
	}
}

func newTestManager(t *testing.T, dir string, cs *fake.Clientset) *Manager {
	t.Helper()
	m, err := NewManager(ManagerConfig{
		Dir: dir, Clients: fakeFactory(cs), ReconcileTick: 20 * time.Millisecond,
		RecorderDefaults: fastRecorder(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}

func statusFor(m *Manager, name string) ContextStatus {
	for _, s := range m.Statuses(nil) {
		if s.Context == name {
			return s
		}
	}
	return ContextStatus{}
}

func TestManagerPauseAndResume(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "history")
	cs := fake.NewSimpleClientset(v1Event("ns", "e1", "uid-1", "Warning", "Failed"))
	m := newTestManager(t, dir, cs)
	m.SetActiveContext("docker-desktop")
	key := ClusterKey("docker-desktop", "https://docker-desktop:6443")

	eventually(t, "recording", func() bool { return statusFor(m, "docker-desktop").State == StateRecording })
	eventually(t, "event", func() bool { _, ok := storedUIDs(t, m.Store(), key)["uid-1"]; return ok })

	m.SetPaused(true)
	eventually(t, "paused", func() bool { return !m.IsRecording("docker-desktop") })
	if st := statusFor(m, "docker-desktop"); st.State != StatePaused {
		t.Fatalf("state = %s", st.State)
	}
	_, _ = cs.EventsV1().Events("ns").Create(context.Background(), v1Event("ns", "e2", "uid-2", "Warning", "Failed"), metav1.CreateOptions{})
	time.Sleep(100 * time.Millisecond)
	if _, ok := storedUIDs(t, m.Store(), key)["uid-2"]; ok {
		t.Fatal("recorded while paused")
	}

	m.SetPaused(false)
	// Resume relists, picking up the event created during the pause.
	eventually(t, "resumed", func() bool { _, ok := storedUIDs(t, m.Store(), key)["uid-2"]; return ok })
}

func TestManagerTurnedOffStopsWrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "history")
	cs := fake.NewSimpleClientset()
	m := newTestManager(t, dir, cs)
	m.SetActiveContext("c")
	eventually(t, "recording", func() bool { return m.IsRecording("c") })
	s := m.Settings()
	s.Enabled = false
	if _, err := m.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	eventually(t, "stopped", func() bool { return !m.IsRecording("c") })
	if st := statusFor(m, "c"); st.State != StateOff {
		t.Fatalf("state = %s", st.State)
	}
	// Persisted for other windows.
	if loadSettings(dir).Enabled {
		t.Fatal("setting not saved")
	}
}

func TestSecondManagerDoesNotRecordSameContext(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "history")
	cs := fake.NewSimpleClientset()
	a := newTestManager(t, dir, cs)
	a.SetFavoriteContexts([]string{"shared"})
	eventually(t, "a recording", func() bool { return a.IsRecording("shared") })

	b := newTestManager(t, dir, cs)
	b.SetActiveContext("shared")
	eventually(t, "b sees other window", func() bool { return statusFor(b, "shared").State == StateOtherWindow })
	if b.IsRecording("shared") {
		t.Fatal("second process recorder started")
	}

	// One write path: every event stored once.
	_, _ = cs.EventsV1().Events("ns").Create(context.Background(), v1Event("ns", "e", "uid-x", "Warning", "Failed"), metav1.CreateOptions{})
	key := ClusterKey("shared", "https://shared:6443")
	eventually(t, "stored", func() bool { _, ok := storedUIDs(t, b.Store(), key)["uid-x"]; return ok })

	// When A stops, B takes over on its next reconcile.
	a.SetFavoriteContexts(nil)
	eventually(t, "b takes over", func() bool { return b.IsRecording("shared") })
}

func TestLockIsExclusive(t *testing.T) {
	dir := t.TempDir()
	l1, err := AcquireLock(dir, "ctx|abc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(dir, "ctx|abc"); err != ErrLocked {
		t.Fatalf("second lock err = %v, want ErrLocked", err)
	}
	l1.Release()
	l2, err := AcquireLock(dir, "ctx|abc")
	if err != nil {
		t.Fatalf("relock: %v", err)
	}
	l2.Release()
}

func TestManagerClearRestartsCoverage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "history")
	cs := fake.NewSimpleClientset(v1Event("ns", "e1", "uid-1", "Warning", "Failed"))
	m := newTestManager(t, dir, cs)
	m.SetActiveContext("c")
	key := ClusterKey("c", "https://c:6443")
	eventually(t, "event", func() bool { _, ok := storedUIDs(t, m.Store(), key)["uid-1"]; return ok })
	if err := m.Clear(); err != nil {
		t.Fatal(err)
	}
	// Recording restarts: coverage begins again from after the clear.
	eventually(t, "coverage restarted", func() bool {
		s, _ := m.Store().HistoryStart(context.Background(), key)
		return !s.IsZero()
	})
}

func TestRecordedEventsMarkedRecorded(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "history")
	m := newTestManager(t, dir, fake.NewSimpleClientset())
	key := ClusterKey("c", "https://srv")
	now := time.Now().UTC()
	_ = m.Store().UpsertEvents(context.Background(), []EventRow{{
		ClusterKey: key, UID: "u", Namespace: "ns", RegardingKind: "Pod", RegardingName: "p",
		Type: "Warning", Reason: "Failed", Count: 4, FirstSeen: now.Add(-3 * time.Hour), LastSeen: now.Add(-2 * time.Hour), RecordedAt: now,
	}})
	recs, _, err := m.RecordedEvents(context.Background(), "c", "https://srv/", model.EventHistoryQuery{Namespace: "ns", Since: now.Add(-24 * time.Hour)})
	if err != nil || len(recs) != 1 {
		t.Fatalf("recs=%v err=%v", recs, err)
	}
	r := recs[0]
	if !r.Recorded || r.UID != "u" || r.Count != 4 || r.FirstSeen == "" || r.InvolvedObject.Name != "p" {
		t.Fatalf("record = %+v", r)
	}
}

func TestRecorderListAllowedWatchDeniedDoesNotSpin(t *testing.T) {
	store := openTestStore(t)
	cs := fake.NewSimpleClientset(v1Event("ns", "e1", "uid-1", "Warning", "Failed"))
	var lists int32
	cs.PrependReactor("list", "events", func(k8stesting.Action) (bool, runtime.Object, error) {
		atomic.AddInt32(&lists, 1)
		return false, nil, nil
	})
	cs.PrependWatchReactor("*", func(a k8stesting.Action) (bool, watch.Interface, error) {
		return true, nil, apierrors.NewForbidden(a.GetResource().GroupResource(), "", fmt.Errorf("watch denied"))
	})
	cfg := fastRecorder()
	cfg.DeniedRetry = time.Hour // a second probe inside the test window would be the bug
	cfg.ClusterKey, cfg.Client, cfg.Store = "k", cs, store
	r, _, _ := startRecorder(t, cfg)

	eventually(t, "not recordable", func() bool { return r.Status().State == StateNotRecordable })
	n := atomic.LoadInt32(&lists)
	time.Sleep(200 * time.Millisecond)
	if after := atomic.LoadInt32(&lists); after != n {
		t.Fatalf("recorder kept listing while watch is denied: %d → %d lists", n, after)
	}
}
