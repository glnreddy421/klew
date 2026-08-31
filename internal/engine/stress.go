package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// StressMix selects synthetic evidence shapes.
type StressMix string

const (
	StressMixLogs   StressMix = "logs"
	StressMixEvents StressMix = "events"
	StressMixMixed  StressMix = "mixed"
)

// StressMode selects direct reducer ingest vs bus + consumer (live path).
type StressMode string

const (
	StressModeDirect StressMode = "direct"
	StressModeBus    StressMode = "bus"
)

// StressConfig controls an in-process investigation flood.
type StressConfig struct {
	Events   int
	Workers  int
	Mix      StressMix
	Mode     StressMode
	Duration time.Duration
	BusSize  int
}

// StressResult summarizes a stress run.
type StressResult struct {
	EventsRequested int64
	EventsIngested  int64
	RingDropped     int64
	Elapsed         time.Duration
	EventsPerSec    float64
	LatencyP50      time.Duration
	LatencyP99      time.Duration
	TimelineLen     int
	LiveEvidenceLen int
	VerdictStatus   model.VerdictStatus
	LeadingSignal   string
}

// RunStress floods the investigation reducer (optionally via the evidence bus).
func RunStress(cfg StressConfig) StressResult {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.BusSize <= 0 {
		cfg.BusSize = 1024
	}
	if cfg.Mix == "" {
		cfg.Mix = StressMixMixed
	}
	if cfg.Mode == "" {
		cfg.Mode = StressModeDirect
	}

	st := mockBaseState()
	store := NewStore(&st)

	var latMu sync.Mutex
	lats := make([]time.Duration, 0, 4096)
	recordLat := func(d time.Duration) {
		latMu.Lock()
		if len(lats) < cap(lats) {
			lats = append(lats, d)
		}
		latMu.Unlock()
	}

	start := time.Now()
	var published atomic.Int64

	if cfg.Mode == StressModeBus {
		runStressBus(cfg, store, &published, recordLat)
	} else {
		runStressDirect(cfg, store, &published, recordLat)
	}

	elapsed := time.Since(start)
	final := store.State()

	p50, p99 := latencyPercentiles(lats)
	ingested := int64(final.Counters.EventsIngested)

	out := StressResult{
		EventsRequested: published.Load(),
		EventsIngested:  ingested,
		RingDropped:     int64(final.DroppedEvidence),
		Elapsed:         elapsed,
		TimelineLen:     len(final.Timeline),
		LiveEvidenceLen: len(final.LiveEvidence),
		VerdictStatus:   final.Verdict.Status,
		LeadingSignal:   final.Verdict.LeadingSignal,
		LatencyP50:      p50,
		LatencyP99:      p99,
	}
	if elapsed > 0 {
		out.EventsPerSec = float64(ingested) / elapsed.Seconds()
	}
	return out
}

func runStressDirect(cfg StressConfig, store *StateStore, published *atomic.Int64, recordLat func(time.Duration)) {
	var wg sync.WaitGroup
	stop := make(chan struct{})
	if cfg.Duration > 0 {
		go func() {
			time.Sleep(cfg.Duration)
			close(stop)
		}()
	}

	for w := 0; w < cfg.Workers; w++ {
		wg.Add(1)
		workerID := w
		go func() {
			defer wg.Done()
			i := workerID
			for {
				if cfg.Duration <= 0 {
					if int(published.Load()) >= cfg.Events {
						return
					}
				} else {
					select {
					case <-stop:
						return
					default:
					}
				}
				seq := int(published.Add(1))
				if cfg.Duration <= 0 && seq > cfg.Events {
					return
				}
				e := synthStressEvent(seq, cfg.Mix)
				t0 := time.Now()
				store.ApplyEvent(e)
				recordLat(time.Since(t0))
				i += cfg.Workers
			}
		}()
	}
	wg.Wait()
}

