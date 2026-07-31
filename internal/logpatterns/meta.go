package logpatterns

import (
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// Historical window for per-pattern minute buckets / sparklines.
const (
	defaultSparklineMinutes = 15
	defaultGCInterval       = time.Minute
	maxPodsPerPattern       = 48  // cap InvolvedObject / pod cardinality per cluster
	maxBucketSlack          = 4   // allow a few out-of-window keys before force-trim
)

// PatternMeta tracks side-car stats for one Drain3 cluster.
// Counts are minute-bucketed — never an unbounded []time.Time.
type PatternMeta struct {
	Count    int
	Severity model.Severity
	Pods     map[string]struct{}
	Samples  []model.LogSample
	// MinuteBuckets maps unix-minute (Truncate(time.Minute).Unix()) → weighted hits.
	MinuteBuckets map[int64]int64
}

// PatternMetaSnap is an immutable copy-on-build view of PatternMeta.
// Safe to read after the MetaStore lock is released.
type PatternMetaSnap struct {
	Count         int
	Severity      model.Severity
	Pods          []string
	Samples       []model.LogSample
	MinuteBuckets map[int64]int64
}

// MetaStore is a concurrency-safe registry of PatternMeta keyed by Drain3 cluster ID.
type MetaStore struct {
	mu             sync.RWMutex
	byCluster      map[int]*PatternMeta
	sparklineMins  int
	maxSamples     int
}

// NewMetaStore creates an empty store. sparklineMins ≤ 0 → 15.
func NewMetaStore(sparklineMins, maxSamples int) *MetaStore {
	if sparklineMins <= 0 {
		sparklineMins = defaultSparklineMinutes
	}
	if maxSamples <= 0 {
		maxSamples = 5
	}
	return &MetaStore{
		byCluster:     make(map[int]*PatternMeta),
		sparklineMins: sparklineMins,
		maxSamples:    maxSamples,
	}
}

// Observe records a matched log against clusterID. Truncates ts to the minute bucket.
func (s *MetaStore) Observe(clusterID, weight int, e model.EvidenceEvent, raw string) {
	if s == nil || clusterID < 0 {
		return
	}
	if weight <= 0 {
		weight = 1
	}
	ts := e.Timestamp.Time()
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	bucket := ts.Truncate(time.Minute).Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	m := s.byCluster[clusterID]
	if m == nil {
		m = &PatternMeta{
			Severity:      e.Severity,
			Pods:          make(map[string]struct{}, 2),
			MinuteBuckets: make(map[int64]int64, s.sparklineMins+2),
		}
		s.byCluster[clusterID] = m
	}
	m.Count += weight
	m.MinuteBuckets[bucket] += int64(weight)
	// Bound bucket map against clock-skew / future timestamps.
	if len(m.MinuteBuckets) > s.sparklineMins+maxBucketSlack {
		cutoff := ts.Truncate(time.Minute).Add(-time.Duration(s.sparklineMins) * time.Minute).Unix()
		for k := range m.MinuteBuckets {
			if k < cutoff {
				delete(m.MinuteBuckets, k)
			}
		}
		for len(m.MinuteBuckets) > s.sparklineMins+maxBucketSlack {
			oldest := int64(0)
			first := true
			for k := range m.MinuteBuckets {
				if first || k < oldest {
					oldest = k
					first = false
				}
			}
			delete(m.MinuteBuckets, oldest)
		}
	}
	if e.Pod != "" {
		if _, ok := m.Pods[e.Pod]; ok || len(m.Pods) < maxPodsPerPattern {
			m.Pods[e.Pod] = struct{}{}
		}
	}
	if sevRank(e.Severity) > sevRank(m.Severity) {
		m.Severity = e.Severity
	}
	if raw != "" && len(m.Samples) < s.maxSamples {
		m.Samples = append(m.Samples, model.LogSample{
			Message:   raw,
			Pod:       e.Pod,
			Container: e.Container,
			Timestamp: model.TimestampFrom(ts),
			Severity:  e.Severity,
		})
	}
}

// Snapshot deep-copies all pattern metadata. Caller must not retain live maps.
// Isolation barrier: returned snaps are fully detached from streaming mutations.
func (s *MetaStore) Snapshot() map[int]PatternMetaSnap {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[int]PatternMetaSnap, len(s.byCluster))
	for id, m := range s.byCluster {
		if m == nil {
			continue
		}
		out[id] = snapshotPatternMeta(m)
	}
	return out
}

