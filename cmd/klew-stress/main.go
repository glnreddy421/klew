// Command klew-stress runs in-process investigation engine load tests (no cluster).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/engine"
)

func main() {
	events := flag.Int("events", 100_000, "total synthetic events to ingest (ignored when -duration > 0)")
	workers := flag.Int("workers", 4, "concurrent publisher goroutines")
	mix := flag.String("mix", "mixed", "event mix: logs | events | mixed")
	mode := flag.String("mode", "direct", "ingest path: direct (ApplyEvent) | bus (Publish + consumer)")
	duration := flag.Duration("duration", 0, "run until elapsed (overrides -events when > 0)")
	busSize := flag.Int("bus-size", 1024, "evidence bus buffer size for -mode bus")
	flag.Parse()

	cfg := engine.StressConfig{
		Events:   *events,
		Workers:  *workers,
		Mix:      engine.StressMix(strings.ToLower(strings.TrimSpace(*mix))),
		Mode:     engine.StressMode(strings.ToLower(strings.TrimSpace(*mode))),
		Duration: *duration,
		BusSize:  *busSize,
	}

	if cfg.Duration <= 0 && cfg.Events <= 0 {
		fmt.Fprintln(os.Stderr, "klew-stress: set -events or -duration")
		os.Exit(2)
	}
	switch cfg.Mix {
	case engine.StressMixLogs, engine.StressMixEvents, engine.StressMixMixed:
	default:
		fmt.Fprintf(os.Stderr, "klew-stress: unknown -mix %q (logs, events, mixed)\n", *mix)
		os.Exit(2)
	}
	switch cfg.Mode {
	case engine.StressModeDirect, engine.StressModeBus:
	default:
		fmt.Fprintf(os.Stderr, "klew-stress: unknown -mode %q (direct, bus)\n", *mode)
		os.Exit(2)
	}

	res := engine.RunStress(cfg)
	printResult(cfg, res)
}

func printResult(cfg engine.StressConfig, res engine.StressResult) {
	fmt.Printf("klew-stress: %s %s flood\n", cfg.Mix, cfg.Mode)
	if cfg.Duration > 0 {
		fmt.Printf("  duration:  %s\n", cfg.Duration.Round(time.Millisecond))
	} else {
		fmt.Printf("  events:    %d requested\n", cfg.Events)
	}
	fmt.Printf("  workers:   %d\n", cfg.Workers)
	fmt.Printf("  ingested:  %d\n", res.EventsIngested)
	fmt.Printf("  published: %d\n", res.EventsRequested)
	if res.EventsRequested > res.EventsIngested {
		fmt.Printf("  bus loss:  %d (buffer overflow or in-flight at shutdown)\n", res.EventsRequested-res.EventsIngested)
	}
	fmt.Printf("  ring drop: %d\n", res.RingDropped)
	fmt.Printf("  elapsed:   %s\n", res.Elapsed.Round(time.Millisecond))
	fmt.Printf("  throughput: %.0f events/s\n", res.EventsPerSec)
	if res.LatencyP50 > 0 {
		fmt.Printf("  apply p50: %s\n", res.LatencyP50.Round(time.Microsecond))
		fmt.Printf("  apply p99: %s\n", res.LatencyP99.Round(time.Microsecond))
	}
	fmt.Printf("  timeline:  %d (cap 400)\n", res.TimelineLen)
	fmt.Printf("  live ev:   %d (ring cap 2000)\n", res.LiveEvidenceLen)
	fmt.Printf("  verdict:   %s", res.VerdictStatus)
	if res.LeadingSignal != "" {
		fmt.Printf(" · leading=%s", res.LeadingSignal)
	}
	fmt.Println()
}
