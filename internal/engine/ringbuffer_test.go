package engine

import (
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestRingBufferKeepsDistinctLogLines(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer(100)
	now := time.Now()
	for i := 0; i < 5; i++ {
		rb.Add(model.EvidenceEvent{
		Timestamp:  model.TimestampFrom(now.Add(time.Duration(i) * time.Millisecond)),
			SourceType: model.SourceLog,
			Severity:   model.SeverityInfo,
			Pod:        "payment-api",
			Container:  "app",
			Raw:        "10.0.0.1 - - [15/Jul/2026:21:34:0" + string(rune('0'+i)) + "] GET /health",
			Message:    "access log",
		})
	}
	if got := len(rb.Snapshot()); got != 5 {
		t.Fatalf("expected 5 distinct log lines, got %d", got)
	}
}

func TestRingBufferStillCollapsesEvents(t *testing.T) {
	t.Parallel()
	rb := NewRingBuffer(100)
	ev := model.EvidenceEvent{
		SourceType: model.SourceK8sEvent,
		Severity:   model.SeverityHigh,
		Reason:     "BackOff",
		SourceKind: "Pod",
		SourceName: "app-1",
		Message:    "back-off restarting",
	}
	rb.Add(ev)
	rb.Add(ev)
	snap := rb.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected collapsed event, got %d", len(snap))
	}
	if snap[0].Count != 2 {
		t.Fatalf("expected count 2, got %d", snap[0].Count)
	}
}
