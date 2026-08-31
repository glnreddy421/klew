package logpatterns

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// Options tunes extraction.
type Options struct {
	MaxTemplates  int
	MaxWords      int
	MaxAttrs      int
	MaxSamples    int
	MaxLines      int
	MaxClusters   int
	MaxKeywords   int
	SparklineMins int
	DrainDepth    int
	DrainSim      float64
	ErrorOnly     bool
}

func defaultOpts(o Options) Options {
	if o.MaxTemplates <= 0 {
		o.MaxTemplates = 40
	}
	if o.MaxWords <= 0 {
		o.MaxWords = 10
	}
	if o.MaxAttrs <= 0 {
		o.MaxAttrs = 10
	}
	if o.MaxSamples <= 0 {
		o.MaxSamples = 5
	}
	if o.MaxLines <= 0 {
		o.MaxLines = 500
	}
	if o.MaxKeywords <= 0 {
		o.MaxKeywords = defaultMaxKeywords
	}
	if o.SparklineMins <= 0 {
		o.SparklineMins = defaultSparklineMinutes
	}
	o.MaxClusters = clampMaxClusters(o.MaxClusters)
	if o.DrainDepth <= 0 {
		o.DrainDepth = 4
	}
	if o.DrainSim <= 0 {
		o.DrainSim = 0.5
	}
	return o
}

type mineHalf struct {
	templates []model.LogTemplate
	words     []model.LogWord
	attrs     []model.LogAttribute
	severity  model.LogSeverityHist
	histogram []int
	window    model.LogPatternsWindow
}

// Extract mines log patterns and event patterns in parallel (isolated Drain3 trees).
// snapshot supplies Pod/Node/PVC events for Event Patterns independent of the live log ring.
func Extract(events []model.EvidenceEvent, snapshot []model.EventRecord, opts Options) model.LogPatterns {
	opts = defaultOpts(opts)
	out := emptyPatterns(opts)

	var logHalf, eventHalf mineHalf
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		logHalf = mineLogs(events, opts)
	}()
	go func() {
		defer wg.Done()
		eventHalf = mineEvents(mergeInfraEvents(events, snapshot), opts)
	}()
	wg.Wait()

	out.Templates = logHalf.templates
	out.Words = logHalf.words
	out.Attributes = logHalf.attrs
	out.Severity = logHalf.severity
	out.Histogram = logHalf.histogram
	out.Window = logHalf.window

	out.EventTemplates = eventHalf.templates
	out.EventWords = eventHalf.words
	out.EventReasons = eventHalf.attrs
	out.EventSeverity = eventHalf.severity
	out.EventHistogram = eventHalf.histogram
	out.EventWindow = eventHalf.window

	// Evidence Board: Jaccard over synchronized volume histories (lock-free).
	out.EvidenceBoard = BuildEvidenceBoard(
		out.Templates,
		out.EventTemplates,
		DefaultJaccardThreshold,
		opts.SparklineMins,
	)
	return out
}

