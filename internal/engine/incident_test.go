package engine

import (
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func applyMock(store *StateStore, e mockEmit) {
	store.ApplyEvent(model.EvidenceEvent{
		Timestamp: time.Now(), SourceType: e.sourceType, SourceKind: e.kind, SourceName: e.name,
		Namespace: "prod", Pod: e.pod, Container: e.ctr, Severity: e.severity,
		Reason: e.reason, Message: e.message, Raw: e.message, Confidence: 0.8,
	})
}

// TestLeadingSignalFlips verifies the correlation engine changes the leading
// signal to OOMKilled once OOM + memory-log evidence arrives, and records it.
func TestLeadingSignalFlips(t *testing.T) {
	st := mockBaseState()
	store := NewStore(&st)

	pre := []mockEmit{
		{model.SourceLog, "Pod", "payment-gateway-a1", "payment-gateway-a1", "app", model.SeverityWarning, "Redis timeout", "redis connection timeout"},
		{model.SourceLog, "Pod", "payment-gateway-a2", "payment-gateway-a2", "app", model.SeverityWarning, "Redis timeout", "redis connection timeout"},
		{model.SourceK8sEvent, "Pod", "payment-gateway-a1", "payment-gateway-a1", "", model.SeverityHigh, "Readiness failed", "readiness probe failed"},
	}
	for _, e := range pre {
		applyMock(store, e)
	}
	before := store.State()
	if before.Verdict.LeadingSignal == "OOMKilled" {
		t.Fatalf("did not expect OOMKilled to lead before OOM evidence, got %q", before.Verdict.LeadingSignal)
	}

	oom := []mockEmit{
		{model.SourceK8sEvent, "Pod", "payment-gateway-a1", "payment-gateway-a1", "", model.SeverityCritical, "OOMKilled", "exceeded memory limit"},
		{model.SourceLog, "Pod", "payment-gateway-a1", "payment-gateway-a1", "app", model.SeverityCritical, "Memory pressure", "fatal: out of memory"},
		{model.SourceK8sEvent, "Pod", "payment-gateway-a2", "payment-gateway-a2", "", model.SeverityCritical, "OOMKilled", "exceeded memory limit"},
	}
	for _, e := range oom {
		applyMock(store, e)
	}
	after := store.State()
	if after.Verdict.LeadingSignal != "OOMKilled" {
		t.Fatalf("expected OOMKilled to lead after OOM evidence, got %q", after.Verdict.LeadingSignal)
	}
	if after.HypothesisChanges == 0 {
		t.Fatalf("expected a recorded hypothesis change")
	}
	if after.Verdict.Status != model.VerdictCritical {
		t.Fatalf("expected critical status, got %q", after.Verdict.Status)
	}
	// a system "Hypothesis changed" event should be in the buffer
	found := false
	for _, ev := range after.LiveEvidence {
		if ev.SourceType == model.SourceSystem && ev.Reason == "HypothesisChanged" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a system HypothesisChanged event in live evidence")
	}
}

func TestRingBufferDropsInfoFirst(t *testing.T) {
	rb := NewRingBuffer(3)
	crit := func(msg string) model.EvidenceEvent {
		return model.EvidenceEvent{SourceType: model.SourceK8sEvent, Severity: model.SeverityCritical, Reason: "OOMKilled", SourceName: msg, Message: msg}
	}
	info := func(msg string) model.EvidenceEvent {
		return model.EvidenceEvent{SourceType: model.SourceLog, Severity: model.SeverityInfo, Pod: "p", Container: "c", Message: msg}
	}
	rb.Add(crit("e1"))
	rb.Add(info("info line 1"))
	rb.Add(crit("e2"))
	rb.Add(crit("e3")) // over cap -> should evict the INFO log, not a critical event
	if rb.Dropped() != 1 {
		t.Fatalf("expected 1 dropped, got %d", rb.Dropped())
	}
	for _, e := range rb.Snapshot() {
		if e.SourceType == model.SourceLog {
			t.Fatalf("critical events should be kept over INFO logs")
		}
	}
}
