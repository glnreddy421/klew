package kube

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/glnreddy421/klew/internal/model"
)

// EvidenceSink receives live evidence events.
type EvidenceSink func(model.EvidenceEvent)

// LiveWatcher watches pods and events in a namespace.
type LiveWatcher struct {
	Client    *Client
	Sink      EvidenceSink
	Namespace string
	Query     string

	scopeMu   sync.RWMutex
	scopePods investigationPods

	dedupeMu sync.Mutex
	dedupe   map[string]struct{}
}

// SetScopePodNames updates the investigation pod allowlist used for live events.
func (w *LiveWatcher) SetScopePodNames(names []string) {
	if w == nil {
		return
	}
	w.scopeMu.Lock()
	w.scopePods = newInvestigationPods(names)
	w.scopeMu.Unlock()
}

func (w *LiveWatcher) scopeSnapshot() investigationPods {
	w.scopeMu.RLock()
	defer w.scopeMu.RUnlock()
	if len(w.scopePods) == 0 {
		return nil
	}
	out := make(investigationPods, len(w.scopePods))
	for k, v := range w.scopePods {
		out[k] = v
	}
	return out
}

// Start launches watch goroutines.
func (w *LiveWatcher) Start(ctx context.Context, wg *sync.WaitGroup, scopePodNames []string) []model.ActiveWatch {
	w.SetScopePodNames(scopePodNames)
	now := time.Now().UTC()
	watches := []model.ActiveWatch{
		{Name: "pods", Resource: "pods", Namespace: w.Namespace, StartedAt: model.TimestampFrom(now)},
		{Name: "events", Resource: "events", Namespace: w.Namespace, StartedAt: model.TimestampFrom(now)},
		{Name: "events_v1", Resource: "events.events.k8s.io", Namespace: w.Namespace, StartedAt: model.TimestampFrom(now)},
		{Name: "deployments", Resource: "deployments", Namespace: w.Namespace, StartedAt: model.TimestampFrom(now)},
	}
	if wg != nil {
		wg.Add(4)
		go func() { defer wg.Done(); w.watchEvents(ctx) }()
		go func() { defer wg.Done(); w.watchEventsV1(ctx) }()
		go func() { defer wg.Done(); w.watchPods(ctx) }()
		go func() { defer wg.Done(); w.watchDeployments(ctx) }()
	}
	return watches
}

