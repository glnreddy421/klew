package history

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func TestOpenReaderWithoutDatabase(t *testing.T) {
	r, err := OpenReader(t.TempDir())
	if err != nil || r != nil {
		t.Fatalf("OpenReader on empty dir = %v, %v; want nil, nil", r, err)
	}
	// A nil reader is a valid, empty kube.EventHistory.
	recs, cov, err := r.RecordedEvents(context.Background(), "kind", "https://x", model.EventHistoryQuery{})
	if err != nil || len(recs) != 0 || cov.Since != "" {
		t.Fatalf("nil reader returned %v, %+v, %v", recs, cov, err)
	}
}

func TestReaderReturnsRecordedEvents(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	key := ClusterKey("kind-lab", "https://127.0.0.1:6443")

	s, err := Open(filepath.Join(dir, "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginSession(ctx, key, "", now.Add(-3*time.Hour), false); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertEvents(ctx, []EventRow{{
		ClusterKey: key, UID: "uid-1", Namespace: "klew-lab", RegardingKind: "Pod", RegardingName: "api-7d9-x",
		Type: "Warning", Reason: "OOMKilled", Message: "container exceeded memory", Count: 3,
		FirstSeen: now.Add(-2 * time.Hour), LastSeen: now.Add(-90 * time.Minute), RecordedAt: now.Add(-90 * time.Minute),
	}}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	r, err := OpenReader(dir)
	if err != nil || r == nil {
		t.Fatalf("OpenReader = %v, %v", r, err)
	}
	defer r.Close()
	recs, cov, err := r.RecordedEvents(ctx, "kind-lab", "https://127.0.0.1:6443", model.EventHistoryQuery{
		Namespace: "klew-lab", RegardingName: "api", NamePrefix: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || !recs[0].Recorded || recs[0].Reason != "OOMKilled" {
		t.Fatalf("recs = %+v", recs)
	}
	if cov.Since == "" {
		t.Fatalf("coverage since is empty: %+v", cov)
	}
}

func TestReaderDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.Exec(`UPDATE meta SET value = '99' WHERE key = 'schema_version'`); err != nil {
		t.Fatal(err)
	}

	r, err := OpenReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var v string
	if err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&v); err != nil || v != "99" {
		t.Fatalf("schema_version = %q, %v; opening a reader must not change it", v, err)
	}
	if _, err := r.store.db.Exec(`UPDATE meta SET value = '1'`); err == nil {
		t.Fatal("reader connection accepted a write")
	}
}

func TestReaderOpensWhileWriterHoldsLock(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, `ROLLBACK`) }()

	start := time.Now()
	r, err := OpenReader(dir)
	if err != nil {
		t.Fatalf("OpenReader with writer active: %v", err)
	}
	defer r.Close()
	if _, _, err := r.RecordedEvents(ctx, "kind", "https://x", model.EventHistoryQuery{}); err != nil {
		t.Fatalf("RecordedEvents with writer active: %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("reader waited on the writer for %v", d)
	}
}

func TestOpenReaderRejectsCorruptDatabase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "events.db"), []byte(strings.Repeat("garbage", 2000)), 0o600); err != nil {
		t.Fatal(err)
	}
	if r, err := OpenReader(dir); err == nil {
		_ = r.Close()
		t.Fatal("OpenReader accepted a corrupt database")
	}
}
