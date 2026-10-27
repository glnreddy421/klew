package kube

import (
	"context"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
)

// RecordedEventsLookback is how far back recorded history is merged into an
// investigation. Older events stay in history but would skew the verdict.
const RecordedEventsLookback = 6 * time.Hour

const recordedEventsQueryLimit = 20000

// EventHistory reads events Klew recorded locally while it was running.
type EventHistory interface {
	RecordedEvents(ctx context.Context, contextName, serverURL string, q model.EventHistoryQuery) ([]model.EventRecord, model.EventHistoryCoverage, error)
}

// collectEvents gathers namespace events for investigation and Event Patterns.
// scopePodNames lists every pod in the investigation snapshot — Pod events for
// those pods are always kept (independent of the search query).
// Recorded events (already expired from the API) are merged in; coverage is
// returned when any recorded event was added.
func (col *Collector) collectEvents(ctx context.Context, namespace, query string, deployNames, scopePodNames []string, listAllowed bool) ([]model.EventRecord, *model.EventHistoryCoverage) {
	scope := newInvestigationPods(scopePodNames)
	var out []model.EventRecord

	// No event list permission → no events, recorded ones included: history
	// must not show more than the current credentials can see.
	if !listAllowed {
		return nil, nil
	}

	if recs, err := col.listCoreEvents(ctx, namespace, query, deployNames, scope); err == nil {
		out = append(out, recs...)
	}
	if recs, err := col.listEventsV1(ctx, namespace, query, deployNames, scope); err == nil {
		out = append(out, recs...)
	}

	return col.mergeRecordedEvents(ctx, dedupeEventRecords(out), namespace, query, deployNames, scope)
}