func runStressBus(cfg StressConfig, store *StateStore, published *atomic.Int64, recordLat func(time.Duration)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := NewBus(cfg.BusSize)
	go bus.RunConsumer(ctx, func(e model.EvidenceEvent) {
		t0 := time.Now()
		store.ApplyEvent(e)
		recordLat(time.Since(t0))
	})

	var wg sync.WaitGroup
	stop := make(chan struct{})
	if cfg.Duration > 0 {
		go func() {
			time.Sleep(cfg.Duration)
			close(stop)
			cancel()
		}()
	}

	for w := 0; w < cfg.Workers; w++ {
		wg.Add(1)
		workerID := w
		go func() {
			defer wg.Done()
			i := workerID
			for {
				if cfg.Duration <= 0 {
					if int(published.Load()) >= cfg.Events {
						return
					}
				} else {
					select {
					case <-stop:
						return
					default:
					}
				}
				seq := int(published.Add(1))
				if cfg.Duration <= 0 && seq > cfg.Events {
					return
				}
				bus.Publish(synthStressEvent(seq, cfg.Mix))
				i += cfg.Workers
			}
		}()
	}
	wg.Wait()

	// Drain remaining buffered events (bus drops overflow during flood — do not wait for full publish count).
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(bus.ch) == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	cancel()
	bus.Close()
}

func synthStressEvent(i int, mix StressMix) model.EvidenceEvent {
	switch mix {
	case StressMixEvents:
		return synthK8sEvent(i)
	case StressMixMixed:
		if i%3 == 0 {
			return synthK8sEvent(i)
		}
		return synthLogEvent(i)
	default:
		return synthLogEvent(i)
	}
}

func synthLogEvent(i int) model.EvidenceEvent {
	pod := fmt.Sprintf("payment-gateway-a%d", (i%4)+1)
	msg := fmt.Sprintf("stress log line %d: connection timeout to redis shard=%d", i, i%128)
	return model.EvidenceEvent{
		Timestamp:  model.TimestampFrom(time.Now().UTC()),
		SourceType: model.SourceLog,
		SourceKind: "Pod",
		SourceName: pod,
		Namespace:  "prod",
		Pod:        pod,
		Container:  "app",
		Severity:   stressLogSeverity(i),
		Message:    msg,
		Raw:        fmt.Sprintf("%s/app: %s", pod, msg),
	}
}

func synthK8sEvent(i int) model.EvidenceEvent {
	pod := fmt.Sprintf("payment-gateway-a%d", (i%4)+1)
	// Every 20th event repeats reason/message to exercise collapse paths.
	reason := "Unhealthy"
	msg := fmt.Sprintf("Readiness probe failed: HTTP probe failed with statuscode: 503 (seq=%d)", i)
	if i%20 != 0 {
		reason = "FailedScheduling"
		msg = "0/4 nodes are available: insufficient memory"
	}
	sev := model.SeverityWarning
	if i%17 == 0 {
		sev = model.SeverityCritical
		reason = "OOMKilled"
		msg = "Container exceeded memory limit"
	}
	return model.EvidenceEvent{
		Timestamp:  model.TimestampFrom(time.Now().UTC()),
		SourceType: model.SourceK8sEvent,
		SourceKind: "Pod",
		SourceName: pod,
		Namespace:  "prod",
		Pod:        pod,
		Severity:   sev,
		Reason:     reason,
		Message:    msg,
		Raw:        msg,
	}
}

func stressLogSeverity(i int) model.Severity {
	switch {
	case i%23 == 0:
		return model.SeverityCritical
	case i%11 == 0:
		return model.SeverityHigh
	case i%7 == 0:
		return model.SeverityWarning
	default:
		return model.SeverityInfo
	}
}

func latencyPercentiles(samples []time.Duration) (p50, p99 time.Duration) {
	if len(samples) == 0 {
		return 0, 0
	}
	cp := append([]time.Duration(nil), samples...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	p50 = cp[len(cp)*50/100]
	p99 = cp[len(cp)*99/100]
	if p99 == 0 {
		p99 = cp[len(cp)-1]
	}
	return p50, p99
}
