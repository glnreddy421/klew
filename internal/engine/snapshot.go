package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// SnapshotOptions configures initial collection.
type SnapshotOptions struct {
	Namespace string
	Query     string
	AllNS     bool
	Tail      int
}

// CollectSnapshot performs initial read-only snapshot via kube collector.
func CollectSnapshot(ctx context.Context, client *kube.Client, opts SnapshotOptions) (model.EvidenceBundle, []model.MatchedObject, error) {
	if opts.AllNS {
		return collectAllNamespaces(ctx, client, opts.Query)
	}
	ns := opts.Namespace
	collector := &kube.Collector{Client: client}
	bundle, err := collector.Collect(ctx, kube.CollectOptions{Namespace: ns, Query: opts.Query, LogLines: opts.Tail})
	if err != nil {
		return model.EvidenceBundle{}, nil, err
	}
	bundle.Metrics = kube.CollectMetrics(ctx, client, bundle.Pods)
	return bundle, bundle.MatchedObjects, nil
}

func collectAllNamespaces(ctx context.Context, client *kube.Client, query string) (model.EvidenceBundle, []model.MatchedObject, error) {
	nss, err := kube.ListNamespaces(ctx, client)
	if err != nil {
		return model.EvidenceBundle{}, nil, err
	}
	collector := &kube.Collector{Client: client}
	var merged model.EvidenceBundle
	var matches []model.MatchedObject
	for _, ns := range nss {
		b, err := collector.Collect(ctx, kube.CollectOptions{Namespace: ns, Query: query})
		if err != nil {
			continue
		}
		if len(b.MatchedObjects) == 0 && len(b.Pods) == 0 && len(b.Workloads) == 0 {
			continue
		}
		if merged.Namespace == "" {
			merged = b
			merged.Namespace = "*"
		} else {
			merged.MatchedObjects = append(merged.MatchedObjects, b.MatchedObjects...)
			merged.Workloads = append(merged.Workloads, b.Workloads...)
			merged.Pods = append(merged.Pods, b.Pods...)
			merged.ReplicaSets = append(merged.ReplicaSets, b.ReplicaSets...)
			merged.Services = append(merged.Services, b.Services...)
			merged.Ingresses = append(merged.Ingresses, b.Ingresses...)
			merged.Events = append(merged.Events, b.Events...)
			merged.Logs = append(merged.Logs, b.Logs...)
			merged.PreviousLogs = append(merged.PreviousLogs, b.PreviousLogs...)
			merged.HPAs = append(merged.HPAs, b.HPAs...)
			merged.Warnings = append(merged.Warnings, b.Warnings...)
		}
		matches = append(matches, b.MatchedObjects...)
	}
	if merged.Namespace == "" {
		merged = model.EvidenceBundle{
			CollectedAt: time.Now().UTC(),
			Namespace:   "*",
			Query:       query,
			KubeContext: model.KubeContext{
				Context: client.Context, Cluster: client.Cluster, User: client.User, Namespace: "*",
			},
			Warnings: []string{"no matching workloads found in any namespace"},
		}
	}
	merged.MatchedObjects = matches
	merged.Metrics = kube.CollectMetrics(ctx, client, merged.Pods)
	return merged, matches, nil
}

// BootstrapState builds initial InvestigationState from snapshot.
func BootstrapState(bundle model.EvidenceBundle, scope model.NamespaceScope, query string, mode model.Mode) *model.InvestigationState {
	st := model.NewInvestigationState(query, mode)
	st.KubeContext = bundle.KubeContext
	st.NamespaceScope = scope
	st.MatchedObjects = bundle.MatchedObjects
	st.Snapshot = bundle
	st.Permissions = bundle.Permissions
	st.Warnings = append(st.Warnings, bundle.Warnings...)
	st.Window = 15 * time.Minute
	st.TailLines = 200
	st.ExpectedWatches = 8
	st.WorkloadGraph = BuildGraph(bundle)
	st.Timeline = BuildTimeline(bundle)
	signals := ScoreSignals(bundle)
	st.Verdict = GenerateVerdict(bundle, st.Timeline, signals)
	return &st
}

// RefreshSnapshot re-collects and merges into reducer (periodic poll fallback).
func RefreshSnapshot(ctx context.Context, client *kube.Client, reducer *Reducer, ns, query string) error {
	bundle, _, err := CollectSnapshot(ctx, client, SnapshotOptions{Namespace: ns, Query: query})
	if err != nil {
		return err
	}
	inv := Analyze(bundle)
	reducer.ApplySnapshot(bundle, inv.Graph, inv.Timeline, inv.Verdict)
	return nil
}

// PublishSnapshotEvents emits snapshot k8s events onto the bus.
func PublishSnapshotEvents(bus *Bus, bundle model.EvidenceBundle) {
	for _, e := range bundle.Events {
		bus.Publish(model.EvidenceEvent{
			Timestamp:         e.Timestamp,
			SourceType:        model.SourceK8sEvent,
			SourceKind:        e.InvolvedObject.Kind,
			SourceName:        e.InvolvedObject.Name,
			Namespace:         e.InvolvedObject.Namespace,
			Severity:          eventRecordSeverity(e.Reason),
			Reason:            e.Reason,
			Message:           e.Message,
			Raw:               e.Message,
			Confidence:        0.85,
			RelatedObjectRefs: []model.ObjectRef{e.InvolvedObject},
		})
	}
	for _, lr := range bundle.Logs {
		for _, line := range tailLines(lr.Lines, 20) {
			bus.Publish(model.EvidenceEvent{
				Timestamp:  lr.CollectedAt,
				SourceType: model.SourceLog,
				SourceKind: "Pod",
				SourceName: lr.PodName,
				Namespace:  bundle.Namespace,
				Pod:        lr.PodName,
				Container:  lr.ContainerName,
				Severity:   ClassifyLogSeverity(line),
				Reason:     ClassifyLogReason(line),
				Message:    line,
				Raw:        line,
				Confidence: 0.7,
			})
		}
	}
	bus.Publish(model.EvidenceEvent{
		Timestamp:  time.Now().UTC(),
		SourceType: model.SourceSystem,
		Severity:   model.SeverityInfo,
		Reason:     "snapshot_complete",
		Message:    fmt.Sprintf("Initial snapshot: %d pods, %d events", len(bundle.Pods), len(bundle.Events)),
		Confidence: 1,
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

func tailLines(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}
