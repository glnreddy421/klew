package logpatterns

import (
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func TestLogsFromSnapshotSpreadsTimestamps(t *testing.T) {
	now := time.Now().UTC()
	lr := model.LogRecord{
		PodName: "payment-api", ContainerName: "app",
		CollectedAt: model.TimestampFrom(now),
		Lines:       []string{"line-a", "line-b", "line-c"},
	}
	evs := LogsFromSnapshot([]model.LogRecord{lr}, nil, "klew-lab")
	if len(evs) != 3 {
		t.Fatalf("len=%d", len(evs))
	}
	if !evs[2].Timestamp.After(evs[0].Timestamp) {
		t.Fatalf("expected newest line later than oldest: %v %v", evs[0].Timestamp, evs[2].Timestamp)
	}
	if evs[0].Reason == "" && !strings.Contains(strings.ToLower(evs[0].Message), "line") {
		t.Fatalf("unexpected event: %#v", evs[0])
	}
}

func TestMergeSnapshotLogsFeedsExtractWithoutLiveTail(t *testing.T) {
	// Correlation compares activity per minute bucket. Anchor 30s into a
	// minute that has already passed so every timestamp below (now-2m-1s …
	// now) keeps the same bucket alignment; time.Now() alone made this test
	// fail whenever it ran within seconds of a minute boundary.
	now := time.Now().UTC().Truncate(time.Minute).Add(-time.Minute + 30*time.Second)
	// Live ring has only k8s events — no log tail.
	live := []model.EvidenceEvent{{
		Timestamp: model.TimestampFrom(now), SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "payment-api",
		Reason: "OOMKilling", Message: "Memory cgroup limit exceeded", Severity: model.SeverityCritical, Count: 1,
	}}
	snapEvents := []model.EventRecord{{
		Timestamp: model.TimestampFrom(now.Add(-2 * time.Minute)), Reason: "OOMKilling",
		Message: "Memory cgroup limit exceeded", Count: 1,
		InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-api", Namespace: "klew-lab"},
	}, {
		Timestamp: model.TimestampFrom(now.Add(-2*time.Minute + 5*time.Second)), Reason: "BackOff",
		Message: "Back-off restarting failed container", Count: 1,
		InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-api", Namespace: "klew-lab"},
	}}
	snapLogs := []model.LogRecord{{
		PodName: "payment-api", ContainerName: "app", Previous: true,
		CollectedAt: model.TimestampFrom(now),
		Lines: []string{
			"allocating memory until OOM",
			"Killed",
		},
	}}
	out := Extract(live, SnapshotInput{
		Events: snapEvents, PreviousLogs: snapLogs,
	}, Options{SparklineMins: 15})
	if len(out.Templates) == 0 {
		t.Fatalf("expected log templates from snapshot logs, got %#v", out.Templates)
	}
	if len(out.EventTemplates) == 0 {
		t.Fatal("expected event templates")
	}
	if out.EvidenceBoard == nil || out.EvidenceBoard.CardCount == 0 {
		t.Fatalf("expected correlated signals, board=%#v", out.EvidenceBoard)
	}
}

func TestMergeSnapshotLogsDedupesRingLines(t *testing.T) {
	now := time.Now().UTC()
	msg := "allocating memory until OOM"
	live := []model.EvidenceEvent{{
		Timestamp: model.TimestampFrom(now), SourceType: model.SourceLog,
		Pod: "p", Container: "c", Message: msg, Raw: msg, Severity: model.SeverityCritical,
	}}
	snap := SnapshotInput{Logs: []model.LogRecord{{
		PodName: "p", ContainerName: "c", CollectedAt: model.TimestampFrom(now),
		Lines: []string{msg, "other line"},
	}}}
	merged := mergeSnapshotLogs(live, snap, 500)
	logCount := 0
	for _, e := range merged {
		if e.SourceType == model.SourceLog {
			logCount++
		}
	}
	if logCount != 2 {
		t.Fatalf("deduped count=%d want 2 (live + one new snapshot line)", logCount)
	}
}

func TestClassifyLogSeverityTrustsStatedLevel(t *testing.T) {
	for line, want := range map[string]model.Severity{
		"2026-11-08T17:16:54+00:00 INFO flushed batch=25198 events=0 errors=0":                  model.SeverityInfo,
		"2026-11-08T17:17:14+00:00 WARN slow flush detected took=1.2s threshold=1s (non-fatal)": model.SeverityWarning,
		"2026/11/08 17:14:18 [notice] 1#1: worker process 41 exited with code 0":                model.SeverityInfo,
		`time=2026-11-08T17:00:00Z level=info msg="retry failed earlier, now ok"`:               model.SeverityInfo,
		`{"level":"error","msg":"db down"}`:                                                     model.SeverityHigh,
		"2026-11-08T17:14:21+00:00 FATAL required_version file not found":                       model.SeverityCritical,
		"dial tcp 10.0.0.5:5432: connect: connection refused":                                   model.SeverityHigh,
		"Info: we do not trust lower-case words, so this error counts":                          model.SeverityHigh,
		"hello": model.SeverityInfo,
	} {
		if got := ClassifyLogSeverity(line); got != want {
			t.Errorf("ClassifyLogSeverity(%q)=%s want %s", line, got, want)
		}
	}
	if r := classifyLogReason("INFO flushed batch=1 errors=0"); r != "" {
		t.Errorf("reason for an INFO line = %q, want none", r)
	}
}
