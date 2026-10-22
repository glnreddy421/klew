package history

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "history", "events.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUpsertIsIdempotentAndKeepsFirstSeen(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 18, 14, 0, 0, 0, time.UTC)
	row := EventRow{
		ClusterKey: "kind|abc", UID: "uid-1", Namespace: "klew-lab",
		RegardingKind: "Pod", RegardingName: "payment-api-1", Type: "Warning",
		Reason: "BackOff", Message: "back-off 10s", Count: 1,
		FirstSeen: t0, LastSeen: t0, RecordedAt: t0,
	}
	if err := s.UpsertEvents(ctx, []EventRow{row}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertEvents(ctx, []EventRow{row}); err != nil {
		t.Fatal(err)
	}
	later := row
	later.FirstSeen = t0.Add(5 * time.Minute) // events.k8s.io may report a newer series start
	later.LastSeen = t0.Add(10 * time.Minute)
	later.Count = 7
	later.Message = "back-off 5m0s"
	if err := s.UpsertEvents(ctx, []EventRow{later}); err != nil {
		t.Fatal(err)
	}
	// A stale replay (older last_seen, lower count) must not regress anything.
	if err := s.UpsertEvents(ctx, []EventRow{row}); err != nil {
		t.Fatal(err)
	}

	got, err := s.Events(ctx, EventQuery{ClusterKey: "kind|abc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 row, got %d", len(got))
	}
	r := got[0]
	if !r.FirstSeen.Equal(t0) {
		t.Errorf("first_seen = %v, want %v", r.FirstSeen, t0)
	}
	if !r.LastSeen.Equal(t0.Add(10 * time.Minute)) {
		t.Errorf("last_seen = %v", r.LastSeen)
	}
	if r.Count != 7 {
		t.Errorf("count = %d, want 7", r.Count)
	}
	if r.Message != "back-off 5m0s" {
		t.Errorf("message = %q", r.Message)
	}
}

func TestSameUIDDifferentClustersStaySeparate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rows := []EventRow{
		{ClusterKey: "a|1", UID: "u", LastSeen: now, RecordedAt: now},
		{ClusterKey: "a|2", UID: "u", LastSeen: now, RecordedAt: now},
	}
	if err := s.UpsertEvents(ctx, rows); err != nil {
		t.Fatal(err)
	}
	n, _ := s.CountEvents(ctx, "")
	if n != 2 {
		t.Fatalf("want 2 rows, got %d", n)
	}
}

func TestWatchStateRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, ok, _ := s.LoadWatchState(ctx, "k", ""); ok {
		t.Fatal("unexpected state")
	}
	st := WatchState{ClusterKey: "k", API: APIEventsV1, ResourceVersion: "42", UpdatedAt: time.Now()}
	if err := s.WriteBatch(ctx, nil, []WatchState{st}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.LoadWatchState(ctx, "k", "")
	if err != nil || !ok || got.ResourceVersion != "42" || got.API != APIEventsV1 {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
	_ = s.ClearWatchState(ctx, "k")
	if _, ok, _ := s.LoadWatchState(ctx, "k", ""); ok {
		t.Fatal("state not cleared")
	}
}

func TestPruneByAge(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rows := []EventRow{
		{ClusterKey: "k", UID: "old", LastSeen: now.Add(-8 * 24 * time.Hour), RecordedAt: now},
		{ClusterKey: "k", UID: "new", LastSeen: now.Add(-1 * time.Hour), RecordedAt: now},
	}
	if err := s.UpsertEvents(ctx, rows); err != nil {
		t.Fatal(err)
	}
	removed, err := s.Prune(ctx, now, 7*24*time.Hour, 0)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d, want 1", removed)
	}
	got, _ := s.Events(ctx, EventQuery{ClusterKey: "k"})
	if len(got) != 1 || got[0].UID != "new" {
		t.Fatalf("unexpected rows %+v", got)
	}
}

func TestPruneBySizeDropsOldestFirst(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	msg := strings.Repeat("x", 150)
	const n = 20000
	var rows []EventRow
	for i := 0; i < n; i++ {
		// Real event UIDs are random, so age order does not follow key order
		// and deletes leave half-empty pages behind.
		rows = append(rows, EventRow{
			ClusterKey: "k", UID: fmt.Sprintf("%05d-u%05d", (i*7919)%n, i), Message: msg,
			LastSeen: now.Add(time.Duration(i) * time.Second), RecordedAt: now,
		})
	}
	if err := s.UpsertEvents(ctx, rows); err != nil {
		t.Fatal(err)
	}
	used, _ := s.usedBytes(ctx)
	budget := used / 4
	removed, err := s.Prune(ctx, now, 0, budget)
	if err != nil {
		t.Fatal(err)
	}
	if removed < n/2 {
		t.Fatalf("removed only %d rows", removed)
	}
	got, _ := s.Events(ctx, EventQuery{ClusterKey: "k", Limit: n})
	if len(got) == 0 {
		t.Fatal("pruned everything")
	}
	if !strings.HasSuffix(got[0].UID, "u19999") {
		t.Fatalf("newest row dropped, first is %s", got[0].UID)
	}
	if oldest := got[len(got)-1].UID; strings.HasSuffix(oldest, "u00000") {
		t.Fatal("oldest row kept")
	}

	// Once it fits, later prunes with the same budget must not keep shrinking history.
	again, err := s.Prune(ctx, now, 0, budget)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Fatalf("second prune removed %d rows; history keeps shrinking", again)
	}
}