// SnapshotOne copies a single cluster's meta (or zero value if missing).
func (s *MetaStore) SnapshotOne(clusterID int) (PatternMetaSnap, bool) {
	if s == nil {
		return PatternMetaSnap{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.byCluster[clusterID]
	if m == nil {
		return PatternMetaSnap{}, false
	}
	return snapshotPatternMeta(m), true
}

// Purge drops minute buckets older than the sparkline window and optionally
// removes PatternMeta for Drain3 cluster IDs that no longer exist (LRU eviction).
// Pass activeClusters=nil to only trim buckets.
func (s *MetaStore) Purge(now time.Time, activeClusters map[int]struct{}) int {
	if s == nil {
		return 0
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.UTC().Truncate(time.Minute).Add(-time.Duration(s.sparklineMins) * time.Minute).Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for id, m := range s.byCluster {
		if activeClusters != nil {
			if _, ok := activeClusters[id]; !ok {
				delete(s.byCluster, id)
				removed++
				continue
			}
		}
		if m == nil || len(m.MinuteBuckets) == 0 {
			continue
		}
		for k := range m.MinuteBuckets {
			if k < cutoff {
				delete(m.MinuteBuckets, k)
				removed++
			}
		}
	}
	return removed
}

// Len returns the number of tracked cluster metas (tests / diagnostics).
func (s *MetaStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byCluster)
}

// SparklineMinutes returns the configured historical window length.
func (s *MetaStore) SparklineMinutes() int {
	if s == nil || s.sparklineMins <= 0 {
		return defaultSparklineMinutes
	}
	return s.sparklineMins
}

func snapshotPatternMeta(m *PatternMeta) PatternMetaSnap {
	pods := sortedKeys(m.Pods)
	podCopy := make([]string, len(pods))
	copy(podCopy, pods)

	samples := make([]model.LogSample, len(m.Samples))
	copy(samples, m.Samples)

	buckets := make(map[int64]int64, len(m.MinuteBuckets))
	for k, v := range m.MinuteBuckets {
		buckets[k] = v
	}

	return PatternMetaSnap{
		Count:         m.Count,
		Severity:      m.Severity,
		Pods:          podCopy,
		Samples:       samples,
		MinuteBuckets: buckets,
	}
}

// flattenBuckets returns a fixed-length chronological series for the last n minutes.
// Missing minutes are explicit zeros. Oldest → newest.
func flattenBuckets(buckets map[int64]int64, now time.Time, n int) []int64 {
	if n <= 0 {
		n = defaultSparklineMinutes
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	end := now.UTC().Truncate(time.Minute)
	out := make([]int64, n)
	for i := 0; i < n; i++ {
		// i=0 is oldest
		t := end.Add(-time.Duration(n-1-i) * time.Minute).Unix()
		if buckets != nil {
			out[i] = buckets[t]
		}
	}
	return out
}

// trendPct: active (newest) minute vs mean of prior minutes in the sparkline.
// Returns percentage change; 0 when insufficient history.
func trendPct(spark []int64) float64 {
	if len(spark) < 2 {
		return 0
	}
	active := float64(spark[len(spark)-1])
	var sum float64
	prev := spark[:len(spark)-1]
	for _, v := range prev {
		sum += float64(v)
	}
	avg := sum / float64(len(prev))
	if avg <= 0 {
		if active > 0 {
			return 100
		}
		return 0
	}
	return (active - avg) / avg * 100
}

func trendArrow(pct float64) string {
	switch {
	case pct >= 25:
		return "↑"
	case pct <= -25:
		return "↓"
	default:
		return "·"
	}
}

// aggregateHistogram merges all pattern buckets into an n-bin sparkline (global counts).
func aggregateHistogram(meta map[int]PatternMetaSnap, now time.Time, n int) []int {
	if n <= 0 {
		n = defaultSparklineMinutes
	}
	merged := make(map[int64]int64)
	for _, m := range meta {
		for k, v := range m.MinuteBuckets {
			merged[k] += v
		}
	}
	spark := flattenBuckets(merged, now, n)
	out := make([]int, len(spark))
	for i, v := range spark {
		out[i] = int(v)
	}
	return out
}
