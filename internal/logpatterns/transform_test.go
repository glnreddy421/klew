package logpatterns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestPatternID_TableDrivenWhitespaceStability(t *testing.T) {
	const canonical = "dial tcp <IP> connect connection refused"
	want := PatternID(canonical)

	cases := []struct {
		name  string
		input string
	}{
		{name: "exact", input: canonical},
		{name: "trailing_space", input: canonical + " "},
		{name: "trailing_spaces", input: canonical + "   "},
		{name: "leading_spaces", input: "   " + canonical},
		{name: "leading_tab", input: "\t" + canonical},
		{name: "trailing_tab", input: canonical + "\t"},
		{name: "leading_trailing_tabs", input: "\t" + canonical + "\t"},
		{name: "trailing_newline", input: canonical + "\n"},
		{name: "trailing_crlf", input: canonical + "\r\n"},
		{name: "leading_newline", input: "\n" + canonical},
		{name: "wrapped_whitespace", input: " \t" + canonical + " \n"},
		{name: "tpl_string_space_nl", input: "tpl_string \n"},
		{name: "tpl_string_plain", input: "tpl_string"},
	}

	if PatternID("tpl_string \n") != PatternID("tpl_string") {
		t.Fatalf("PatternID must ignore trailing whitespace on tpl_string")
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := PatternID(tc.input)
			if strings.TrimSpace(tc.input) == canonical && got != want {
				t.Fatalf("PatternID(%q)=%q, want %q", tc.input, got, want)
			}
			if got == "" || !strings.HasPrefix(got, "tpl_") {
				t.Fatalf("PatternID(%q)=%q; want tpl_<hex>", tc.input, got)
			}
			body := strings.TrimSpace(tc.input)
			if PatternID(tc.input) != PatternID(body) {
				t.Fatalf("not TrimSpace-stable: %q vs %q", PatternID(tc.input), PatternID(body))
			}
		})
	}
}

func TestSnapshotClusters_ConcurrentWithAdd(t *testing.T) {
	miner, err := newMiner(4, 0.5, 256)
	if err != nil {
		t.Fatalf("newMiner: %v", err)
	}
	meta := NewMetaStore(15, 5)

	const writers, readers, perWriter = 6, 8, 80
	var adds, snaps atomic.Uint64
	start := make(chan struct{})
	seed := []string{
		"dial tcp 10.0.0.1:5432: connect: connection refused",
		"payment failed userId=u1 error.code=TIMEOUT",
		"health check ok status=ready",
	}

	var wg sync.WaitGroup
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		w := w
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				msg := fmt.Sprintf("%s writer=%d seq=%d", seed[(w+i)%len(seed)], w, i)
				res := miner.Add(msg)
				if res != nil && res.Cluster != nil {
					meta.Observe(res.Cluster.ClusterID, 1, model.EvidenceEvent{
						Timestamp: time.Now().UTC(),
						Pod:       fmt.Sprintf("pod-%d", w),
						Severity:  model.SeverityInfo,
					}, msg)
					adds.Add(1)
				}
			}
		}()
	}
	wg.Add(readers)
	for r := 0; r < readers; r++ {
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				_ = miner.SnapshotClusters()
				_ = meta.Snapshot()
				_ = BuildLogTemplates(miner, meta, 1, time.Now().UTC(), 40, 5)
				snaps.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if adds.Load() == 0 || snaps.Load() == 0 {
		t.Fatalf("adds=%d snaps=%d", adds.Load(), snaps.Load())
	}
}

func TestBuildLogTemplates_DeepCopyIsolatesMeta(t *testing.T) {
	miner, err := newMiner(4, 0.5, 64)
	if err != nil {
		t.Fatalf("newMiner: %v", err)
	}
	meta := NewMetaStore(15, 5)
	ts0 := time.Unix(1_700_000_000, 0).UTC()
	msg := "payment failed userId=u1 error.code=TIMEOUT"
	res := miner.Add(msg)
	if res == nil || res.Cluster == nil {
		t.Fatal("expected cluster")
	}
	meta.Observe(res.Cluster.ClusterID, 3, model.EvidenceEvent{
		Timestamp: ts0,
		Pod:       "payment-api-7db86bb96c-xzqpw",
		Severity:  model.SeverityHigh,
	}, "payment failed userId=u1")

	view := BuildLogTemplates(miner, meta, 3, ts0.Add(time.Minute), 40, 5)
	if len(view) == 0 {
		t.Fatal("expected templates")
	}
	row := view[0]
	if len(row.Pods) != 1 || row.Pods[0] != "payment-api-7db86bb96c-xzqpw" {
		t.Fatalf("pods=%v", row.Pods)
	}
	if len(row.Sparkline) != 15 {
		t.Fatalf("sparkline len=%d want 15", len(row.Sparkline))
	}
	if len(row.VolumeHistory) != 15 {
		t.Fatalf("volumeHistory len=%d want 15", len(row.VolumeHistory))
	}

	// Mutate live store after View build.
	meta.Observe(res.Cluster.ClusterID, 1, model.EvidenceEvent{
		Timestamp: ts0.Add(time.Hour),
		Pod:       "auth-service-7db86bb96c-abcde",
		Severity:  model.SeverityCritical,
	}, "LEAKED_SAMPLE")

	if len(row.Pods) != 1 || row.Pods[0] != "payment-api-7db86bb96c-xzqpw" {
		t.Fatalf("pods leaked: %v", row.Pods)
	}
	if len(row.Samples) != 1 || row.Samples[0].Message != "payment failed userId=u1" {
		t.Fatalf("samples leaked: %#v", row.Samples)
	}
	if row.Count != 3 {
		t.Fatalf("count leaked: %d", row.Count)
	}
	if row.Severity != model.SeverityHigh {
		t.Fatalf("severity leaked: %v", row.Severity)
	}
}

