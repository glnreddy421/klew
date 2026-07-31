package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kloudmate/drain3"

	"github.com/glnreddy421/klew/tools/k8s-log-engine/internal/draincfg"
	"github.com/glnreddy421/klew/tools/k8s-log-engine/internal/tail"
	"github.com/glnreddy421/klew/tools/k8s-log-engine/internal/tfidf"
)

// Config tunes the concurrent pipeline.
type Config struct {
	Workers        int
	ChannelBuffer  int
	DashboardEvery time.Duration
	TopKeywords    int
	MaxDashboard   int
}

func (c Config) withDefaults() Config {
	if c.Workers <= 0 {
		c.Workers = 4
	}
	if c.ChannelBuffer <= 0 {
		c.ChannelBuffer = 4096
	}
	if c.DashboardEvery <= 0 {
		c.DashboardEvery = 2 * time.Second
	}
	if c.TopKeywords <= 0 {
		c.TopKeywords = 5
	}
	if c.MaxDashboard <= 0 {
		c.MaxDashboard = 12
	}
	return c
}

// Orchestrator owns Drain3 + TF–IDF workers and the dashboard loop.
type Orchestrator struct {
	cfg    Config
	source tail.Source
	out    io.Writer

	miner *drain3.TemplateMiner
	tfidf *tfidf.Engine

	processed atomic.Uint64
}

// New builds an Orchestrator. source must be non-nil.
func New(cfg Config, source tail.Source, out io.Writer) (*Orchestrator, error) {
	cfg = cfg.withDefaults()
	if source == nil {
		return nil, fmt.Errorf("nil log source")
	}
	if out == nil {
		out = os.Stdout
	}
	miner, err := draincfg.NewMiner()
	if err != nil {
		return nil, err
	}
	return &Orchestrator{
		cfg:    cfg,
		source: source,
		out:    out,
		miner:  miner,
		tfidf:  tfidf.New(),
	}, nil
}

// Run starts the tailer, workers, and dashboard until ctx is cancelled
// or the source reaches EOF.
func (o *Orchestrator) Run(ctx context.Context) error {
	events := make(chan tail.Event, o.cfg.ChannelBuffer)

	var prodWG, workWG, dashWG sync.WaitGroup
	errCh := make(chan error, 1)

	prodWG.Add(1)
	go func() {
		defer prodWG.Done()
		defer close(events)
		if err := o.source.Run(ctx, events); err != nil && ctx.Err() == nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	for i := 0; i < o.cfg.Workers; i++ {
		workWG.Add(1)
		go func() {
			defer workWG.Done()
			o.worker(ctx, events)
		}()
	}

	dashCtx, cancelDash := context.WithCancel(ctx)
	defer cancelDash()
	dashWG.Add(1)
	go func() {
		defer dashWG.Done()
		o.dashboard(dashCtx)
	}()

	pipelineDone := make(chan struct{})
	go func() {
		prodWG.Wait()
		workWG.Wait()
		close(pipelineDone)
	}()

	var runErr error
	select {
	case <-ctx.Done():
		runErr = ctx.Err()
	case err := <-errCh:
		runErr = err
	case <-pipelineDone:
		runErr = nil
	}

	cancelDash()
	workWG.Wait()
	dashWG.Wait()
	o.printDashboard()
	return runErr
}

func (o *Orchestrator) worker(ctx context.Context, in <-chan tail.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-in:
			if !ok {
				return
			}
			if ev.Message == "" {
				continue
			}
			res := o.miner.AddLogMessage(ev.Message)
			if res.Cluster == nil {
				continue
			}
			switch res.ChangeType {
			case drain3.ChangeClusterCreated, drain3.ChangeClusterTemplateChanged:
				o.tfidf.UpsertDocument(res.Cluster.ClusterID, res.Cluster.GetTemplate())
			}
			o.processed.Add(1)
		}
	}
}

func (o *Orchestrator) dashboard(ctx context.Context) {
	t := time.NewTicker(o.cfg.DashboardEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			o.printDashboard()
		}
	}
}

func (o *Orchestrator) printDashboard() {
	clusters := o.miner.Clusters()
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Size != clusters[j].Size {
			return clusters[i].Size > clusters[j].Size
		}
		return clusters[i].ClusterID < clusters[j].ClusterID
	})
	if len(clusters) > o.cfg.MaxDashboard {
		clusters = clusters[:o.cfg.MaxDashboard]
	}

	fmt.Fprintf(o.out, "\n── k8s-log-engine  processed=%d  clusters=%d  docs=%d ──\n",
		o.processed.Load(), o.miner.ClusterCount(), o.tfidf.DocCount())

	for _, c := range clusters {
		keywords := o.tfidf.TopN(c.ClusterID, o.cfg.TopKeywords)
		fmt.Fprintf(o.out, "[id=%d size=%d] %s\n", c.ClusterID, c.Size, c.GetTemplate())
		if len(keywords) == 0 {
			fmt.Fprintf(o.out, "    keywords: (none)\n")
			continue
		}
		fmt.Fprintf(o.out, "    keywords:")
		for i, k := range keywords {
			if i > 0 {
				fmt.Fprint(o.out, ",")
			}
			fmt.Fprintf(o.out, " %s(%.2f)", k.Word, k.Score)
		}
		fmt.Fprintln(o.out)
	}
}
