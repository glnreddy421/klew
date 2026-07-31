package views

import (
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestFoldStreamRunsKeepsLogLines(t *testing.T) {
	t.Parallel()
	now := time.Now()
	evs := []model.EvidenceEvent{
		{Timestamp: now, SourceType: model.SourceLog, Pod: "payment-api", Container: "app", Message: "line 1", Raw: "line 1"},
		{Timestamp: now.Add(-time.Second), SourceType: model.SourceLog, Pod: "payment-api", Container: "app", Message: "line 2", Raw: "line 2"},
		{Timestamp: now.Add(-2 * time.Second), SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "payment-api", Reason: "BackOff", Message: "back-off"},
		{Timestamp: now.Add(-3 * time.Second), SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "payment-api", Reason: "BackOff", Message: "back-off"},
	}
	runs := foldStreamRuns(evs)
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs (2 logs + 1 folded event), got %d", len(runs))
	}
}
