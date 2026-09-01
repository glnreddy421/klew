package logpatterns

import (
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// SnapshotInput supplies refresh-fetched snapshot data for pattern mining without
// requiring every line to pass through the live evidence ring.
type SnapshotInput struct {
	Events       []model.EventRecord
	Logs         []model.LogRecord
	PreviousLogs []model.LogRecord
}

// LogRecordToEvidence converts one container log line into evidence for mining.
func LogRecordToEvidence(lr model.LogRecord, line string, lineIndex, lineCount int, namespace string) model.EvidenceEvent {
	msg := strings.TrimSpace(line)
	ts := lineTimestamp(lr.CollectedAt.Time(), lineIndex, lineCount, lr.Previous)
	sev := classifyLogSeverity(msg)
	return model.EvidenceEvent{
		Timestamp:  model.TimestampFrom(ts),
		SourceType: model.SourceLog,
		SourceKind: "Pod",
		SourceName: lr.PodName,
		Namespace:  namespace,
		Pod:        lr.PodName,
		Container:  lr.ContainerName,
		Severity:   sev,
		Reason:     classifyLogReason(msg),
		Message:    msg,
		Raw:        msg,
		Count:      1,
		Confidence: 0.7,
	}
}

// LogsFromSnapshot expands snapshot log records into evidence events with
// timestamps spread across the sparkline window so minute buckets align with
// infrastructure events instead of collapsing at CollectedAt.
func LogsFromSnapshot(logs, previous []model.LogRecord, namespace string) []model.EvidenceEvent {
	total := 0
	for _, lr := range append(logs, previous...) {
		total += len(lr.Lines)
	}
	if total == 0 {
		return nil
	}
	out := make([]model.EvidenceEvent, 0, total)
	appendRecord := func(lr model.LogRecord) {
		n := len(lr.Lines)
		for i, line := range lr.Lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			out = append(out, LogRecordToEvidence(lr, line, i, n, namespace))
		}
	}
	for _, lr := range logs {
		appendRecord(lr)
	}
	for _, lr := range previous {
		appendRecord(lr)
	}
	return out
}

// mergeSnapshotLogs appends snapshot log lines not already present in the live ring.
// Keeps Log Patterns and Correlated Signals fed on periodic snapshot refresh.
func mergeSnapshotLogs(live []model.EvidenceEvent, snap SnapshotInput, max int) []model.EvidenceEvent {
	snapLogs := LogsFromSnapshot(snap.Logs, snap.PreviousLogs, "")
	if len(snapLogs) == 0 {
		return live
	}
	if max <= 0 {
		max = 500
	}

	seen := make(map[string]struct{}, len(live)/4+len(snapLogs))
	logKey := func(e model.EvidenceEvent) string {
		msg := strings.TrimSpace(e.Raw)
		if msg == "" {
			msg = strings.TrimSpace(e.Message)
		}
		return e.Pod + "|" + e.Container + "|" + msg
	}

	out := make([]model.EvidenceEvent, 0, len(live)+len(snapLogs))
	for _, e := range live {
		if e.SourceType == model.SourceLog {
			seen[logKey(e)] = struct{}{}
		}
		out = append(out, e)
	}
	for _, e := range snapLogs {
		k := logKey(e)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
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

// lineTimestamp spreads lines across recent time ending near collection time.
// Small tails (typical OOM bursts) stay within the same minute bucket(s) so
// Correlated Signals can link with infra events. Larger tails spread across
// the sparkline window oldest→newest.
func lineTimestamp(collectedAt time.Time, lineIndex, lineCount int, previous bool) time.Time {
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}
	end := collectedAt
	if previous {
		end = collectedAt.Add(-2 * time.Minute)
	}
	if lineCount <= 1 {
		return end
	}
	if lineCount <= 20 {
		offset := time.Duration(lineCount-1-lineIndex) * time.Second
		return end.Add(-offset)
	}
	span := time.Duration(defaultSparklineMinutes-1) * time.Minute
	if span < time.Minute {
		span = time.Minute
	}
	frac := float64(lineIndex) / float64(lineCount-1)
	offset := time.Duration(float64(span) * (1 - frac))
	return end.Add(-offset)
}

func classifyLogSeverity(line string) model.Severity {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "oom"), strings.Contains(l, "fatal"), strings.Contains(l, "panic"):
		return model.SeverityCritical
	case strings.Contains(l, "error"), strings.Contains(l, "failed"), strings.Contains(l, "refused"):
		return model.SeverityHigh
	case strings.Contains(l, "warn"), strings.Contains(l, "timeout"):
		return model.SeverityWarning
	default:
		return model.SeverityInfo
	}
}

func classifyLogReason(line string) string {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "allocating memory"), strings.Contains(l, "dd if="),
		strings.Contains(l, "/tmp/leak"), strings.Contains(l, "/dev/shm"):
		return "Memory allocation"
	case strings.Contains(l, "out of memory allocating"), strings.Contains(l, "cannot allocate"),
		strings.Contains(l, "memory allocation failed"):
		return "Memory allocation failures"
	case strings.Contains(l, "oom"), strings.Contains(l, "out of memory"):
		return "OOMKilled"
	case strings.Contains(l, "memory leak"), strings.Contains(l, "heap growth"):
		return "Memory leak"
	case strings.Contains(l, "redis"), strings.Contains(l, "timeout"):
		return "Redis timeout"
	case strings.Contains(l, "error"), strings.Contains(l, "failed"):
		return "Error"
	case strings.Contains(l, "warn"):
		return "Warning"
	default:
		return ""
	}
}
