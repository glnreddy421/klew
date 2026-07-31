package logpatterns

import (
	"context"
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// Tracker owns a Drain3 Miner + MetaStore with a background bucket GC.
// Ingest holds a short critical section; View builds run lock-free on snapshots.
type Tracker struct {
	mu sync.Mutex // orders Ingest vs Capture (not held during Build)

	miner *Miner
	meta  *MetaStore

	gcEvery time.Duration

	stop context.CancelFunc
	wg   sync.WaitGroup
}

// TrackerConfig tunes Drain3 + sparkline window + GC cadence.
type TrackerConfig struct {
	DrainDepth      int
	DrainSim        float64
	MaxClusters     int
	SparklineMins   int
	MaxSamples      int
	GCInterval      time.Duration
}

// NewTracker constructs a miner + metadata store. Call StartGC for eviction.
func NewTracker(cfg TrackerConfig) (*Tracker, error) {
	if cfg.SparklineMins <= 0 {
		cfg.SparklineMins = defaultSparklineMinutes
	}
	if cfg.GCInterval <= 0 {
		cfg.GCInterval = defaultGCInterval
	}
	miner, err := newMiner(cfg.DrainDepth, cfg.DrainSim, cfg.MaxClusters)
	if err != nil {
		return nil, err
	}
	return &Tracker{
		miner:   miner,
		meta:    NewMetaStore(cfg.SparklineMins, cfg.MaxSamples),
		gcEvery: cfg.GCInterval,
	}, nil
}

// StartGC launches a ticker that purges minute buckets outside the sparkline window.
func (t *Tracker) StartGC(parent context.Context) {
	if t == nil || t.stop != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	t.stop = cancel
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		tick := time.NewTicker(t.gcEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-tick.C:
				t.gcOnce(now.UTC())
			}
		}
	}()
}

// gcOnce trims stale minute buckets and drops meta for LRU-evicted Drain3 clusters.
func (t *Tracker) gcOnce(now time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	snaps := t.miner.SnapshotClusters()
	active := make(map[int]struct{}, len(snaps))
	for _, c := range snaps {
		active[c.ID] = struct{}{}
	}
	t.meta.Purge(now, active)
	t.mu.Unlock()
}

// Stop cancels the GC worker and waits for exit.
func (t *Tracker) Stop() {
	if t == nil || t.stop == nil {
		return
	}
	t.stop()
	t.wg.Wait()
	t.stop = nil
}

// Ingest routes a log line through Drain3 and records minute-bucket metadata.
// Critical section is brief: Add + Observe only.
func (t *Tracker) Ingest(raw string, weight int, e model.EvidenceEvent) {
	if t == nil || raw == "" {
		return
	}
	if weight <= 0 {
		weight = 1
	}
	t.mu.Lock()
	res := t.miner.Add(raw)
	if res != nil && res.Cluster != nil {
		t.meta.Observe(res.Cluster.ClusterID, weight, e, raw)
	}
	t.mu.Unlock()
}

// Capture is the isolation barrier: under lock, snapshot Drain3 primitives and
// deep-copy metadata maps, then release immediately.
type Capture struct {
	Clusters []clusterSnap
	Meta     map[int]PatternMetaSnap
	Now      time.Time
}

// CaptureSnapshot freezes miner + meta for a lock-free BuildLogTemplates pass.
func (t *Tracker) CaptureSnapshot(now time.Time) Capture {
	if t == nil {
		return Capture{}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	t.mu.Lock()
	clusters := t.miner.SnapshotClusters() // releases miner.mu internally; still under t.mu
	meta := t.meta.Snapshot()
	t.mu.Unlock()
	return Capture{Clusters: clusters, Meta: meta, Now: now.UTC()}
}

// BuildView transforms a prior Capture into JSON-ready templates (no locks held).
func (t *Tracker) BuildView(cap Capture, totalLines, maxTemplates, maxKeywords int) []model.LogTemplate {
	return BuildLogTemplatesFromSnapshot(cap.Clusters, cap.Meta, totalLines, cap.Now, maxTemplates, maxKeywords, t.meta.SparklineMinutes())
}

// Miner exposes the underlying Drain3 facade (tests / diagnostics).
func (t *Tracker) Miner() *Miner { return t.miner }

// Meta exposes the metadata store (tests / diagnostics).
func (t *Tracker) Meta() *MetaStore { return t.meta }