func (w *LiveWatcher) watchEvents(ctx context.Context) {
	if w.Sink == nil {
		return
	}
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		established, err := w.watchEventsOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if established {
			backoff = time.Second
		}
		if err != nil {
			w.Sink(systemEvent("events_watch_failed", err.Error(), model.SeverityWarning))
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

func (w *LiveWatcher) watchEventsV1(ctx context.Context) {
	if w.Sink == nil {
		return
	}
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		established, err := w.watchEventsV1Once(ctx)
		if ctx.Err() != nil {
			return
		}
		if established {
			backoff = time.Second
		}
		if err != nil {
			w.Sink(systemEvent("events_v1_watch_failed", err.Error(), model.SeverityWarning))
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

func (w *LiveWatcher) watchEventsOnce(ctx context.Context) (established bool, err error) {
	list, err := w.Client.Clientset.CoreV1().Events(w.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	const maxSeed = 300
	items := list.Items
	if len(items) > maxSeed {
		items = items[len(items)-maxSeed:]
	}
	scope := w.scopeSnapshot()
	for i := range items {
		w.emitCoreEvent(&items[i], scope)
	}

	wi, err := w.Client.Clientset.CoreV1().Events(w.Namespace).Watch(ctx, metav1.ListOptions{
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
				return true, fmt.Errorf("events watch closed")
			}
			if ev.Type == watch.Error {
				return true, fmt.Errorf("events watch error")
			}
			e, ok := ev.Object.(*corev1.Event)
			if !ok || e == nil {
				continue
			}
			w.emitCoreEvent(e, w.scopeSnapshot())
		}
	}
}

func (w *LiveWatcher) watchEventsV1Once(ctx context.Context) (established bool, err error) {
	list, err := w.Client.Clientset.EventsV1().Events(w.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	wi, err := w.Client.Clientset.EventsV1().Events(w.Namespace).Watch(ctx, metav1.ListOptions{
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
				return true, fmt.Errorf("events v1 watch closed")
			}
			if ev.Type == watch.Error {
				return true, fmt.Errorf("events v1 watch error")
			}
			e, ok := ev.Object.(*eventsv1.Event)
			if !ok || e == nil {
				continue
			}
			w.emitEventsV1(e, w.scopeSnapshot())
		}
	}
}

func (w *LiveWatcher) shouldEmit(kind, name, reason, msg string) bool {
	key := kind + "|" + name + "|" + reason + "|" + msg
	w.dedupeMu.Lock()
	defer w.dedupeMu.Unlock()
	if w.dedupe == nil {
		w.dedupe = make(map[string]struct{}, 256)
	}
	if _, ok := w.dedupe[key]; ok {
		return false
	}
	w.dedupe[key] = struct{}{}
	if len(w.dedupe) > 500 {
		w.dedupe = make(map[string]struct{}, 256)
	}
	return true
}

func (w *LiveWatcher) emitCoreEvent(e *corev1.Event, scope investigationPods) {
	if e == nil || w.Sink == nil {
		return
	}
	kind := e.InvolvedObject.Kind
	name := e.InvolvedObject.Name
	if !EventInInvestigationScope(kind, name, w.Query, scope) {
		return
	}
	if !w.shouldEmit(kind, name, e.Reason, e.Message) {
		return
	}
	ts := e.LastTimestamp.Time
	if ts.IsZero() {
		ts = e.EventTime.Time
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	w.Sink(model.EvidenceEvent{
		Timestamp:  model.TimestampFrom(ts),
		SourceType: model.SourceK8sEvent,
		SourceKind: kind,
		SourceName: name,
		Namespace:  e.InvolvedObject.Namespace,
		Severity:   eventSeverity(e.Reason, e.Message),
		Reason:     e.Reason,
		Message:    e.Message,
		Raw:        e.Message,
		Count:      int(e.Count),
		Confidence: 0.9,
		RelatedObjectRefs: []model.ObjectRef{{
			Kind: kind, Name: name, Namespace: e.InvolvedObject.Namespace,
		}},
	})
}

func (w *LiveWatcher) emitEventsV1(e *eventsv1.Event, scope investigationPods) {
	if e == nil || w.Sink == nil {
		return
	}
	kind := e.Regarding.Kind
	name := e.Regarding.Name
	msg := e.Note
	if !EventInInvestigationScope(kind, name, w.Query, scope) {
		return
	}
	if !w.shouldEmit(kind, name, e.Reason, msg) {
		return
	}
	rec := eventV1ToRecord(*e)
	w.Sink(model.EvidenceEvent{
		Timestamp:  rec.Timestamp,
		SourceType: model.SourceK8sEvent,
		SourceKind: kind,
		SourceName: name,
		Namespace:  rec.InvolvedObject.Namespace,
		Severity:   eventRecordSeverity(rec.Reason),
		Reason:     rec.Reason,
		Message:    rec.Message,
		Raw:        rec.Message,
		Count:      int(rec.Count),
		Confidence: 0.9,
		RelatedObjectRefs: []model.ObjectRef{{
			Kind: kind, Name: name, Namespace: rec.InvolvedObject.Namespace,
		}},
	})
}

func eventRecordSeverity(reason string) model.Severity {
	switch reason {
	case "OOMKilling", "Failed", "FailedScheduling", "FailedMount":
		return model.SeverityCritical
	case "BackOff", "Unhealthy":
		return model.SeverityHigh
	default:
		return model.SeverityWarning
	}
}

func (w *LiveWatcher) watchPods(ctx context.Context) {
	if w.Sink == nil {
		return
	}
	wi, err := w.Client.Clientset.CoreV1().Pods(w.Namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		w.Sink(systemEvent("pods_watch_failed", err.Error(), model.SeverityWarning))
		return
	}
	defer wi.Stop()
	for ev := range wi.ResultChan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		pod, ok := ev.Object.(*corev1.Pod)
		if !ok {
			continue
		}
		if !matchesQueryName(pod.Name, w.Query) && w.Query != "" {
			continue
		}
		w.Sink(model.EvidenceEvent{
			Timestamp:  model.TimestampFrom(time.Now().UTC()),
			SourceType: model.SourceObjectChange,
			SourceKind: "Pod",
			SourceName: pod.Name,
			Namespace:  pod.Namespace,
			Pod:        pod.Name,
			Node:       pod.Spec.NodeName,
			Severity:   model.SeverityInfo,
			Reason:     string(ev.Type),
			Message:    fmt.Sprintf("Pod %s phase=%s", pod.Name, pod.Status.Phase),
			Confidence: 0.85,
		})
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				w.Sink(model.EvidenceEvent{
					Timestamp:  model.TimestampFrom(time.Now().UTC()),
					SourceType: model.SourceObjectChange,
					SourceKind: "Container",
					SourceName: cs.Name,
					Namespace:  pod.Namespace,
					Pod:        pod.Name,
					Container:  cs.Name,
					Severity:   reasonSeverity(cs.State.Waiting.Reason),
					Reason:     cs.State.Waiting.Reason,
					Message:    fmt.Sprintf("Container waiting: %s", cs.State.Waiting.Message),
					Confidence: 0.88,
				})
			}
		}
	}
}

func (w *LiveWatcher) watchDeployments(ctx context.Context) {
	if w.Sink == nil {
		return
	}
	wi, err := w.Client.Clientset.AppsV1().Deployments(w.Namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		w.Sink(systemEvent("deployments_watch_failed", err.Error(), model.SeverityWarning))
		return
	}
	defer wi.Stop()
	for ev := range wi.ResultChan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if ev.Type == watch.Error {
			continue
		}
		dep, ok := ev.Object.(*appsv1.Deployment)
		if !ok {
			continue
		}
		if !matchesQueryName(dep.Name, w.Query) && w.Query != "" {
			continue
		}
		if ev.Type == watch.Deleted {
			continue
		}
		w.emitDeploymentEvidence(dep, string(ev.Type))
	}
}

func (w *LiveWatcher) emitDeploymentEvidence(dep *appsv1.Deployment, changeType string) {
	if w.Sink == nil || dep == nil {
		return
	}
	image := ""
	if len(dep.Spec.Template.Spec.Containers) > 0 {
		image = dep.Spec.Template.Spec.Containers[0].Image
	}
	scenario := dep.Labels["klew-lab/scenario"]
	if scenario != "" {
		w.Sink(model.EvidenceEvent{
			Timestamp:  model.TimestampFrom(time.Now().UTC()),
			SourceType: model.SourceObjectChange,
			SourceKind: "Deployment",
			SourceName: dep.Name,
			Namespace:  dep.Namespace,
			Severity:   model.SeverityInfo,
			Reason:     "ScenarioActive",
			Message:    fmt.Sprintf("Scenario %s active on deployment/%s (image=%s)", scenario, dep.Name, image),
			Confidence: 0.92,
		})
	}
	if denial, ok := dep.Annotations["klew-lab/admission-denial"]; ok && strings.TrimSpace(denial) != "" {
		w.Sink(model.EvidenceEvent{
			Timestamp:  model.TimestampFrom(time.Now().UTC()),
			SourceType: model.SourceObjectChange,
			SourceKind: "Deployment",
			SourceName: dep.Name,
			Namespace:  dep.Namespace,
			Severity:   eventSeverity("AdmissionDenied", denial),
			Reason:     "AdmissionDenied",
			Message:    strings.TrimSpace(denial),
			Raw:        denial,
			Confidence: 0.95,
			RelatedObjectRefs: []model.ObjectRef{{
				Kind: "Deployment", Name: dep.Name, Namespace: dep.Namespace,
			}},
		})
	}
	if changeType != "" && image != "" {
		w.Sink(model.EvidenceEvent{
			Timestamp:  model.TimestampFrom(time.Now().UTC()),
			SourceType: model.SourceObjectChange,
			SourceKind: "Deployment",
			SourceName: dep.Name,
			Namespace:  dep.Namespace,
			Severity:   model.SeverityInfo,
			Reason:     changeType,
			Message:    fmt.Sprintf("Deployment %s updated (image=%s, replicas=%d/%d)", dep.Name, image, dep.Status.ReadyReplicas, dep.Status.Replicas),
			Confidence: 0.85,
		})
	}
}

func systemEvent(reason, msg string, sev model.Severity) model.EvidenceEvent {
	return model.EvidenceEvent{
		Timestamp: model.TimestampFrom(time.Now().UTC()), SourceType: model.SourceSystem,
		Severity: sev, Reason: reason, Message: msg, Confidence: 1,
	}
}

func reasonSeverity(reason string) model.Severity {
	switch reason {
	case "OOMKilling", "Failed", "FailedScheduling", "FailedMount", "ErrImagePull", "ImagePullBackOff":
		return model.SeverityCritical
	case "BackOff", "CrashLoopBackOff", "Unhealthy", "AdmissionDenied", "ValidatingAdmission", "PolicyViolation":
		return model.SeverityHigh
	default:
		return model.SeverityWarning
	}
}

func eventSeverity(reason, message string) model.Severity {
	if sev := reasonSeverity(reason); sev != model.SeverityWarning {
		return sev
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "admission webhook") && strings.Contains(lower, "denied"):
		return model.SeverityHigh
	case strings.Contains(lower, "policy") && (strings.Contains(lower, "failed") || strings.Contains(lower, "violation") || strings.Contains(lower, "denied")):
		return model.SeverityHigh
	case strings.Contains(lower, ":latest"):
		return model.SeverityHigh
	default:
		return reasonSeverity(reason)
	}
}

func matchesQueryName(name, query string) bool {
	if query == "" {
		return true
	}
	_, n, free := ParseQuery(query)
	needle := free
	if n != "" {
		needle = n
	}
	if needle == "" {
		return true
	}
	return containsFold(name, needle)
}

func containsFold(hay, needle string) bool {
	return len(needle) > 0 && (hay == needle || len(hay) >= len(needle) && indexFold(hay, needle))
}

func indexFold(hay, needle string) bool {
	h := []rune(hay)
	n := []rune(needle)
	for i := 0; i+len(n) <= len(h); i++ {
		match := true
		for j := 0; j < len(n); j++ {
			a, b := h[i+j], n[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
