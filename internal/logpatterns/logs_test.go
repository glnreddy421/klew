package logpatterns

import (
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
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
	now := time.Now().UTC()
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