func TestAttachTemplateKeywords_SparseNilAndOmitEmpty(t *testing.T) {
	cases := []struct {
		name     string
		template string
	}{
		{name: "only_wildcards", template: "<*> <*> <*>"},
		{name: "wildcards_and_masks", template: "<*> <IP> <UUID> <TS> <POD>"},
		{name: "json_boilerplate", template: `{"msg":"<*>","level":"info","time":"<*>"}`},
		{name: "stopwords_only", template: "msg level time timestamp ts error info"},
		{name: "mixed_stripped", template: "msg <*> level <*> time"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rows := []model.LogTemplate{{ID: PatternID(tc.template), Template: tc.template, Count: 1}}
			attachTemplateKeywords(rows, []string{tc.template}, 5)
			if rows[0].Keywords != nil {
				t.Fatalf("Keywords must be nil; got %#v", rows[0].Keywords)
			}
			raw, err := json.Marshal(rows[0])
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), `"keywords"`) {
				t.Fatalf("omitempty failed: %s", raw)
			}
		})
	}
}

func TestMinuteBucketsAndPurge(t *testing.T) {
	store := NewMetaStore(15, 3)
	now := time.Date(2026, 7, 22, 12, 30, 0, 0, time.UTC)
	store.Observe(1, 2, model.EvidenceEvent{Timestamp: now, Pod: "p", Severity: model.SeverityInfo}, "a")
	store.Observe(1, 1, model.EvidenceEvent{Timestamp: now.Add(-2 * time.Minute), Pod: "p", Severity: model.SeverityInfo}, "b")
	store.Observe(1, 1, model.EvidenceEvent{Timestamp: now.Add(-20 * time.Minute), Pod: "p", Severity: model.SeverityInfo}, "old")

	snap := store.Snapshot()[1]
	if snap.MinuteBuckets[now.Truncate(time.Minute).Unix()] != 2 {
		t.Fatalf("active bucket=%v", snap.MinuteBuckets)
	}
	spark := flattenBuckets(snap.MinuteBuckets, now, 15)
	if len(spark) != 15 || spark[len(spark)-1] != 2 {
		t.Fatalf("spark=%v", spark)
	}
	// oldest slot in 15-min window should be 0 (20-min-old event outside window for display,
	// but still in map until purge)
	removed := store.Purge(now, nil)
	if removed < 1 {
		t.Fatalf("expected purge of old bucket, removed=%d", removed)
	}
	snap2 := store.Snapshot()[1]
	oldKey := now.Add(-20 * time.Minute).Truncate(time.Minute).Unix()
	if _, ok := snap2.MinuteBuckets[oldKey]; ok {
		t.Fatalf("old bucket still present: %v", snap2.MinuteBuckets)
	}
}

func TestTrendPct(t *testing.T) {
	pct := trendPct([]int64{10, 10, 10, 10, 20})
	if pct < 90 || pct > 110 {
		t.Fatalf("trendPct=%v want ~100", pct)
	}
	if trendArrow(pct) != "↑" {
		t.Fatalf("arrow=%s", trendArrow(pct))
	}
	if trendArrow(0) != "·" {
		t.Fatal("flat should be ·")
	}
}

func TestMetaStorePurgeOrphansAndPodCap(t *testing.T) {
	store := NewMetaStore(15, 3)
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		store.Observe(i, 1, model.EvidenceEvent{Timestamp: now, Pod: "p", Severity: model.SeverityInfo}, "x")
	}
	if store.Len() != 3 {
		t.Fatalf("len=%d", store.Len())
	}
	active := map[int]struct{}{1: {}}
	removed := store.Purge(now, active)
	if removed < 2 || store.Len() != 1 {
		t.Fatalf("removed=%d len=%d", removed, store.Len())
	}

	store2 := NewMetaStore(15, 3)
	for i := 0; i < maxPodsPerPattern+20; i++ {
		store2.Observe(1, 1, model.EvidenceEvent{
			Timestamp: now, Pod: fmt.Sprintf("pod-%d", i), Severity: model.SeverityInfo,
		}, "msg")
	}
	snap := store2.Snapshot()[1]
	if len(snap.Pods) != maxPodsPerPattern {
		t.Fatalf("pods=%d want %d", len(snap.Pods), maxPodsPerPattern)
	}
}

func TestTrackerGCAndCapture(t *testing.T) {
	tr, err := NewTracker(TrackerConfig{SparklineMins: 15, GCInterval: 20 * time.Millisecond, MaxClusters: 64})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr.StartGC(ctx)
	defer tr.Stop()

	now := time.Now().UTC().Truncate(time.Minute)
	tr.Ingest("payment failed timeout", 1, model.EvidenceEvent{
		Timestamp: now, Pod: "p1", Severity: model.SeverityHigh,
	})
	cap := tr.CaptureSnapshot(now)
	if len(cap.Clusters) == 0 {
		t.Fatal("expected clusters in capture")
	}
	view := tr.BuildView(cap, 1, 40, 5)
	if len(view) == 0 || view[0].ID == "" {
		t.Fatalf("view=%#v", view)
	}
	if len(view[0].Sparkline) != 15 {
		t.Fatalf("sparkline=%d", len(view[0].Sparkline))
	}
}
