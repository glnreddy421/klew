package logpatterns_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/logpatterns"
	"github.com/glnreddy421/klew/internal/model"
)

func TestExtractEventPatternsAllowlist(t *testing.T) {
	now := time.Now().UTC()
	ev := []model.EvidenceEvent{
		{
			Timestamp: now, SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "api-1",
			Reason: "Failed", Message: "Failed to pull image nginx:latest", Severity: model.SeverityHigh, Count: 1,
		},
		{
			Timestamp: now, SourceType: model.SourceK8sEvent, SourceKind: "Deployment", SourceName: "api",
			Reason: "ScalingReplicaSet", Message: "Scaled up replica set", Severity: model.SeverityInfo, Count: 1,
		},
		{
			Timestamp: now, SourceType: model.SourceK8sEvent, SourceKind: "Node", SourceName: "node-1",
			Reason: "NodeNotReady", Message: "Node is not ready", Severity: model.SeverityCritical, Count: 1,
		},
		{
			Timestamp: now, SourceType: model.SourceK8sEvent, SourceKind: "PersistentVolumeClaim", SourceName: "data",
			Reason: "FailedBinding", Message: "no persistent volumes available", Severity: model.SeverityHigh, Count: 1,
		},
		{
			Timestamp: now, SourceType: model.SourceLog, Pod: "api-1", Container: "app",
			Message: "api-1/app: dial tcp 10.0.0.1:5432: connection refused", Severity: model.SeverityHigh, Count: 1,
		},
	}
	out := logpatterns.Extract(ev, logpatterns.Options{})
	if out.EventWindow.Scope != "infra" {
		t.Fatalf("eventWindow.scope=%s", out.EventWindow.Scope)
	}
	if len(out.EventTemplates) == 0 {
		t.Fatalf("expected event templates, got %#v", out.EventTemplates)
	}
	if out.EvidenceBoard == nil {
		t.Fatal("expected EvidenceBoard payload")
	}
	if len(out.EventTemplates[0].VolumeHistory) == 0 && len(out.EventTemplates[0].Sparkline) == 0 {
		t.Fatal("expected volume history / sparkline on event templates")
	}
	for _, tpl := range out.EventTemplates {
		if strings.Contains(strings.ToLower(tpl.Template), "scalingreplicaset") {
			t.Fatalf("Deployment boilerplate leaked into EventTemplates: %q", tpl.Template)
		}
	}
	if out.EventWindow.LineCount < 3 {
		t.Fatalf("expected ≥3 allowed events, lineCount=%d", out.EventWindow.LineCount)
	}
	if len(out.Templates) == 0 {
		t.Fatal("expected log templates mined in parallel")
	}
}

func TestMergeSnapshotEventsFeedsExtract(t *testing.T) {
	now := time.Now().UTC()
	// Live ring has only logs — snapshot carries the infra events.
	live := []model.EvidenceEvent{{
		Timestamp: now, SourceType: model.SourceLog, Pod: "p",
		Message: "p/c: hello", Severity: model.SeverityInfo, Count: 1,
	}}
	snap := []model.EventRecord{{
		Timestamp: now, Reason: "FailedMount", Message: "mount volume failed", Count: 2,
		InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "p", Namespace: "ns"},
	}, {
		Timestamp: now, Reason: "ScalingReplicaSet", Message: "scaled", Count: 1,
		InvolvedObject: model.ObjectRef{Kind: "Deployment", Name: "api", Namespace: "ns"},
	}}
	merged := logpatterns.MergeSnapshotEvents(live, snap)
	out := logpatterns.Extract(merged, logpatterns.Options{})
	if len(out.EventTemplates) == 0 {
		t.Fatalf("expected event templates from snapshot merge, got %#v", out)
	}
	if out.EventWindow.LineCount < 1 {
		t.Fatalf("lineCount=%d", out.EventWindow.LineCount)
	}
}

func TestFormatEventPatternCompound(t *testing.T) {
	// Exercised via Extract: compound [Reason] Message must appear in samples/templates.
	now := time.Now().UTC()
	ev := []model.EvidenceEvent{{
		Timestamp: now, SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "p",
		Reason: "BackOff", Message: "Back-off restarting failed container", Severity: model.SeverityHigh, Count: 1,
	}}
	out := logpatterns.Extract(ev, logpatterns.Options{})
	found := false
	for _, tpl := range out.EventTemplates {
		if strings.Contains(tpl.Template, "BackOff") || strings.Contains(tpl.Template, "Back-off") {
			found = true
		}
		for _, s := range tpl.Samples {
			if strings.HasPrefix(s.Message, "[BackOff]") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected [Reason] Message compound pattern, got %#v", out.EventTemplates)
	}
}