func TestCoverageGaps(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 18, 14, 2, 0, 0, time.UTC)

	id1, _ := s.BeginSession(ctx, "k", "cluster", t0, false)
	_ = s.TouchSession(ctx, id1, t0.Add(time.Hour))
	// Short reconnect (< tolerance) is not a gap.
	id2, _ := s.BeginSession(ctx, "k", "cluster", t0.Add(time.Hour+30*time.Second), false)
	_ = s.TouchSession(ctx, id2, t0.Add(2*time.Hour))
	// Klew closed for 3h.
	id3, _ := s.BeginSession(ctx, "k", "cluster", t0.Add(5*time.Hour), false)
	_ = s.TouchSession(ctx, id3, t0.Add(6*time.Hour))
	// Other cluster is ignored.
	_, _ = s.BeginSession(ctx, "other", "cluster", t0.Add(-time.Hour), false)

	cov, err := s.Coverage(ctx, "k", time.Time{}, t0.Add(6*time.Hour+30*time.Second), DefaultEventTTL)
	if err != nil {
		t.Fatal(err)
	}
	if !cov.Since.Equal(t0) {
		t.Errorf("since = %v", cov.Since)
	}
	if len(cov.Sessions) != 2 {
		t.Fatalf("sessions = %+v", cov.Sessions)
	}
	if len(cov.Gaps) != 1 {
		t.Fatalf("gaps = %+v", cov.Gaps)
	}
	g := cov.Gaps[0]
	if !g.From.Equal(t0.Add(2*time.Hour)) || !g.To.Equal(t0.Add(5*time.Hour)) {
		t.Errorf("gap = %+v", g)
	}

	// Trailing gap when nothing has recorded recently.
	cov, _ = s.Coverage(ctx, "k", time.Time{}, t0.Add(8*time.Hour), DefaultEventTTL)
	if len(cov.Gaps) != 2 {
		t.Fatalf("want trailing gap, got %+v", cov.Gaps)
	}

	// Object lifetime overlap.
	over := cov.GapsOverlapping(t0.Add(4*time.Hour), t0.Add(7*time.Hour))
	if len(over) != 2 || !over[0].From.Equal(t0.Add(4*time.Hour)) {
		t.Errorf("overlap = %+v", over)
	}

	start, _ := s.HistoryStart(ctx, "k")
	if !start.Equal(t0) {
		t.Errorf("history start = %v", start)
	}
}

func TestClearEmptiesEverything(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_ = s.UpsertEvents(ctx, []EventRow{{ClusterKey: "k", UID: "u", LastSeen: now, RecordedAt: now}})
	_, _ = s.BeginSession(ctx, "k", "", now, false)
	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	n, _ := s.CountEvents(ctx, "")
	start, _ := s.HistoryStart(ctx, "k")
	if n != 0 || !start.IsZero() {
		t.Fatalf("not cleared: n=%d start=%v", n, start)
	}
}

func TestTwoHandlesShareOneFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	done := make(chan error, 2)
	for i, st := range []*Store{a, b} {
		go func(i int, st *Store) {
			var err error
			for j := 0; j < 50 && err == nil; j++ {
				err = st.UpsertEvents(ctx, []EventRow{{ClusterKey: "k", UID: fmt.Sprintf("%d-%d", i, j), LastSeen: now, RecordedAt: now}})
			}
			done <- err
		}(i, st)
	}
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	n, _ := a.CountEvents(ctx, "k")
	if n != 100 {
		t.Fatalf("want 100 rows, got %d", n)
	}
}

// Backfill credit: a session that began with a successful list (or a resumed
// watch) also covers the event TTL before it, because the API server still
// held those events.
func TestCoverageBackfillShortPauseHasNoGap(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	id1, _ := s.BeginSession(ctx, "k", "cluster", t0, true)
	_ = s.EndSession(ctx, id1, t0.Add(time.Hour), EndKlewClosed)
	// Klew closed for 10 minutes (< 60m TTL), then relisted successfully.
	id2, _ := s.BeginSession(ctx, "k", "cluster", t0.Add(70*time.Minute), true)
	_ = s.TouchSession(ctx, id2, t0.Add(2*time.Hour))

	cov, err := s.Coverage(ctx, "k", time.Time{}, t0.Add(2*time.Hour), DefaultEventTTL)
	if err != nil {
		t.Fatal(err)
	}
	if len(cov.Gaps) != 0 {
		t.Fatalf("short pause should be backfilled, got gaps %+v", cov.Gaps)
	}
	// The first session's own backfill makes history start one TTL earlier.
	if want := t0.Add(-time.Hour); !cov.Since.Equal(want) {
		t.Fatalf("since = %v, want %v", cov.Since, want)
	}
}

