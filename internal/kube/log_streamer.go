package kube

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/glnreddy421/klew/internal/model"
)

// ANSI CSI / OSC sequences from colored container logs (klog, zap, kyverno, …).
var reANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// stripANSI removes terminal color/style escapes so the UI and Drain3 see plain text.
func stripANSI(s string) string {
	if s == "" || !strings.ContainsRune(s, '\x1b') {
		return s
	}
	return reANSI.ReplaceAllString(s, "")
}

// DefaultMaxLogRequests caps concurrent pod/container log follows
// so apiserver/kubelet load stays bounded.
const DefaultMaxLogRequests = 50

// pendingMultiplier caps waiting targets relative to MaxLogRequests
// (deduped by ns/pod/container; oldest waiters dropped when full).
const pendingMultiplier = 4

// LogStreamer tails logs from matched pods/containers without polling Lists.
type LogStreamer struct {
	Client    *Client
	Sink      EvidenceSink
	Namespace string
	Query     string
	Tail      int
	Since     time.Duration
	// MaxLogRequests limits concurrent GetLogs follows. 0 → DefaultMaxLogRequests.
	MaxLogRequests int
}

type logTarget struct {
	ns, pod, container string
}

func (t logTarget) key() string {
	return t.ns + "/" + t.pod + "/" + t.container
}

type logStreamController struct {
	ls *LogStreamer

	mu          sync.Mutex
	active      map[string]context.CancelFunc
	pending     []logTarget
	pendingSet  map[string]bool
	sem         chan struct{}
	capWarned   bool
	allowedPods map[string]bool // investigation scope; empty → fall back to Query match

	parentCtx context.Context
	parentWG  *sync.WaitGroup
}

func (c *logStreamController) podAllowed(name string) bool {
	if len(c.allowedPods) > 0 {
		return c.allowedPods[name]
	}
	return matchesQueryName(name, c.ls.Query) || c.ls.Query == ""
}

func (ls *LogStreamer) maxRequests() int {
	if ls.MaxLogRequests > 0 {
		return ls.MaxLogRequests
	}
	return DefaultMaxLogRequests
}

// Start streams logs for initial pods, then watches for pod add/update/delete.
// Prefer StartWithStop for sessions that need a clean shutdown boundary.
func (ls *LogStreamer) Start(ctx context.Context, pods []model.PodSummary, wg *sync.WaitGroup) {
	ls.start(ctx, pods, wg)
}

// StartWithStop begins follows and returns stop, which cancels streams and waits for workers.
func (ls *LogStreamer) StartWithStop(parent context.Context, pods []model.PodSummary) (stop func()) {
	if parent == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	ls.start(ctx, pods, &wg)
	return func() {
		cancel()
		wg.Wait()
	}
}

func (ls *LogStreamer) start(ctx context.Context, pods []model.PodSummary, wg *sync.WaitGroup) {
	if ls.Sink == nil || ls.Client == nil {
		return
	}

	ctrl := &logStreamController{
		ls:         ls,
		active:     make(map[string]context.CancelFunc),
		pendingSet: make(map[string]bool),
		sem:        make(chan struct{}, ls.maxRequests()),
		parentCtx:  ctx,
		parentWG:   wg,
	}
	for _, p := range pods {
		if p.Name == "" {
			continue
		}
		if ctrl.allowedPods == nil {
			ctrl.allowedPods = make(map[string]bool)
		}
		ctrl.allowedPods[p.Name] = true
	}

	for _, p := range pods {
		if !ctrl.podAllowed(p.Name) {
			continue
		}
		ns := p.Namespace
		if ns == "" {
			ns = ls.Namespace
		}
		for _, c := range p.Containers {
			if c.Name == "" {
				continue
			}
			ctrl.ensure(logTarget{ns: ns, pod: p.Name, container: c.Name})
		}
	}

	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		ctrl.watchPods(ctx)
	}()
}

func (c *logStreamController) ensure(t logTarget) {
	if t.ns == "" || t.pod == "" || t.container == "" {
		return
	}
	key := t.key()

	c.mu.Lock()
	if _, ok := c.active[key]; ok {
		c.mu.Unlock()
		return
	}
	if c.pendingSet[key] {
		c.mu.Unlock()
		return
	}

	select {
	case c.sem <- struct{}{}:
		streamCtx, cancel := context.WithCancel(c.parentCtx)
		c.active[key] = cancel
		c.mu.Unlock()
		c.startStream(streamCtx, cancel, t)
	default:
		c.enqueuePending(t)
		warn := !c.capWarned
		if warn {
			c.capWarned = true
		}
		c.mu.Unlock()
		if warn && c.ls.Sink != nil {
			c.ls.Sink(systemEvent(
				"log_stream_capacity",
				fmt.Sprintf("Reached concurrent log follow limit (%d); additional containers wait for a free slot", c.ls.maxRequests()),
				model.SeverityWarning,
			))
		}
	}
}

