package kube

import (
	"context"
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// MetricsPoller reports whether metrics.k8s.io is available. Aggregate usage is
// refreshed through the snapshot loop (CollectMetrics); this only emits a single
// accurate note so the Live Evidence Stream reflects the metrics source.
type MetricsPoller struct {
	Client    *Client
	Sink      EvidenceSink
	Namespace string
}

// Start probes metrics availability once and returns true if metrics-server
// responded. It never spams per-pod stub events.
func (m *MetricsPoller) Start(ctx context.Context, pods []model.PodSummary, wg *sync.WaitGroup) bool {
	if m.Sink == nil || len(pods) == 0 {
		return false
	}
	ms := CollectMetrics(ctx, m.Client, pods)
	sev := model.SeverityInfo
	msg := "Metrics source: metrics-server (live CPU/memory usage)"
	if !ms.Available {
		sev = model.SeverityWarning
		msg = "metrics-server not available — showing pod requests/limits only"
	}
	m.Sink(model.EvidenceEvent{
		Timestamp: model.TimestampFrom(time.Now()), SourceType: model.SourceSystem,
		Severity: sev, Reason: "metrics_source", Message: msg, Confidence: 1,
	})
	_ = wg
	return ms.Available
}

