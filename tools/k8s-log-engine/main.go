// Command k8s-log-engine tails Kubernetes-style logs, mines templates with
// Drain3 (github.com/kloudmate/drain3), and ranks anomaly keywords via TF–IDF.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/glnreddy421/klew/tools/k8s-log-engine/internal/engine"
	"github.com/glnreddy421/klew/tools/k8s-log-engine/internal/tail"
)

func main() {
	file := flag.String("f", "", "log file to tail (JSON / logfmt lines); empty = simulator")
	workers := flag.Int("workers", 4, "concurrent Drain3/TF–IDF workers")
	top := flag.Int("top", 5, "top keywords per cluster")
	interval := flag.Duration("interval", 2*time.Second, "dashboard refresh interval")
	simCount := flag.Int("sim-count", 0, "simulator line cap (0 = until signal)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var source tail.Source
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open %s: %v\n", *file, err)
			os.Exit(1)
		}
		defer f.Close()
		source = &tail.ReaderSource{R: f}
	} else {
		source = &tail.Simulator{Count: *simCount}
		fmt.Fprintln(os.Stderr, "k8s-log-engine: simulating live log stream (Ctrl-C to stop)")
	}

	orch, err := engine.New(engine.Config{
		Workers:        *workers,
		DashboardEvery: *interval,
		TopKeywords:    *top,
	}, source, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "engine: %v\n", err)
		os.Exit(1)
	}

	if err := orch.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}
