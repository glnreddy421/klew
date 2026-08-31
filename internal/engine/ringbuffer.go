package engine

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// RingBuffer is a bounded, newest-first store of live evidence.
//
// Policy:
//   - identical events (same fingerprint) are collapsed and their Count bumped
//   - when full, the oldest low-value INFO logs are dropped first
//   - critical Kubernetes events are never dropped to make room
//   - dropped counts are tracked and exposed
type RingBuffer struct {
	mu        sync.Mutex
	cap       int
	events    []model.EvidenceEvent // index 0 == newest
	index     map[string]int        // fingerprint -> position
	dropped   int64
	droppedBy map[model.Severity]int64
}

// NewRingBuffer creates a buffer bounded to capacity events.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 500
	}
	return &RingBuffer{
		cap:       capacity,
		index:     make(map[string]int, capacity),
		droppedBy: make(map[model.Severity]int64),
	}
}

// Add inserts an event, collapsing duplicates and evicting under pressure.
func (r *RingBuffer) Add(e model.EvidenceEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e.Fingerprint == "" {
		e.Fingerprint = Fingerprint(e)
	}
	if e.Count == 0 {
		e.Count = 1
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = model.TimestampFrom(time.Now().UTC())
	}

	// Log lines are never collapsed — each tail line is distinct evidence.
	if e.SourceType == model.SourceLog {
		e.Fingerprint = logLineFingerprint(e)
		r.pushFront(e)
		for len(r.events) > r.cap {
			r.evict()
		}
		return
	}

	// collapse duplicate: bump count, refresh, move to front
	if pos, ok := r.index[e.Fingerprint]; ok && pos < len(r.events) {
		prev := r.events[pos]
		e.Count = prev.Count + 1
		if severityRank(prev.Severity) > severityRank(e.Severity) {
			e.Severity = prev.Severity
		}
		r.remove(pos)
		r.pushFront(e)
		return
	}

	r.pushFront(e)
	for len(r.events) > r.cap {
		r.evict()
	}
}

func (r *RingBuffer) pushFront(e model.EvidenceEvent) {
	r.events = append([]model.EvidenceEvent{e}, r.events...)
	r.reindex()
}

func (r *RingBuffer) remove(pos int) {
	r.events = append(r.events[:pos], r.events[pos+1:]...)
	r.reindex()
}

func (r *RingBuffer) reindex() {
	r.index = make(map[string]int, len(r.events))
	for i, e := range r.events {
		r.index[e.Fingerprint] = i
	}
}

// evict drops one event, preferring the oldest low-value INFO log and never
// dropping Kubernetes events unless nothing else is available.
func (r *RingBuffer) evict() {
	if len(r.events) == 0 {
		return
	}
	// 1) oldest INFO log
	if pos := r.lastMatch(func(e model.EvidenceEvent) bool {
		return e.SourceType == model.SourceLog && e.Severity == model.SeverityInfo
	}); pos >= 0 {
		r.dropAt(pos)
		return
	}
	// 2) oldest log of any severity (protect k8s_event for Infrastructure Patterns)
	if pos := r.lastMatch(func(e model.EvidenceEvent) bool {
		return e.SourceType == model.SourceLog
	}); pos >= 0 {
		r.dropAt(pos)
		return
	}
	// 3) oldest non-event noise (object_change / system / metric)
	if pos := r.lastMatch(func(e model.EvidenceEvent) bool {
		return e.SourceType != model.SourceK8sEvent
	}); pos >= 0 {
		r.dropAt(pos)
		return
	}
	// 4) only k8s events left — drop oldest non-critical, else oldest
	if pos := r.lastMatch(func(e model.EvidenceEvent) bool {
		return e.Severity != model.SeverityCritical
	}); pos >= 0 {
		r.dropAt(pos)
		return
	}
	r.dropAt(len(r.events) - 1)
}

func (r *RingBuffer) lastMatch(pred func(model.EvidenceEvent) bool) int {
	for i := len(r.events) - 1; i >= 0; i-- {
		if pred(r.events[i]) {
			return i
		}
	}
	return -1
}

func (r *RingBuffer) dropAt(pos int) {
	e := r.events[pos]
	r.dropped++
	r.droppedBy[e.Severity]++
	r.remove(pos)
}

// KeepLogsForPods removes log lines from pods outside the allowlist.
func (r *RingBuffer) KeepLogsForPods(podNames []string) {
	if len(podNames) == 0 {
		return
	}
	allow := make(map[string]bool, len(podNames))
	for _, n := range podNames {
		if n != "" {
			allow[n] = true
		}
	}
	if len(allow) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	filtered := r.events[:0]
	for _, e := range r.events {
		if e.SourceType == model.SourceLog && !allow[e.Pod] {
			continue
		}
		filtered = append(filtered, e)
	}
	r.events = filtered
	r.reindex()
}

// ClearLogsForPods removes log lines for the named pods only.
func (r *RingBuffer) ClearLogsForPods(podNames []string) {
	if len(podNames) == 0 {
		return
	}
	drop := make(map[string]bool, len(podNames))
	for _, n := range podNames {
		if n != "" {
			drop[n] = true
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	filtered := r.events[:0]
	for _, e := range r.events {
		if e.SourceType == model.SourceLog && drop[e.Pod] {
			continue
		}
		filtered = append(filtered, e)
	}
	r.events = filtered
	r.reindex()
}

// ClearLogs removes all buffered log lines and resets drop counters.
func (r *RingBuffer) ClearLogs() {
	r.mu.Lock()
	defer r.mu.Unlock()
	filtered := r.events[:0]
	for _, e := range r.events {
		if e.SourceType != model.SourceLog {
			filtered = append(filtered, e)
		}
	}
	r.events = filtered
	r.reindex()
	r.dropped = 0
	r.droppedBy = make(map[model.Severity]int64)
}

// Snapshot returns a newest-first copy of buffered events.
func (r *RingBuffer) Snapshot() []model.EvidenceEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.EvidenceEvent, len(r.events))
	copy(out, r.events)
	return out
}

// Dropped returns the total number of evicted events.
func (r *RingBuffer) Dropped() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

// Len returns the current number of buffered events.
func (r *RingBuffer) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

var digitRe = regexp.MustCompile(`\d+`)

// Fingerprint derives a stable identity used to collapse duplicate evidence.
func Fingerprint(e model.EvidenceEvent) string {
	var key string
	switch e.SourceType {
	case model.SourceLog:
		key = "log|" + e.Pod + "|" + e.Container + "|" + normalizeMsg(e.Raw+e.Message)
	case model.SourceK8sEvent:
		key = "event|" + e.Reason + "|" + e.SourceKind + "|" + e.SourceName
	case model.SourceObjectChange:
		key = "obj|" + e.SourceKind + "|" + e.SourceName + "|" + e.Reason
	case model.SourceMetric:
		key = "metric|" + e.SourceName + "|" + e.Reason
	default:
		key = "sys|" + e.Reason + "|" + normalizeMsg(e.Message)
	}
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:8])
}

func normalizeMsg(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = digitRe.ReplaceAllString(s, "#")
	return strings.Join(strings.Fields(s), " ")
}

// logLineFingerprint uniquely identifies one tailed log line (no digit normalization).
func logLineFingerprint(e model.EvidenceEvent) string {
	key := "logline|" + e.Pod + "|" + e.Container + "|" + strings.TrimSpace(e.Raw) + "|" + e.Timestamp.Time().Format(time.RFC3339Nano)
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:8])
}
