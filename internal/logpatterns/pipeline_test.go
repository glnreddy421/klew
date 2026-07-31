package logpatterns

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestDualPipeline_EventGuardAndBoard(t *testing.T) {
	p, err := NewDualPipeline(DualPipelineConfig{
		Logs:   TrackerConfig{SparklineMins: 15, MaxClusters: 64},
		Events: TrackerConfig{SparklineMins: 15, MaxClusters: 64},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.StartGC(ctx)
	defer p.Stop()

	now := time.Now().UTC().Truncate(time.Minute)
	// Shared active minutes → high Jaccard.
	for i := 0; i < 5; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute)
		p.IngestLog("dial tcp 10.0.0.1:5432: connection refused", 1, model.EvidenceEvent{
			Timestamp: ts, Pod: "api-1", Severity: model.SeverityHigh, SourceType: model.SourceLog,
		})
		p.IngestEvent(model.EvidenceEvent{
			Timestamp: ts, SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "api-1",
			Reason: "Failed", Message: "Failed to pull image", Severity: model.SeverityHigh, Count: 1,
		})
	}
	// Deployment noise must be discarded by EventMiner guard.
	p.IngestEvent(model.EvidenceEvent{
		Timestamp: now, SourceType: model.SourceK8sEvent, SourceKind: "Deployment", SourceName: "api",
		Reason: "ScalingReplicaSet", Message: "Scaled up", Severity: model.SeverityInfo, Count: 1,
	})

	logs, events, board := p.BuildSnapshots(now, 5, 5, 40, 5)
	if len(logs) == 0 {
		t.Fatal("expected log templates")
	}
	if len(events) == 0 {
		t.Fatal("expected event templates")
	}
	for _, e := range events {
		low := strings.ToLower(e.Template)
		if strings.Contains(low, "scalingreplicaset") || strings.Contains(low, "scaled up") {
			t.Fatalf("deployment noise leaked: %q", e.Template)
		}
	}
	if board == nil || board.CardCount < 1 {
		t.Fatalf("expected evidence card, board=%#v", board)
	}
	if board.Cards[0].Confidence < DefaultJaccardThreshold {
		t.Fatalf("confidence=%v", board.Cards[0].Confidence)
	}
	if len(logs[0].VolumeHistory) != 15 || len(logs[0].Sparkline) != 15 {
		t.Fatalf("volume/spark lengths %d/%d", len(logs[0].VolumeHistory), len(logs[0].Sparkline))
	}
}