// enqueuePending appends t; drops oldest waiters when over cap. Caller holds c.mu.
func (c *logStreamController) enqueuePending(t logTarget) {
	key := t.key()
	if c.pendingSet[key] {
		return
	}
	c.pending = append(c.pending, t)
	c.pendingSet[key] = true
	max := c.ls.maxRequests() * pendingMultiplier
	if max < pendingMultiplier {
		max = pendingMultiplier
	}
	for len(c.pending) > max {
		drop := c.pending[0]
		c.pending = c.pending[1:]
		delete(c.pendingSet, drop.key())
	}
}

func (c *logStreamController) startStream(streamCtx context.Context, cancel context.CancelFunc, t logTarget) {
	wg := c.parentWG
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		defer cancel()
		defer c.onStreamDone(t)

		c.ls.streamContainer(streamCtx, t.ns, t.pod, t.container)
	}()
}

func (c *logStreamController) onStreamDone(t logTarget) {
	key := t.key()

	c.mu.Lock()
	delete(c.active, key)
	select {
	case <-c.sem:
	default:
	}

	for len(c.pending) > 0 {
		cand := c.pending[0]
		c.pending = c.pending[1:]
		delete(c.pendingSet, cand.key())
		if _, exists := c.active[cand.key()]; exists {
			continue
		}
		select {
		case c.sem <- struct{}{}:
			streamCtx, cancel := context.WithCancel(c.parentCtx)
			c.active[cand.key()] = cancel
			c.mu.Unlock()
			c.startStream(streamCtx, cancel, cand)
			return
		default:
			c.pending = append([]logTarget{cand}, c.pending...)
			c.pendingSet[cand.key()] = true
			c.mu.Unlock()
			return
		}
	}
	c.mu.Unlock()
}