func mineLogs(events []model.EvidenceEvent, opts Options) mineHalf {
	logs := filterLogs(events, opts)
	if len(logs) > opts.MaxLines {
		logs = logs[len(logs)-opts.MaxLines:]
	}
	h := emptyHalf(opts, "all")
	if opts.ErrorOnly {
		h.window.Scope = "signal"
	}
	if len(logs) == 0 {
		return h
	}

	miner, err := newMiner(opts.DrainDepth, opts.DrainSim, opts.MaxClusters)
	if err != nil {
		return h
	}
	meta := NewMetaStore(opts.SparklineMins, opts.MaxSamples)

	podsSeen := map[string]struct{}{}
	var wordDocs []tfDoc
	var fieldDocs []tfDoc
	analyzed := 0

	for _, e := range logs {
		raw := stripPrefix(e)
		if len(raw) < 2 {
			continue
		}
		weight := e.Count
		if weight <= 0 {
			weight = 1
		}
		analyzed += weight
		if e.Pod != "" {
			podsSeen[e.Pod] = struct{}{}
		}
		bumpSeverity(&h.severity, e.Severity, weight)

		res := miner.Add(raw)
		if res != nil && res.Cluster != nil {
			meta.Observe(res.Cluster.ClusterID, weight, e, raw)
		}
		wordDocs = append(wordDocs, tfDoc{tokens: tokenizeWords(raw), ts: e.Timestamp.Time(), weight: weight})
		if fields := extractAttributes(raw); len(fields) > 0 {
			fieldDocs = append(fieldDocs, tfDoc{tokens: fields, ts: e.Timestamp.Time(), weight: weight})
		}
	}

	total := analyzed
	if total < 1 {
		total = 1
	}
	now := time.Now().UTC()
	// Snapshot-then-build outside ingest path.
	clusters := miner.SnapshotClusters()
	metaSnap := meta.Snapshot()
	h.templates = BuildLogTemplatesFromSnapshot(
		clusters, metaSnap, total, now, opts.MaxTemplates, opts.MaxKeywords, opts.SparklineMins,
	)
	h.words = rankTopWordsTfIdf(wordDocs, opts.MaxWords)
	h.attrs = rankTopFieldsTfIdf(fieldDocs, opts.MaxAttrs)
	h.histogram = aggregateHistogram(metaSnap, now, opts.SparklineMins)
	histMax := 0
	for _, v := range h.histogram {
		if v > histMax {
			histMax = v
		}
	}
	h.window = model.LogPatternsWindow{
		LineCount:    analyzed,
		TotalLogs:    len(logs),
		PatternCount: len(h.templates),
		PodCount:     len(podsSeen),
		Scope:        h.window.Scope,
		HistMax:      histMax,
	}
	return h
}

func mineEvents(events []model.EvidenceEvent, opts Options) mineHalf {
	evs := filterInfraEvents(events, opts.MaxLines)
	h := emptyHalf(opts, "infra")
	if len(evs) == 0 {
		return h
	}

	miner, err := newEventMiner(opts.DrainDepth, opts.DrainSim, opts.MaxClusters)
	if err != nil {
		return h
	}
	meta := NewMetaStore(opts.SparklineMins, opts.MaxSamples)

	objsSeen := map[string]struct{}{}
	var wordDocs []tfDoc
	var reasonDocs []tfDoc
	analyzed := 0

	for _, e := range evs {
		line := formatEventPattern(e)
		if len(line) < 2 {
			continue
		}
		weight := e.Count
		if weight <= 0 {
			weight = 1
		}
		analyzed += weight
		obj := e.SourceName
		if obj == "" && len(e.RelatedObjectRefs) > 0 {
			obj = e.RelatedObjectRefs[0].Name
		}
		if obj != "" {
			objsSeen[obj] = struct{}{}
		}
		bumpSeverity(&h.severity, e.Severity, weight)

		// Observe with SourceName as pod slot for sample display.
		metaEv := e
		if metaEv.Pod == "" {
			metaEv.Pod = obj
		}
		res := miner.Add(line)
		if res != nil && res.Cluster != nil {
			meta.Observe(res.Cluster.ClusterID, weight, metaEv, line)
		}
		wordDocs = append(wordDocs, tfDoc{tokens: tokenizeWords(line), ts: e.Timestamp.Time(), weight: weight})
		if reason := strings.TrimSpace(e.Reason); reason != "" {
			reasonDocs = append(reasonDocs, tfDoc{tokens: []string{reason}, ts: e.Timestamp.Time(), weight: weight})
		}
	}

	total := analyzed
	if total < 1 {
		total = 1
	}
	now := time.Now().UTC()
	clusters := miner.SnapshotClusters()
	metaSnap := meta.Snapshot()
	h.templates = BuildLogTemplatesFromSnapshot(
		clusters, metaSnap, total, now, opts.MaxTemplates, opts.MaxKeywords, opts.SparklineMins,
	)
	h.words = rankTopWordsTfIdf(wordDocs, opts.MaxWords)
	h.attrs = rankTopFieldsTfIdf(reasonDocs, opts.MaxAttrs)
	h.histogram = aggregateHistogram(metaSnap, now, opts.SparklineMins)
	histMax := 0
	for _, v := range h.histogram {
		if v > histMax {
			histMax = v
		}
	}
	h.window = model.LogPatternsWindow{
		LineCount:    analyzed,
		TotalLogs:    len(evs),
		PatternCount: len(h.templates),
		PodCount:     len(objsSeen),
		Scope:        "infra",
		HistMax:      histMax,
	}
	return h
}

