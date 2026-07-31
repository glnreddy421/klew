package logpatterns

import (
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// allowInfraEventKind is the high-throughput guard for EventMiner ingest.
// Keeps Pod / Node / PVC lifecycle signals; drops Deployment/Service/ConfigMap noise.
func allowInfraEventKind(kind string) bool {
	switch kind {
	case "Pod", "Node", "PersistentVolumeClaim":
		return true
	default:
		return false
	}
}

// AllowInfraEventKind reports whether InvolvedObject kind is mined for Infrastructure Patterns.
func AllowInfraEventKind(kind string) bool { return allowInfraEventKind(kind) }

// formatEventPattern builds the compound Drain3 document for a K8s event.
func formatEventPattern(e model.EvidenceEvent) string {
	reason := strings.TrimSpace(e.Reason)
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = strings.TrimSpace(e.Raw)
	}
	if reason == "" {
		return msg
	}
	if msg == "" {
		return "[" + reason + "]"
	}
	return "[" + reason + "] " + msg
}

// EventRecordToEvidence converts a snapshot EventRecord into live-style evidence.
func EventRecordToEvidence(e model.EventRecord) model.EvidenceEvent {
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	sev := model.SeverityWarning
	switch e.Reason {
	case "OOMKilling", "Failed", "FailedScheduling", "FailedMount", "Killing":
		sev = model.SeverityCritical
	case "BackOff", "Unhealthy", "FailedCreatePodSandBox", "NetworkNotReady":
		sev = model.SeverityHigh
	}
	count := int(e.Count)
	if count <= 0 {
		count = 1
	}
	return model.EvidenceEvent{
		Timestamp:  ts,
		SourceType: model.SourceK8sEvent,
		SourceKind: e.InvolvedObject.Kind,
		SourceName: e.InvolvedObject.Name,
		Namespace:  e.InvolvedObject.Namespace,
		Severity:   sev,
		Reason:     e.Reason,
		Message:    e.Message,
		Raw:        e.Message,
		Count:      count,
		Confidence: 0.85,
		RelatedObjectRefs: []model.ObjectRef{
			e.InvolvedObject,
		},
	}
}

// MergeSnapshotEvents appends Pod/Node/PVC snapshot events not already present in live.
// Keeps Infrastructure Patterns fed even when the live ring is dominated by log lines.
func MergeSnapshotEvents(live []model.EvidenceEvent, snap []model.EventRecord) []model.EvidenceEvent {
	if len(snap) == 0 {
		return live
	}
	seen := make(map[string]struct{}, len(live)/4+len(snap))
	key := func(kind, name, reason, msg string) string {
		return kind + "|" + name + "|" + reason + "|" + msg
	}
	for _, e := range live {
		if e.SourceType != model.SourceK8sEvent {
			continue
		}
		seen[key(e.SourceKind, e.SourceName, e.Reason, e.Message)] = struct{}{}
	}
	out := append([]model.EvidenceEvent(nil), live...)
	for _, rec := range snap {
		if !allowInfraEventKind(rec.InvolvedObject.Kind) {
			continue
		}
		k := key(rec.InvolvedObject.Kind, rec.InvolvedObject.Name, rec.Reason, rec.Message)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, EventRecordToEvidence(rec))
	}
	return out
}

// filterInfraEvents selects k8s_event evidence that passes the kind allowlist.
// Returns newest-first, capped at max.
func filterInfraEvents(events []model.EvidenceEvent, max int) []model.EvidenceEvent {
	if max <= 0 {
		max = 500
	}
	capHint := max
	if len(events) < capHint {
		capHint = len(events)
	}
	out := make([]model.EvidenceEvent, 0, capHint)
	for _, e := range events {
		if e.SourceType != model.SourceK8sEvent {
			continue
		}
		if !allowInfraEventKind(e.SourceKind) {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	if len(out) > max {
		out = out[:max]
	}
	return out
}
