package engine

import (
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestReducerIngestsLiveEvidence(t *testing.T) {
	st := model.NewInvestigationState("payment", model.ModeLive)
	r := NewReducer(&st)
	r.ApplyEvent(model.EvidenceEvent{
		Timestamp:  time.Now().UTC(),
		SourceType: model.SourceLog,
		SourceKind: "Pod",
		SourceName: "payment-abc",
		Severity:   model.SeverityCritical,
		Reason:     "OOMKilled",
		Message:    "container oom",
	})
	out := r.State()
	if len(out.LiveEvidence) != 1 {
		t.Fatalf("expected 1 live event, got %d", len(out.LiveEvidence))
	}
	if out.Counters.LogsIngested != 1 {
		t.Fatalf("expected log counter 1, got %d", out.Counters.LogsIngested)
	}
}

func TestReducerPause(t *testing.T) {
	st := model.NewInvestigationState("x", model.ModeLive)
	r := NewReducer(&st)
	r.SetPaused(true)
	r.ApplyEvent(model.EvidenceEvent{Message: "ignored"})
	if len(r.State().LiveEvidence) != 0 {
		t.Fatal("expected paused reducer to ignore events")
	}
}
