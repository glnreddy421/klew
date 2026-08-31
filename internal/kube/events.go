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

// collectEvents gathers namespace events for investigation and Event Patterns.
// scopePodNames lists every pod in the investigation snapshot — Pod events for
// those pods are always kept (independent of the search query).
func (col *Collector) collectEvents(ctx context.Context, namespace, query string, deployNames, scopePodNames []string) []model.EventRecord {
	scope := newInvestigationPods(scopePodNames)
	var out []model.EventRecord

	if recs, err := col.listCoreEvents(ctx, namespace, query, deployNames, scope); err == nil {
		out = append(out, recs...)
	}
	if recs, err := col.listEventsV1(ctx, namespace, query, deployNames, scope); err == nil {
		out = append(out, recs...)
	}

	out = dedupeEventRecords(out)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
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
	}
}

func dedupeEventRecords(in []model.EventRecord) []model.EventRecord {
	seen := map[string]struct{}{}
	out := make([]model.EventRecord, 0, len(in))
	key := func(e model.EventRecord) string {
		return e.InvolvedObject.Kind + "|" + e.InvolvedObject.Name + "|" + e.Reason + "|" + e.Message
	}
	for _, e := range in {
		k := key(e)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, e)
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

func eventMatchesDeploy(e corev1.Event, deployNames []string) bool {
	return eventNameMatchesDeploy(e.InvolvedObject.Name, deployNames)
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