func (c *logStreamController) stopPod(ns, name string) {
	prefix := ns + "/" + name + "/"
	c.mu.Lock()
	var cancels []context.CancelFunc
	for key, cancel := range c.active {
		if strings.HasPrefix(key, prefix) {
			cancels = append(cancels, cancel)
		}
	}
	// Drop pending targets for this pod.
	filtered := c.pending[:0]
	for _, t := range c.pending {
		if t.ns == ns && t.pod == name {
			delete(c.pendingSet, t.key())
			continue
		}
		filtered = append(filtered, t)
	}
	c.pending = filtered
	c.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

// syncPod starts follows for current app containers and cancels streams/pending
// for containers that were removed from the pod spec.
func (c *logStreamController) syncPod(pod *corev1.Pod) {
	if pod == nil {
		return
	}
	if !c.podAllowed(pod.Name) {
		return
	}
	if pod.DeletionTimestamp != nil ||
		pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed {
		c.stopPod(pod.Namespace, pod.Name)
		return
	}
	want := map[string]bool{}
	for _, name := range appContainerNames(pod) {
		want[name] = true
		c.ensure(logTarget{ns: pod.Namespace, pod: pod.Name, container: name})
	}
	prefix := pod.Namespace + "/" + pod.Name + "/"
	c.mu.Lock()
	var cancels []context.CancelFunc
	for key, cancel := range c.active {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		cont := strings.TrimPrefix(key, prefix)
		if !want[cont] {
			cancels = append(cancels, cancel)
		}
	}
	filtered := c.pending[:0]
	for _, t := range c.pending {
		if t.ns == pod.Namespace && t.pod == pod.Name && !want[t.container] {
			delete(c.pendingSet, t.key())
			continue
		}
		filtered = append(filtered, t)
	}
	c.pending = filtered
	c.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// reconcileFromList aligns active/pending streams with a fresh pod List
// (used after watch reconnect so deleted pods do not retain follow slots).
func (c *logStreamController) reconcileFromList(pods []corev1.Pod) {
	live := make(map[string]struct{}, len(pods))
	for i := range pods {
		p := &pods[i]
		if !c.podAllowed(p.Name) {
			continue
		}
		live[p.Namespace+"/"+p.Name] = struct{}{}
		c.syncPod(p)
	}

	c.mu.Lock()
	type podKey struct{ ns, name string }
	var stale []podKey
	seen := map[string]bool{}
	mark := func(ns, name string) {
		pk := ns + "/" + name
		if _, ok := live[pk]; ok || seen[pk] {
			return
		}
		seen[pk] = true
		stale = append(stale, podKey{ns: ns, name: name})
	}
	for key := range c.active {
		parts := strings.SplitN(key, "/", 3)
		if len(parts) >= 2 {
			mark(parts[0], parts[1])
		}
	}
	for _, t := range c.pending {
		mark(t.ns, t.pod)
	}
	c.mu.Unlock()

	for _, p := range stale {
		c.stopPod(p.ns, p.name)
	}
}

func (c *logStreamController) watchPods(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		established, err := c.watchPodsOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if established {
			backoff = time.Second
		}
		if err != nil && c.ls.Sink != nil {
			c.ls.Sink(systemEvent("log_pods_watch_failed", err.Error(), model.SeverityWarning))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if !established {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (c *logStreamController) watchPodsOnce(ctx context.Context) (established bool, err error) {
	list, err := c.ls.Client.Clientset.CoreV1().Pods(c.ls.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	c.reconcileFromList(list.Items)

	wi, err := c.ls.Client.Clientset.CoreV1().Pods(c.ls.Namespace).Watch(ctx, metav1.ListOptions{
		ResourceVersion: list.ResourceVersion,
	})
	if err != nil {
		return false, err
	}
	defer wi.Stop()
	established = true

	for {
		select {
		case <-ctx.Done():
			return true, nil
		case ev, ok := <-wi.ResultChan():
			if !ok {
				return true, fmt.Errorf("pod watch closed")
			}
			if ev.Type == watch.Error {
				return true, fmt.Errorf("pod watch error")
			}
			pod, ok := ev.Object.(*corev1.Pod)
			if !ok || pod == nil {
				continue
			}
			switch ev.Type {
			case watch.Added, watch.Modified:
				c.syncPod(pod)
			case watch.Deleted:
				c.stopPod(pod.Namespace, pod.Name)
			}
		}
	}
}

// appContainerNames returns regular app container names only (no init/ephemeral)
// to keep follow load bounded.
func appContainerNames(pod *corev1.Pod) []string {
	if pod == nil {
		return nil
	}
	names := make([]string, 0, len(pod.Spec.Containers))
	seen := map[string]bool{}
	for _, c := range pod.Spec.Containers {
		if c.Name == "" || seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		names = append(names, c.Name)
	}
	// If Spec is empty but status exists (unusual), fall back to status names.
	if len(names) == 0 {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == "" || seen[cs.Name] {
				continue
			}
			seen[cs.Name] = true
			names = append(names, cs.Name)
		}
	}
	return names
}

func (ls *LogStreamer) streamContainer(ctx context.Context, ns, pod, container string) {
	attempt := 0
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := ls.followOnce(ctx, ns, pod, container, attempt == 0)
		if ctx.Err() != nil {
			return
		}
		attempt++
		if err != nil && err != io.EOF && ls.Sink != nil && attempt == 1 {
			// One soft notice; reconnects stay quiet to avoid noise.
			ls.Sink(systemEvent(
				"log_stream_retry",
				fmt.Sprintf("%s/%s: %v", pod, container, err),
				model.SeverityInfo,
			))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 15*time.Second {
			backoff *= 2
			if backoff > 15*time.Second {
				backoff = 15 * time.Second
			}
		}
	}
}

func (ls *LogStreamer) followOnce(ctx context.Context, ns, pod, container string, initial bool) error {
	tail := int64(ls.Tail)
	if tail <= 0 {
		tail = 200
	}
	if !initial {
		// Reconnects should not replay the whole tail window.
		tail = 0
	}
	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    true,
		TailLines: &tail,
	}
	if initial && ls.Since > 0 {
		secs := int64(ls.Since.Seconds())
		if secs > 0 {
			opts.SinceSeconds = &secs
		}
	}
	req := ls.Client.Clientset.CoreV1().Pods(ns).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	sc := bufio.NewScanner(stream)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := stripANSI(sc.Text())
		sev := classifyLogLine(line)
		ls.Sink(model.EvidenceEvent{
			Timestamp:  model.TimestampFrom(time.Now().UTC()),
			SourceType: model.SourceLog,
			SourceKind: "Pod",
			SourceName: pod,
			Namespace:  ns,
			Pod:        pod,
			Container:  container,
			Severity:   sev,
			Reason:     classifyLogReason(line),
			Message:    fmt.Sprintf("%s/%s: %s", pod, container, line),
			Raw:        line,
			Confidence: 0.75,
		})
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return io.EOF
}

func classifyLogLine(line string) model.Severity {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "oom"), strings.Contains(l, "fatal"), strings.Contains(l, "panic"):
		return model.SeverityCritical
	case strings.Contains(l, "error"), strings.Contains(l, "failed"), strings.Contains(l, "refused"):
		return model.SeverityHigh
	case strings.Contains(l, "warn"), strings.Contains(l, "timeout"):
		return model.SeverityWarning
	default:
		return model.SeverityInfo
	}
}

func classifyLogReason(line string) string {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "allocating memory"), strings.Contains(l, "dd if="),
		strings.Contains(l, "/tmp/leak"), strings.Contains(l, "/dev/shm"):
		return "Memory allocation"
	case strings.Contains(l, "out of memory allocating"), strings.Contains(l, "cannot allocate"):
		return "Memory allocation failures"
	case strings.Contains(l, "oom"), strings.Contains(l, "out of memory"):
		return "OOMKilled"
	case strings.Contains(l, "memory leak"), strings.Contains(l, "heap growth"):
		return "Memory leak"
	case strings.Contains(l, "redis"), strings.Contains(l, "timeout"):
		return "Redis timeout"
	default:
		return ""
	}
}