func (col *Collector) mergeRecordedEvents(ctx context.Context, live []model.EventRecord, namespace, query string, deployNames []string, scope investigationPods) ([]model.EventRecord, *model.EventHistoryCoverage) {
	out := live
	var cov *model.EventHistoryCoverage
	if c := col.Client; c != nil && c.History != nil && c.Config != nil {
		since := time.Now().UTC().Add(-RecordedEventsLookback)
		// The scope filter below runs in Go, so fetch generously: a busy
		// namespace must not push the investigated workload's events out.
		recs, coverage, err := c.History.RecordedEvents(ctx, c.Context, c.Config.Host, model.EventHistoryQuery{
			Namespace: namespace, Since: since, Limit: recordedEventsQueryLimit,
		})
		if err == nil {
			var allowed []model.EventRecord
			for _, r := range recs {
				obj := r.InvolvedObject
				if EventInInvestigationScope(obj.Kind, obj.Name, query, scope) ||
					(len(deployNames) > 0 && eventNameMatchesDeploy(obj.Name, deployNames)) {
					allowed = append(allowed, r)
				}
			}
			var added int
			out, added = MergeRecordedEvents(out, allowed)
			if coverage.Since != "" {
				coverage.RecordedEvents = added
				cov = &coverage
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, cov
}

// MergeRecordedEvents appends recorded events to live and collapses repeats
// (same UID, or same object/reason/message as before history existed). Live
// rows win; a collapsed row keeps the earliest FirstSeen and the higher count.
// Returns the merged list and how many rows came only from history.
func MergeRecordedEvents(live, recorded []model.EventRecord) ([]model.EventRecord, int) {
	var withUID []model.EventRecord
	for _, r := range recorded {
		if r.UID != "" {
			withUID = append(withUID, r)
		}
	}
	out := dedupeEventRecords(append(append([]model.EventRecord(nil), live...), withUID...))
	added := 0
	for _, e := range out {
		if e.Recorded {
			added++
		}
	}
	return out, added
}

func (col *Collector) listCoreEvents(ctx context.Context, namespace, query string, deployNames []string, scope investigationPods) ([]model.EventRecord, error) {
	evs, err := col.Client.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var out []model.EventRecord
	for _, e := range evs.Items {
		if !eventAllowed(&e, query, deployNames, scope) {
			continue
		}
		out = append(out, coreEventToRecord(e))
	}
	return out, nil
}

func (col *Collector) listEventsV1(ctx context.Context, namespace, query string, deployNames []string, scope investigationPods) ([]model.EventRecord, error) {
	evs, err := col.Client.Clientset.EventsV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var out []model.EventRecord
	for _, e := range evs.Items {
		if !eventV1Allowed(&e, query, deployNames, scope) {
			continue
		}
		out = append(out, eventV1ToRecord(e))
	}
	return out, nil
}

// ListEvents returns every Event in namespace ("" = all namespaces) from
// core/v1 and events.k8s.io/v1, deduplicated. The core/v1 error is returned
// so callers can tell a permission denial from an empty list.
func ListEvents(ctx context.Context, c *Client, namespace string) ([]model.EventRecord, error) {
	core, err := c.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]model.EventRecord, 0, len(core.Items))
	for _, e := range core.Items {
		out = append(out, coreEventToRecord(e))
	}
	if v1, err := c.Clientset.EventsV1().Events(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for _, e := range v1.Items {
			out = append(out, eventV1ToRecord(e))
		}
	}
	return dedupeEventRecords(out), nil
}

type investigationPods map[string]struct{}

func newInvestigationPods(names []string) investigationPods {
	set := make(investigationPods, len(names))
	for _, n := range names {
		if n != "" {
			set[n] = struct{}{}
		}
	}
	return set
}

func eventAllowed(e *corev1.Event, query string, deployNames []string, scope investigationPods) bool {
	kind := e.InvolvedObject.Kind
	name := e.InvolvedObject.Name
	if EventInInvestigationScope(kind, name, query, scope) {
		return true
	}
	return len(deployNames) > 0 && eventNameMatchesDeploy(name, deployNames)
}

func eventV1Allowed(e *eventsv1.Event, query string, deployNames []string, scope investigationPods) bool {
	kind := e.Regarding.Kind
	name := e.Regarding.Name
	if EventInInvestigationScope(kind, name, query, scope) {
		return true
	}
	return len(deployNames) > 0 && eventNameMatchesDeploy(name, deployNames)
}

func coreEventToRecord(e corev1.Event) model.EventRecord {
	ts := e.LastTimestamp.Time
	if ts.IsZero() {
		ts = e.EventTime.Time
	}
	if ts.IsZero() {
		ts = e.FirstTimestamp.Time
	}
	first := e.FirstTimestamp.Time
	if first.IsZero() {
		first = e.EventTime.Time
	}
	return model.EventRecord{
		Timestamp: model.TimestampFrom(ts),
		Type:      e.Type,
		Reason:    e.Reason,
		Message:   e.Message,
		Count:     e.Count,
		Source:    e.Source.Component,
		InvolvedObject: model.ObjectRef{
			Kind:      e.InvolvedObject.Kind,
			Name:      e.InvolvedObject.Name,
			Namespace: e.InvolvedObject.Namespace,
		},
		UID:       string(e.UID),
		FirstSeen: model.TimestampFrom(first),
		LastSeen:  model.TimestampFrom(ts),
	}
}

func eventV1ToRecord(e eventsv1.Event) model.EventRecord {
	ts := e.EventTime.Time
	if ts.IsZero() && !e.DeprecatedLastTimestamp.Time.IsZero() {
		ts = e.DeprecatedLastTimestamp.Time
	}
	if ts.IsZero() && !e.DeprecatedFirstTimestamp.Time.IsZero() {
		ts = e.DeprecatedFirstTimestamp.Time
	}
	if e.Series != nil && !e.Series.LastObservedTime.Time.IsZero() {
		ts = e.Series.LastObservedTime.Time
	}
	first := e.DeprecatedFirstTimestamp.Time
	if first.IsZero() {
		first = e.EventTime.Time
	}
	count := int32(1)
	if e.DeprecatedCount > 0 {
		count = e.DeprecatedCount
	} else if e.Series != nil && e.Series.Count > 0 {
		count = e.Series.Count
	}
	component := e.DeprecatedSource.Component
	if component == "" {
		component = e.ReportingController
	}
	return model.EventRecord{
		Timestamp: model.TimestampFrom(ts),
		Type:      e.Type,
		Reason:    e.Reason,
		Message:   e.Note,
		Count:     count,
		Source:    component,
		InvolvedObject: model.ObjectRef{
			Kind:      e.Regarding.Kind,
			Name:      e.Regarding.Name,
			Namespace: e.Regarding.Namespace,
		},
		UID:       string(e.UID),
		FirstSeen: model.TimestampFrom(first),
		LastSeen:  model.TimestampFrom(ts),
	}
}

// dedupeEventRecords collapses repeats: the same Event object (UID — core/v1
// and events.k8s.io/v1 return the same objects) or the same
// kind|name|reason|message (separate Event objects for a recurring problem).
// The first row wins; it keeps the earliest FirstSeen and the higher count,
// and stays marked Recorded only if every collapsed row was recorded.
func dedupeEventRecords(in []model.EventRecord) []model.EventRecord {
	byKey := map[string]int{}
	out := make([]model.EventRecord, 0, len(in))
	for _, e := range in {
		content := "c|" + e.InvolvedObject.Kind + "|" + e.InvolvedObject.Name + "|" + e.Reason + "|" + e.Message
		idx, ok := byKey[content]
		if !ok && e.UID != "" {
			idx, ok = byKey["u|"+e.UID]
		}
		if ok {
			kept := &out[idx]
			if e.FirstSeen != "" && (kept.FirstSeen == "" || e.FirstSeen.Before(kept.FirstSeen)) {
				kept.FirstSeen = e.FirstSeen
			}
			if e.Count > kept.Count {
				kept.Count = e.Count
			}
			kept.Recorded = kept.Recorded && e.Recorded
			byKey[content] = idx
			if e.UID != "" {
				byKey["u|"+e.UID] = idx
			}
			continue
		}
		idx = len(out)
		out = append(out, e)
		byKey[content] = idx
		if e.UID != "" {
			byKey["u|"+e.UID] = idx
		}
	}
	return out
}

func eventNameMatchesDeploy(name string, deployNames []string) bool {
	for _, d := range deployNames {
		if contains(name, d) {
			return true
		}
	}
	return len(deployNames) == 0
}

func contains(hay, needle string) bool {
	return len(needle) > 0 && (hay == needle || len(hay) >= len(needle) && indexContains(hay, needle))
}

func indexContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// NormalizeEvents converts EventRecords with stable timestamps.
func NormalizeEvents(events []model.EventRecord) []model.EventRecord {
	out := make([]model.EventRecord, len(events))
	copy(out, events)
	for i := range out {
		if out[i].Timestamp.IsZero() {
			out[i].Timestamp = model.TimestampFrom(time.Now().UTC())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}