func emptyHalf(opts Options, scope string) mineHalf {
	n := opts.SparklineMins
	if n <= 0 {
		n = defaultSparklineMinutes
	}
	return mineHalf{
		histogram: make([]int, n),
		window: model.LogPatternsWindow{
			Scope: scope,
		},
	}
}

func emptyPatterns(opts Options) model.LogPatterns {
	h := emptyHalf(opts, "all")
	eh := emptyHalf(opts, "infra")
	return model.LogPatterns{
		Histogram:      h.histogram,
		Window:         h.window,
		EventHistogram: eh.histogram,
		EventWindow:    eh.window,
	}
}

func filterLogs(events []model.EvidenceEvent, opts Options) []model.EvidenceEvent {
	out := make([]model.EvidenceEvent, 0, len(events))
	for _, e := range events {
		if e.SourceType != model.SourceLog {
			continue
		}
		if opts.ErrorOnly && !isSignalSeverity(e.Severity) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func isSignalSeverity(s model.Severity) bool {
	switch s {
	case model.SeverityCritical, model.SeverityHigh, model.SeverityWarning:
		return true
	default:
		return false
	}
}

func bumpSeverity(h *model.LogSeverityHist, s model.Severity, n int) {
	switch s {
	case model.SeverityCritical:
		h.Fatal += n
	case model.SeverityHigh:
		h.Error += n
	case model.SeverityWarning:
		h.Warn += n
	default:
		h.Info += n
	}
}

func stripPrefix(e model.EvidenceEvent) string {
	m := e.Message
	if m == "" {
		m = e.Raw
	}
	m = stripANSIColors(m)
	if e.Pod != "" && e.Container != "" {
		m = strings.Replace(m, e.Pod+"/"+e.Container+": ", "", 1)
	}
	if e.Pod != "" {
		m = strings.Replace(m, e.Pod+": ", "", 1)
	}
	return strings.TrimSpace(m)
}

var reANSIColors = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func stripANSIColors(s string) string {
	if s == "" || !strings.ContainsRune(s, '\x1b') {
		return s
	}
	return reANSIColors.ReplaceAllString(s, "")
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func scoreTemplate(count int, sev model.Severity, trend string, podCount int) float64 {
	sevW := 1.0
	switch sev {
	case model.SeverityCritical:
		sevW = 5
	case model.SeverityHigh:
		sevW = 4
	case model.SeverityWarning:
		sevW = 2
	}
	score := float64(count) * (1 + sevW*0.15)
	switch trend {
	case "↑":
		score *= 1.2
	case "↓":
		score *= 0.9
	}
	score += float64(podCount)
	return score
}

func sevRank(s model.Severity) float64 {
	switch s {
	case model.SeverityCritical:
		return 5
	case model.SeverityHigh:
		return 4
	case model.SeverityWarning:
		return 2
	default:
		return 1
	}
}

var (
	reWord = regexp.MustCompile(`[a-z][a-z0-9_./:-]{2,}`)
	reAttr = regexp.MustCompile(`\b([a-zA-Z_][\w.-]{1,40})\s*[=:]\s*(?:"[^"]*"|'[^']*'|[^\s,;]+)`)
)
