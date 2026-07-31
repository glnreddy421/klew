package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// BuildTimeline merges events, container states, rollouts into chronological story.
func BuildTimeline(b model.EvidenceBundle) []model.TimelineEvent {
	var events []model.TimelineEvent

	for _, e := range b.Events {
		sev, conf := eventSeverity(e.Reason, e.Message)
		events = append(events, model.TimelineEvent{
			Timestamp:  e.Timestamp,
			Type:       "event",
			Severity:   sev,
			SourceKind: e.InvolvedObject.Kind,
			SourceName: e.InvolvedObject.Name,
			Namespace:  b.Namespace,
			Message:    e.Message,
			Reason:     e.Reason,
			InvolvedObject: e.InvolvedObject,
			Confidence: conf,
			EvidenceRefs: []string{fmt.Sprintf("event:%s/%s:%s", e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Reason)},
		})
	}

	for _, p := range b.Pods {
		events = append(events, model.TimelineEvent{
			Timestamp:  p.CreatedAt,
			Type:       "pod",
			Severity:   model.SeverityInfo,
			SourceKind: "Pod",
			SourceName: p.Name,
			Namespace:  p.Namespace,
			Message:    fmt.Sprintf("Pod created phase=%s ready=%v restarts=%d", p.Phase, p.Ready, p.RestartCount),
			Confidence: 0.9,
			InvolvedObject: model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
			EvidenceRefs: []string{"pod:" + p.Name},
		})
		for _, c := range p.Containers {
			if c.LastReason != "" {
				sev := containerReasonSeverity(c.LastReason)
				events = append(events, model.TimelineEvent{
					Timestamp:  timeOrNow(c.FinishedAt),
					Type:       "container",
					Severity:   sev,
					SourceKind: "Container",
					SourceName: c.Name,
					Namespace:  p.Namespace,
					Message:    fmt.Sprintf("Container %s last state=%s reason=%s exit=%d restarts=%d", c.Name, c.LastState, c.LastReason, c.LastExitCode, c.RestartCount),
					Reason:     c.LastReason,
					Confidence: 0.85,
					InvolvedObject: model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
					EvidenceRefs: []string{fmt.Sprintf("container:%s/%s", p.Name, c.Name)},
				})
			}
		}
	}

	for _, rs := range b.ReplicaSets {
		events = append(events, model.TimelineEvent{
			Timestamp:  rs.CreatedAt,
			Type:       "rollout",
			Severity:   model.SeverityInfo,
			SourceKind: "ReplicaSet",
			SourceName: rs.Name,
			Namespace:  rs.Namespace,
			Message:    fmt.Sprintf("ReplicaSet replicas=%d ready=%d owner=%s", rs.Replicas, rs.Ready, rs.DeploymentOwner),
			Confidence: 0.8,
			InvolvedObject: model.ObjectRef{Kind: "ReplicaSet", Name: rs.Name, Namespace: rs.Namespace},
		})
	}

	for _, svc := range b.Services {
		if svc.ReadyEndpoints == 0 && svc.TotalEndpoints == 0 {
			events = append(events, model.TimelineEvent{
				Timestamp:  b.CollectedAt,
				Type:       "service",
				Severity:   model.SeverityHigh,
				SourceKind: "Service",
				SourceName: svc.Name,
				Namespace:  svc.Namespace,
				Message:    "Service has zero ready endpoints",
				Reason:     "NoEndpoints",
				Confidence: 0.9,
				InvolvedObject: model.ObjectRef{Kind: "Service", Name: svc.Name, Namespace: svc.Namespace},
			})
		}
	}

	return SortTimeline(events)
}

func eventSeverity(reason, message string) (model.Severity, float64) {
	r := strings.ToLower(reason)
	m := strings.ToLower(message)
	switch {
	case strings.Contains(r, "oom") || strings.Contains(m, "oomkilled"):
		return model.SeverityCritical, 0.95
	case strings.Contains(r, "backoff") || strings.Contains(r, "crashloop"):
		return model.SeverityCritical, 0.92
	case strings.Contains(r, "failedscheduling"):
		return model.SeverityCritical, 0.9
	case strings.Contains(r, "failedmount"):
		return model.SeverityCritical, 0.9
	case strings.Contains(r, "errimagepull") || strings.Contains(r, "imagepullbackoff"):
		return model.SeverityCritical, 0.9
	case strings.Contains(r, "unhealthy") || strings.Contains(r, "failed"):
		return model.SeverityHigh, 0.85
	case strings.Contains(r, "pulling"):
		return model.SeverityInfo, 0.6
	default:
		return model.SeverityWarning, 0.7
	}
}

func containerReasonSeverity(reason string) model.Severity {
	r := strings.ToLower(reason)
	switch {
	case strings.Contains(r, "oom"), strings.Contains(r, "crashloop"), strings.Contains(r, "error"):
		return model.SeverityCritical
	case strings.Contains(r, "backoff"), strings.Contains(r, "imagepull"):
		return model.SeverityHigh
	default:
		return model.SeverityWarning
	}
}

func timeOrNow(t *time.Time) time.Time {
	if t == nil || t.IsZero() {
		return time.Now().UTC()
	}
	return *t
}
