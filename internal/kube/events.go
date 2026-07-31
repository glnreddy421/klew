package kube

import (
	"context"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
)

func (col *Collector) collectEvents(ctx context.Context, namespace string, deployNames []string) []model.EventRecord {
	evs, err := col.Client.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	var out []model.EventRecord
	for _, e := range evs.Items {
		kind := e.InvolvedObject.Kind
		// Infrastructure Patterns need Pod/Node/PVC lifecycle events even when the
		// investigation query is a free-text / DaemonSet name that would otherwise
		// exclude Node/PVC. Pods still prefer query match when names are set.
		if len(deployNames) > 0 {
			switch {
			case kind == "Node", kind == "PersistentVolumeClaim":
				// keep
			case kind == "Pod":
				if !eventMatchesDeploy(e, deployNames) {
					continue
				}
			default:
				if !eventMatchesDeploy(e, deployNames) {
					continue
				}
			}
		}
		ts := e.LastTimestamp.Time
		if ts.IsZero() {
			ts = e.EventTime.Time
		}
		if ts.IsZero() {
			ts = e.FirstTimestamp.Time
		}
		out = append(out, model.EventRecord{
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
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

func eventMatchesDeploy(e corev1.Event, deployNames []string) bool {
	name := e.InvolvedObject.Name
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

// unused import guard
var _ = metav1.CreateOptions{}
