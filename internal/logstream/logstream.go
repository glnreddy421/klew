// Package logstream follows container logs for pods the user picks, with or
// without an investigation running. It only reads: it gets workloads to learn
// their selectors, lists and watches pods, and follows their logs.
package logstream

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/model"
)

const (
	// DefaultBufferLines is how many recent lines a session keeps for late readers.
	DefaultBufferLines = 5000
	// MaxTargets bounds how many sources one session follows.
	MaxTargets = 20
	// maxBatchLines caps one flush; lines beyond it are counted as dropped.
	maxBatchLines = 2000
	// resumeTail bounds the lines replayed after a pause.
	resumeTail = 1000
	flushEvery = 150 * time.Millisecond
)

// Target is one source of pods to follow. Set Pods, Kind and Name, or LabelSelector.
type Target struct {
	Namespace     string   `json:"namespace"`
	Pods          []string `json:"pods,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Name          string   `json:"name,omitempty"`
	LabelSelector string   `json:"labelSelector,omitempty"`
}

// Spec describes what a session follows.
type Spec struct {
	Targets   []Target `json:"targets"`
	Container string   `json:"container,omitempty"`
	// TailLines replayed per container on start; 0 → the streamer default (200).
	TailLines    int `json:"tailLines,omitempty"`
	SinceSeconds int `json:"sinceSeconds,omitempty"`
}

// Line is one container log line.
type Line struct {
	Seq       uint64          `json:"seq"`
	Timestamp model.Timestamp `json:"timestamp"`
	Namespace string          `json:"namespace"`
	Pod       string          `json:"pod"`
	Container string          `json:"container"`
	Severity  model.Severity  `json:"severity"`
	Text      string          `json:"text"`
}

// Source is a resolved target, described for the stream header.
type Source struct {
	Namespace string `json:"namespace"`
	Label     string `json:"label"`
	// Follows is true when new pods matching the source join automatically.
	Follows bool `json:"follows"`
}

// Status is the session state shown in the stream header.
type Status struct {
	// ID is the session's ID (Session.ID), so a desktop with several streams
	// can tell their statuses apart.
	ID      string   `json:"id,omitempty"`
	Active  bool     `json:"active"`
	Paused  bool     `json:"paused"`
	Spec    Spec     `json:"spec"`
	Sources []Source `json:"sources,omitempty"`
	Dropped uint64   `json:"dropped"`
	Notices []string `json:"notices,omitempty"`
}

// Batch is one flush of new lines.
type Batch struct {
	ID      string `json:"id,omitempty"`
	Lines   []Line `json:"lines"`
	Dropped uint64 `json:"dropped"`
}

// Session follows the logs of one Spec. Start replaces any running spec.
type Session struct {
	// ID is copied onto every Status and Batch.
	ID     string
	Client *kube.Client
	// MaxLogRequests is shared across all targets; 0 → kube.DefaultMaxLogRequests.
	MaxLogRequests int
	BufferLines    int
	OnBatch        func(Batch)
	OnStatus       func(Status)
	// OnLog receives every log line as it arrives, before batching
	// (the desktop forwards them to a running investigation).
	OnLog func(model.EvidenceEvent)

	// ctl serialises Start, Stop, Pause and Resume; mu guards the fields below.
	ctl      sync.Mutex
	mu       sync.Mutex
	status   Status
	streams  []resolved
	stop     func()
	pausedAt time.Time
	seq      uint64
	ring     []Line
	ringNext int // oldest line once the ring is full
	ringFull bool
	pending  []Line
	notices  []string
}

type resolved struct {
	source   Source
	streamer kube.LogStreamer
	pods     []model.PodSummary
}

// Start resolves spec and begins following it (replaces any previous follow set
// and clears buffered lines).
func (s *Session) Start(ctx context.Context, spec Spec) error {
	s.ctl.Lock()
	defer s.ctl.Unlock()
	if s.Client == nil || s.Client.Clientset == nil {
		return fmt.Errorf("no cluster client")
	}
	streams, err := s.resolve(ctx, spec)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.stopLocked()
	s.resetBufferLocked()
	s.streams = streams
	s.status = Status{Active: true, Spec: spec, Sources: sources(streams)}
	s.runLocked(ctx, 0, 0)
	st := s.statusLocked()
	s.mu.Unlock()
	s.emitStatus(st)
	return nil
}

// MergeInto adds sources to an active, non-paused session without clearing the
// line buffer or resetting the line sequence.
func (s *Session) MergeInto(ctx context.Context, add Spec) error {
	s.ctl.Lock()
	defer s.ctl.Unlock()
	if s.Client == nil || s.Client.Clientset == nil {
		return fmt.Errorf("no cluster client")
	}
	s.mu.Lock()
	if !s.status.Active || s.status.Paused {
		s.mu.Unlock()
		return fmt.Errorf("cannot merge: stream is not active")
	}
	merged, err := mergeSpecs(s.status.Spec, add)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	streams, err := s.resolve(ctx, merged)
	if err != nil {
		return err
	}
	s.mu.Lock()
	prevDropped := s.status.Dropped
	prevNotices := append([]string(nil), s.notices...)
	s.stopLocked()
	// Keep ring buffer and seq; new lines continue the sequence from sink.
	s.streams = streams
	s.status = Status{
		Active:  true,
		Paused:  false,
		Spec:    merged,
		Sources: sources(streams),
		Dropped: prevDropped,
		Notices: prevNotices,
	}
	s.runLocked(ctx, 0, 0)
	st := s.statusLocked()
	s.mu.Unlock()
	s.emitStatus(st)
	return nil
}

// Stop ends the session and clears its lines.
func (s *Session) Stop() {
	s.ctl.Lock()
	defer s.ctl.Unlock()
	s.mu.Lock()
	s.stopLocked()
	s.resetBufferLocked()
	s.streams = nil
	s.status = Status{}
	st := s.statusLocked()
	s.mu.Unlock()
	s.emitStatus(st)
}

// Pause closes the log follows but keeps the spec and lines.
func (s *Session) Pause() error {
	s.ctl.Lock()
	defer s.ctl.Unlock()
	s.mu.Lock()
	if !s.status.Active {
		s.mu.Unlock()
		return fmt.Errorf("no log stream running")
	}
	if s.status.Paused {
		s.mu.Unlock()
		return nil
	}
	s.stopLocked()
	s.status.Paused = true
	s.pausedAt = time.Now()
	st := s.statusLocked()
	s.mu.Unlock()
	s.emitStatus(st)
	return nil
}

// Resume reopens the follows, replaying what was logged while paused.
func (s *Session) Resume(ctx context.Context) error {
	s.ctl.Lock()
	defer s.ctl.Unlock()
	s.mu.Lock()
	if !s.status.Active {
		s.mu.Unlock()
		return fmt.Errorf("no log stream running")
	}
	if !s.status.Paused {
		s.mu.Unlock()
		return nil
	}
	since := time.Since(s.pausedAt) + time.Second
	s.status.Paused = false
	s.runLocked(ctx, resumeTail, since)
	st := s.statusLocked()
	s.mu.Unlock()
	s.emitStatus(st)
	return nil
}

// Status reports the current session state.
func (s *Session) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

// Lines returns the buffered lines, oldest first.
func (s *Session) Lines() []Line {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ringFull {
		return append([]Line(nil), s.ring...)
	}
	out := make([]Line, 0, len(s.ring))
	out = append(out, s.ring[s.ringNext:]...)
	return append(out, s.ring[:s.ringNext]...)
}

// runLocked starts one streamer per source. tail and since override the spec (resume).
func (s *Session) runLocked(parent context.Context, tail int, since time.Duration) {
	ctx, cancel := context.WithCancel(parent)
	per := s.maxRequests() / len(s.streams)
	if per < 1 {
		per = 1
	}
	spec := s.status.Spec
	var stops []func()
	for _, r := range s.streams {
		ls := r.streamer
		ls.Sink = s.sink
		ls.MaxLogRequests = per
		ls.Container = spec.Container
		ls.Timestamps = true
		ls.Tail = spec.TailLines
		if spec.SinceSeconds > 0 {
			ls.Since = time.Duration(spec.SinceSeconds) * time.Second
		}
		if since > 0 {
			ls.Tail, ls.Since = tail, since
		}
		stops = append(stops, ls.StartWithStop(ctx, r.pods))
	}

	done := make(chan struct{})
	flushed := make(chan struct{})
	go func() {
		defer close(flushed)
		t := time.NewTicker(flushEvery)
		defer t.Stop()
		for {
			select {
			case <-done:
				s.flush()
				return
			case <-t.C:
				s.flush()
			}
		}
	}()
	s.stop = func() {
		cancel()
		for _, stop := range stops {
			stop()
		}
		close(done)
		<-flushed
	}
}

// stopLocked stops streams and the flusher. Streams and the flusher take s.mu,
// so the lock is released while they drain; s.ctl keeps other calls out.
func (s *Session) stopLocked() {
	stop := s.stop
	s.stop = nil
	if stop == nil {
		return
	}
	s.mu.Unlock()
	stop()
	s.mu.Lock()
}

func (s *Session) maxRequests() int {
	if s.MaxLogRequests > 0 {
		return s.MaxLogRequests
	}
	return kube.DefaultMaxLogRequests
}

func (s *Session) sink(e model.EvidenceEvent) {
	if s.OnLog != nil && e.SourceType == model.SourceLog {
		s.OnLog(e)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.SourceType != model.SourceLog {
		if e.Message != "" && len(s.notices) < 20 {
			s.notices = append(s.notices, e.Message)
		}
		return
	}
	if len(s.pending) >= maxBatchLines {
		s.status.Dropped++
		return
	}
	s.seq++
	line := Line{
		Seq:       s.seq,
		Timestamp: e.Timestamp,
		Namespace: e.Namespace,
		Pod:       e.Pod,
		Container: e.Container,
		Severity:  e.Severity,
		Text:      e.Raw,
	}
	s.pending = append(s.pending, line)
	s.pushRingLocked(line)
}

func (s *Session) flush() {
	s.mu.Lock()
	lines := s.pending
	s.pending = nil
	dropped := s.status.Dropped
	notices := len(s.notices) != len(s.status.Notices)
	if notices {
		s.status.Notices = append([]string(nil), s.notices...)
	}
	st := s.statusLocked()
	s.mu.Unlock()
	if len(lines) > 0 && s.OnBatch != nil {
		s.OnBatch(Batch{ID: s.ID, Lines: lines, Dropped: dropped})
	}
	if notices {
		s.emitStatus(st)
	}
}

func (s *Session) pushRingLocked(l Line) {
	capacity := s.BufferLines
	if capacity <= 0 {
		capacity = DefaultBufferLines
	}
	if !s.ringFull {
		s.ring = append(s.ring, l)
		s.ringFull = len(s.ring) >= capacity
		return
	}
	s.ring[s.ringNext] = l
	s.ringNext = (s.ringNext + 1) % capacity
}

func (s *Session) resetBufferLocked() {
	s.ring, s.ringNext, s.ringFull = nil, 0, false
	s.pending, s.notices = nil, nil
}

func (s *Session) statusLocked() Status {
	st := s.status
	st.ID = s.ID
	st.Sources = append([]Source(nil), st.Sources...)
	st.Notices = append([]string(nil), st.Notices...)
	return st
}

func (s *Session) emitStatus(st Status) {
	if s.OnStatus != nil {
		s.OnStatus(st)
	}
}

func sources(rs []resolved) []Source {
	out := make([]Source, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.source)
	}
	return out
}

// resolve turns targets into streamer settings, reading workload selectors.
func (s *Session) resolve(ctx context.Context, spec Spec) ([]resolved, error) {
	if len(spec.Targets) == 0 {
		return nil, fmt.Errorf("select at least one pod or workload")
	}
	if len(spec.Targets) > MaxTargets {
		return nil, fmt.Errorf("too many sources (%d); stream at most %d at once", len(spec.Targets), MaxTargets)
	}
	out := make([]resolved, 0, len(spec.Targets))
	for _, t := range spec.Targets {
		r, err := s.resolveTarget(ctx, t)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Session) resolveTarget(ctx context.Context, t Target) (resolved, error) {
	ns := strings.TrimSpace(t.Namespace)
	if ns == "" {
		return resolved{}, fmt.Errorf("namespace is required")
	}
	base := resolved{streamer: kube.LogStreamer{Client: s.Client, Namespace: ns}}

	kind := strings.TrimSpace(t.Kind)
	name := strings.TrimSpace(t.Name)
	if kind == "Pod" && name != "" {
		t.Pods = append(t.Pods, name)
		kind, name = "", ""
	}

	switch {
	case len(t.Pods) > 0:
		names := uniqueNames(t.Pods)
		for _, p := range names {
			base.pods = append(base.pods, model.PodSummary{Name: p, Namespace: ns})
		}
		base.source = Source{Namespace: ns, Label: podsLabel(names)}
		// Named pods get no initial containers; the pod watch starts them.
		return base, nil
	case kind != "" && name != "":
		sel, err := s.workloadSelector(ctx, ns, kind, name)
		if err != nil {
			return resolved{}, err
		}
		base.streamer.LabelSelector = sel
		base.source = Source{Namespace: ns, Label: kind + "/" + name, Follows: true}
		return base, nil
	case strings.TrimSpace(t.LabelSelector) != "":
		sel, err := labels.Parse(t.LabelSelector)
		if err != nil {
			return resolved{}, fmt.Errorf("label selector %q: %w", t.LabelSelector, err)
		}
		base.streamer.LabelSelector = sel.String()
		base.source = Source{Namespace: ns, Label: sel.String(), Follows: true}
		return base, nil
	}
	return resolved{}, fmt.Errorf("target in %s names no pods, workload or selector", ns)
}

// workloadSelector reads the pod selector of a workload or Service.
func (s *Session) workloadSelector(ctx context.Context, ns, kind, name string) (string, error) {
	cs := s.Client.Clientset
	get := metav1.GetOptions{}
	var sel *metav1.LabelSelector
	switch kind {
	case "Deployment":
		o, err := cs.AppsV1().Deployments(ns).Get(ctx, name, get)
		if err != nil {
			return "", err
		}
		sel = o.Spec.Selector
	case "StatefulSet":
		o, err := cs.AppsV1().StatefulSets(ns).Get(ctx, name, get)
		if err != nil {
			return "", err
		}
		sel = o.Spec.Selector
	case "DaemonSet":
		o, err := cs.AppsV1().DaemonSets(ns).Get(ctx, name, get)
		if err != nil {
			return "", err
		}
		sel = o.Spec.Selector
	case "ReplicaSet":
		o, err := cs.AppsV1().ReplicaSets(ns).Get(ctx, name, get)
		if err != nil {
			return "", err
		}
		sel = o.Spec.Selector
	case "Job":
		o, err := cs.BatchV1().Jobs(ns).Get(ctx, name, get)
		if err != nil {
			return "", err
		}
		sel = o.Spec.Selector
	case "Service":
		o, err := cs.CoreV1().Services(ns).Get(ctx, name, get)
		if err != nil {
			return "", err
		}
		if len(o.Spec.Selector) == 0 {
			return "", fmt.Errorf("service %s has no pod selector", name)
		}
		return labels.SelectorFromSet(o.Spec.Selector).String(), nil
	default:
		return "", fmt.Errorf("cannot stream logs for %s", kind)
	}
	if sel == nil {
		return "", fmt.Errorf("%s/%s has no pod selector", kind, name)
	}
	parsed, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return "", fmt.Errorf("%s/%s selector: %w", kind, name, err)
	}
	if parsed.Empty() {
		// An empty selector would follow every pod in the namespace.
		return "", fmt.Errorf("%s/%s has an empty pod selector", kind, name)
	}
	return parsed.String(), nil
}

func uniqueNames(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, n := range in {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func podsLabel(names []string) string {
	if len(names) == 1 {
		return "Pod/" + names[0]
	}
	return fmt.Sprintf("%d pods", len(names))
}

func targetKey(t Target) string {
	ns := strings.TrimSpace(t.Namespace)
	kind := strings.TrimSpace(t.Kind)
	name := strings.TrimSpace(t.Name)
	ls := strings.TrimSpace(t.LabelSelector)
	if len(t.Pods) > 0 {
		names := uniqueNames(t.Pods)
		return ns + "|pods:" + strings.Join(names, ",")
	}
	if kind != "" && name != "" {
		return ns + "|" + kind + "/" + name
	}
	if ls != "" {
		return ns + "|sel:" + ls
	}
	return ns + "|empty"
}

func mergeSpecs(existing, add Spec) (Spec, error) {
	var out Spec
	seen := map[string]struct{}{}
	for _, t := range existing.Targets {
		k := targetKey(t)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out.Targets = append(out.Targets, t)
	}
	for _, t := range add.Targets {
		k := targetKey(t)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out.Targets = append(out.Targets, t)
	}
	if len(out.Targets) > MaxTargets {
		return Spec{}, fmt.Errorf("too many sources after merge (%d); stream at most %d at once", len(out.Targets), MaxTargets)
	}
	c1, c2 := strings.TrimSpace(existing.Container), strings.TrimSpace(add.Container)
	switch {
	case c1 == "":
		out.Container = c2
	case c2 == "" || c1 == c2:
		out.Container = c1
	default:
		// Second selection used a different filter; keep the session's filter.
		out.Container = c1
	}
	out.TailLines = maxNonZero(existing.TailLines, add.TailLines)
	out.SinceSeconds = maxNonZero(existing.SinceSeconds, add.SinceSeconds)
	return out, nil
}

func maxNonZero(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a > b {
		return a
	}
	return b
}
