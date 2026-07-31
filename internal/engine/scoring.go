package engine

import (
	"fmt"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// ScoreSignals produces deterministic signals from bundle evidence.
func ScoreSignals(b model.EvidenceBundle) []model.Signal {
	var signals []model.Signal

	for _, p := range b.Pods {
		for _, c := range p.Containers {
			if c.LastReason == "OOMKilled" || strings.Contains(strings.ToLower(c.LastReason), "oom") {
				signals = append(signals, model.Signal{
					ID: "oom_killed", Label: "OOMKilled", Severity: model.SeverityCritical, Strength: "strong",
					Score: 95, Evidence: fmt.Sprintf("container %s/%s reason=%s", p.Name, c.Name, c.LastReason),
					ObjectRef: model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
				})
			}
			if strings.Contains(strings.ToLower(c.LastReason), "crashloop") || strings.Contains(strings.ToLower(c.Reason), "crashloop") {
				signals = append(signals, model.Signal{
					ID: "crashloop", Label: "CrashLoopBackOff", Severity: model.SeverityCritical, Strength: "strong",
					Score: 92, Evidence: fmt.Sprintf("container %s/%s", p.Name, c.Name),
					ObjectRef: model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
				})
			}
			if c.RestartCount >= 3 {
				signals = append(signals, model.Signal{
					ID: "high_restarts", Label: "High restart count", Severity: model.SeverityHigh, Strength: "strong",
					Score: 80, Evidence: fmt.Sprintf("pod %s container %s restarts=%d", p.Name, c.Name, c.RestartCount),
					ObjectRef: model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
				})
			}
		}
	}

	for _, e := range b.Events {
		reason := strings.ToLower(e.Reason)
		switch {
		case strings.Contains(reason, "failedscheduling"):
			signals = append(signals, signalFromEvent(e, "failed_scheduling", "FailedScheduling", model.SeverityCritical, "strong", 90))
		case strings.Contains(reason, "failedmount"):
			signals = append(signals, signalFromEvent(e, "failed_mount", "FailedMount", model.SeverityCritical, "strong", 88))
		case strings.Contains(reason, "errimagepull"), strings.Contains(reason, "imagepullbackoff"):
			signals = append(signals, signalFromEvent(e, "image_pull", "ImagePullBackOff", model.SeverityCritical, "strong", 88))
		case strings.Contains(reason, "backoff"):
			signals = append(signals, signalFromEvent(e, "backoff", "BackOff", model.SeverityHigh, "medium", 75))
		case strings.Contains(reason, "unhealthy"):
			signals = append(signals, signalFromEvent(e, "probe_fail", "Probe failure", model.SeverityHigh, "strong", 85))
		}
	}

	for _, svc := range b.Services {
		if svc.ReadyEndpoints == 0 {
			signals = append(signals, model.Signal{
				ID: "no_endpoints", Label: "Zero ready endpoints", Severity: model.SeverityHigh, Strength: "strong",
				Score: 85, Evidence: fmt.Sprintf("service %s ready=%d total=%d", svc.Name, svc.ReadyEndpoints, svc.TotalEndpoints),
				ObjectRef: model.ObjectRef{Kind: "Service", Name: svc.Name, Namespace: svc.Namespace},
			})
		}
	}

	for _, n := range b.Nodes {
		if !n.Ready {
			signals = append(signals, model.Signal{
				ID: "node_not_ready", Label: "Node NotReady", Severity: model.SeverityCritical, Strength: "strong",
				Score: 90, Evidence: "node " + n.Name,
				ObjectRef: model.ObjectRef{Kind: "Node", Name: n.Name},
			})
		}
		if n.MemoryPressure || n.DiskPressure || n.PIDPressure {
			signals = append(signals, model.Signal{
				ID: "node_pressure", Label: "Node resource pressure", Severity: model.SeverityHigh, Strength: "medium",
				Score: 70, Evidence: fmt.Sprintf("node %s pressure mem=%v disk=%v pid=%v", n.Name, n.MemoryPressure, n.DiskPressure, n.PIDPressure),
				ObjectRef: model.ObjectRef{Kind: "Node", Name: n.Name},
			})
		}
	}

	for _, h := range b.HPAs {
		if h.AtMax {
			signals = append(signals, model.Signal{
				ID: "hpa_max", Label: "HPA at max replicas", Severity: model.SeverityWarning, Strength: "medium",
				Score: 55, Evidence: fmt.Sprintf("hpa %s current=%d max=%d", h.Name, h.CurrentReplicas, h.MaxReplicas),
				ObjectRef: model.ObjectRef{Kind: "HorizontalPodAutoscaler", Name: h.Name, Namespace: h.Namespace},
			})
		}
	}

	// Rollout shortly before failures (medium)
	if len(b.ReplicaSets) > 1 {
		signals = append(signals, model.Signal{
			ID: "rollout_activity", Label: "Recent rollout activity", Severity: model.SeverityWarning, Strength: "medium",
			Score: 50, Evidence: fmt.Sprintf("%d replicasets observed", len(b.ReplicaSets)),
		})
	}

	for _, w := range b.Warnings {
		if strings.Contains(strings.ToLower(w), "metrics") {
			signals = append(signals, model.Signal{
				ID: "metrics_missing", Label: "Metrics API unavailable", Severity: model.SeverityWarning, Strength: "weak",
				Score: 20, Evidence: w,
			})
		}
	}

	return signals
}

func signalFromEvent(e model.EventRecord, id, label string, sev model.Severity, strength string, score float64) model.Signal {
	return model.Signal{
		ID: id, Label: label, Severity: sev, Strength: strength, Score: score,
		Evidence: e.Message, ObjectRef: e.InvolvedObject,
	}
}
