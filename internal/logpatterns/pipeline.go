package logpatterns

import (
	"context"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// DualPipeline owns two isolated Drain3 trees:
//   - Logs:   application container log lines
//   - Events: Pod / Node / PVC infrastructure lifecycle events
//
// Each side has its own MetaStore (minute buckets) and GC ticker.
// Snapshot-then-build never holds ingest locks during TF–IDF / correlation.
type DualPipeline struct {
	Logs   *Tracker
	Events *Tracker
}

// DualPipelineConfig tunes both halves. Zero values inherit Tracker defaults.
type DualPipelineConfig struct {
	Logs   TrackerConfig
	Events TrackerConfig
}

// NewDualPipeline constructs isolated LogMiner + EventMiner trackers.
func NewDualPipeline(cfg DualPipelineConfig) (*DualPipeline, error) {
	logs, err := NewTracker(cfg.Logs)
	if err != nil {
		return nil, err
	}
	events, err := NewTracker(cfg.Events)
	if err != nil {
		return nil, err
	}
	return &DualPipeline{Logs: logs, Events: events}, nil
}

// StartGC launches background bucket purgers on both halves.
func (p *DualPipeline) StartGC(parent context.Context) {
	if p == nil {
		return
	}
	p.Logs.StartGC(parent)
	p.Events.StartGC(parent)
}

// Stop cancels both GC workers.
func (p *DualPipeline) Stop() {
	if p == nil {
		return
	}
	p.Logs.Stop()
	p.Events.Stop()
}

// IngestLog routes a container log line into the LogMiner half.
func (p *DualPipeline) IngestLog(raw string, weight int, e model.EvidenceEvent) {
	if p == nil || p.Logs == nil {
		return
	}
	p.Logs.Ingest(raw, weight, e)
}

// IngestEvent routes a K8s event into the EventMiner half.
// Guard: only Pod / Node / PersistentVolumeClaim InvolvedObject kinds are accepted.
func (p *DualPipeline) IngestEvent(e model.EvidenceEvent) {
	if p == nil || p.Events == nil {
		return
	}
	if e.SourceType != model.SourceK8sEvent {
		return
	}
	if !allowInfraEventKind(e.SourceKind) {
		return
	}
	line := formatEventPattern(e)
	if line == "" {
		return
	}
	metaEv := e
	if metaEv.Pod == "" {
		metaEv.Pod = e.SourceName
		if metaEv.Pod == "" && len(e.RelatedObjectRefs) > 0 {
			metaEv.Pod = e.RelatedObjectRefs[0].Name
		}
	}
	weight := e.Count
	if weight <= 0 {
		weight = 1
	}
	p.Events.Ingest(line, weight, metaEv)
}

// BuildSnapshots captures both halves under their ingest locks, then builds
// templates + Evidence Board entirely outside the critical section.
func (p *DualPipeline) BuildSnapshots(
	now time.Time,
	logLines, eventLines int,
	maxTemplates, maxKeywords int,
) (logs []model.LogTemplate, events []model.LogTemplate, board *model.EvidenceBoardPayload) {
	if p == nil {
		return nil, nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	logCap := p.Logs.CaptureSnapshot(now)
	evCap := p.Events.CaptureSnapshot(now)

	// Isolation barrier: Build* runs with zero ingest locks held.
	logs = p.Logs.BuildView(logCap, logLines, maxTemplates, maxKeywords)
	events = p.Events.BuildView(evCap, eventLines, maxTemplates, maxKeywords)
	mins := defaultSparklineMinutes
	if p.Logs != nil && p.Logs.meta != nil {
		mins = p.Logs.Meta().SparklineMinutes()
	}
	board = BuildEvidenceBoard(logs, events, DefaultJaccardThreshold, mins)
	return logs, events, board
}
