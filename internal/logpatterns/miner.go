package logpatterns

import (
	"fmt"
	"strings"
	"sync"

	"github.com/kloudmate/drain3"
)

// Cluster budget for a single Extract / live window.
const (
	defaultMaxClusters = 512
	hardMaxClusters    = 2000
)

// Miner is a concurrency-safe Drain3 facade.
// kloudmate/drain3 already locks internally; we still serialize Add/Clusters
// so callers can share one miner across ingestion workers without racing
// Klew-side metadata that is updated alongside Add results.
type Miner struct {
	mu          sync.Mutex
	tm          *drain3.TemplateMiner
	maxClusters int
}

// newMiner configures a log-oriented Drain3 TemplateMiner (masking + LRU).
func newMiner(depth int, sim float64, maxClusters int) (*Miner, error) {
	return newDrainMiner(depth, sim, maxClusters)
}

// newEventMiner is an isolated Drain3 tree dedicated to K8s event lifecycles.
// Never shares state with the log miner.
func newEventMiner(depth int, sim float64, maxClusters int) (*Miner, error) {
	return newDrainMiner(depth, sim, maxClusters)
}

func newDrainMiner(depth int, sim float64, maxClusters int) (*Miner, error) {
	if depth <= 0 {
		depth = 4
	}
	if sim <= 0 {
		sim = 0.5
	}
	maxClusters = clampMaxClusters(maxClusters)

	tm, err := drain3.New(
		drain3.WithSimTh(sim),
		drain3.WithDepth(depth),
		drain3.WithMaxChildren(100),
		drain3.WithMaxClusters(maxClusters),
		drain3.WithExtraDelimiters("=", ":"),
		drain3.WithParametrizeNumericTokens(true),
		drain3.WithMasking(
			`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`,
			"IP",
		),
		drain3.WithMasking(
			`\b(?:(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,7}:|(?:[0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|::(?:[0-9a-fA-F]{1,4}:){0,5}[0-9a-fA-F]{1,4}|::)\b`,
			"IPV6",
		),
		drain3.WithMasking(
			`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`,
			"UUID",
		),
		drain3.WithMasking(
			`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`,
			"TS",
		),
		drain3.WithMasking(
			`\b[a-z0-9]+(?:-[a-z0-9]+)*-[a-f0-9]{8,10}-[a-z0-9]{5}\b`,
			"POD",
		),
	)
	if err != nil {
		return nil, fmt.Errorf("drain3.New: %w", err)
	}
	return &Miner{tm: tm, maxClusters: maxClusters}, nil
}

func clampMaxClusters(n int) int {
	if n <= 0 {
		return defaultMaxClusters
	}
	if n > hardMaxClusters {
		return hardMaxClusters
	}
	return n
}

// Add routes a log line through masking + Drain. Safe for concurrent callers.
func (m *Miner) Add(message string) *drain3.AddLogMessageResult {
	if m == nil || m.tm == nil || message == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tm.AddLogMessage(message)
}

// clusterSnap is an immutable copy of Drain3 cluster fields needed for View build.
// Captured under Miner.mu so BuildLogTemplates never touches live *LogCluster
// pointers after the lock is released (avoids races with concurrent Add).
type clusterSnap struct {
	ID       int
	Size     int
	Template string
}

// SnapshotClusters copies ClusterID/Size/Template under the Miner lock and
// returns immediately after unlock — safe for long copy-on-build / TF–IDF work.
func (m *Miner) SnapshotClusters() []clusterSnap {
	if m == nil || m.tm == nil {
		return nil
	}
	m.mu.Lock()
	src := m.tm.Clusters()
	out := make([]clusterSnap, 0, len(src))
	for _, c := range src {
		if c == nil {
			continue
		}
		// GetTemplate joins tokens; do it while locked so tokens aren't mutated mid-join.
		tpl := strings.TrimSpace(c.GetTemplate())
		if tpl == "" {
			continue
		}
		out = append(out, clusterSnap{
			ID:       c.ClusterID,
			Size:     c.Size,
			Template: tpl,
		})
	}
	m.mu.Unlock()
	return out
}

// Clusters returns pointer snapshots of live clusters (tests / diagnostics).
// Prefer SnapshotClusters for View payload construction.
func (m *Miner) Clusters() []*drain3.LogCluster {
	if m == nil || m.tm == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	src := m.tm.Clusters()
	out := make([]*drain3.LogCluster, len(src))
	copy(out, src)
	return out
}

// ClusterCount is the current unique-template cardinality.
func (m *Miner) ClusterCount() int {
	if m == nil || m.tm == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tm.ClusterCount()
}

// AtCapacity reports whether the LRU cluster budget is saturated.
func (m *Miner) AtCapacity() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxClusters > 0 && m.tm.ClusterCount() >= m.maxClusters
}

// MaxClusters returns the configured cluster ceiling.
func (m *Miner) MaxClusters() int {
	if m == nil {
		return 0
	}
	return m.maxClusters
}