func TestCoverageBackfillLongPauseLeavesTTLShortGap(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	id1, _ := s.BeginSession(ctx, "k", "cluster", t0, true)
	_ = s.EndSession(ctx, id1, t0.Add(time.Hour), EndKlewClosed)
	// Closed for 90 minutes; the relist recovers the last 60.
	resume := t0.Add(150 * time.Minute)
	id2, _ := s.BeginSession(ctx, "k", "cluster", resume, true)
	_ = s.TouchSession(ctx, id2, resume.Add(time.Hour))

	cov, _ := s.Coverage(ctx, "k", time.Time{}, resume.Add(time.Hour), DefaultEventTTL)
	if len(cov.Gaps) != 1 {
		t.Fatalf("want one gap, got %+v", cov.Gaps)
	}
	g := cov.Gaps[0]
	if !g.From.Equal(t0.Add(time.Hour)) || !g.To.Equal(resume.Add(-time.Hour)) {
		t.Fatalf("gap = %v–%v, want 30 minutes from %v", g.From, g.To, t0.Add(time.Hour))
	}
	if g.Reason != EndKlewClosed {
		t.Fatalf("gap reason = %q", g.Reason)
	}
	// A cluster with a shorter --event-ttl backfills less.
	cov, _ = s.Coverage(ctx, "k", time.Time{}, resume.Add(time.Hour), 15*time.Minute)
	if got := cov.Gaps[0].To.Sub(cov.Gaps[0].From); got != 75*time.Minute {
		t.Fatalf("gap with 15m TTL = %v, want 75m", got)
	}
}

func TestCoverageWithoutBackfillGetsNoCredit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	id1, _ := s.BeginSession(ctx, "k", "cluster", t0, true)
	_ = s.EndSession(ctx, id1, t0.Add(time.Hour), "")
	// Initial list failed: the session started without capturing retained events.
	id2, _ := s.BeginSession(ctx, "k", "cluster", t0.Add(70*time.Minute), false)
	_ = s.TouchSession(ctx, id2, t0.Add(2*time.Hour))

	cov, _ := s.Coverage(ctx, "k", time.Time{}, t0.Add(2*time.Hour), DefaultEventTTL)
	if len(cov.Gaps) != 1 || cov.Gaps[0].To.Sub(cov.Gaps[0].From) != 10*time.Minute {
		t.Fatalf("want the full 10m gap, got %+v", cov.Gaps)
	}
	if cov.Gaps[0].Reason != "" {
		t.Fatalf("unknown end should have no reason, got %q", cov.Gaps[0].Reason)
	}

	// Credit arrives once the resync lists successfully.
	_ = s.MarkBackfilled(ctx, id2)
	cov, _ = s.Coverage(ctx, "k", time.Time{}, t0.Add(2*time.Hour), DefaultEventTTL)
	if len(cov.Gaps) != 0 {
		t.Fatalf("backfill credit not applied: %+v", cov.Gaps)
	}
}

func TestTouchClearsProvisionalEndReason(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	id, _ := s.BeginSession(ctx, "k", "cluster", t0, false)
	_ = s.SetEndReason(ctx, id, EndUnreachable)
	_ = s.TouchSession(ctx, id, t0.Add(time.Minute))
	cov, _ := s.Coverage(ctx, "k", time.Time{}, t0.Add(time.Hour), 0)
	if len(cov.Gaps) != 1 || cov.Gaps[0].Reason != "" {
		t.Fatalf("touch should clear the provisional reason: %+v", cov.Gaps)
	}
}

func TestMigratesV1CoverageTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE coverage (id INTEGER PRIMARY KEY AUTOINCREMENT, cluster_key TEXT NOT NULL,
			scope TEXT NOT NULL DEFAULT '', started_at INTEGER NOT NULL, ended_at INTEGER NOT NULL)`,
		`INSERT INTO coverage (cluster_key, started_at, ended_at) VALUES ('k', 1000, 2000)`,
	} {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	_ = old.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open v1 db: %v", err)
	}
	defer s.Close()
	cov, err := s.Coverage(context.Background(), "k", time.Time{}, fromMS(3000), DefaultEventTTL)
	if err != nil {
		t.Fatal(err)
	}
	// Old rows claim no backfill.
	if len(cov.Sessions) != 1 || cov.Sessions[0].Backfilled || !cov.Since.Equal(fromMS(1000)) {
		t.Fatalf("migrated coverage = %+v", cov)
	}
}
