package engine

import (
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// IncidentActive reports whether the current snapshot shows an ongoing failure.
func IncidentActive(b model.EvidenceBundle) bool {
	for _, w := range b.Workloads {
		if w.Replicas > 0 && w.Ready < w.Replicas {
			return true
		}
		if w.Replicas > 0 && w.Available < w.Replicas && w.Ready < w.Replicas {
			return true
		}
	}
	for _, svc := range b.Services {
		if svc.TotalEndpoints > 0 && svc.ReadyEndpoints == 0 {
			return true
		}
	}
	for _, p := range b.Pods {
		if podActivelyFailing(p) {
			return true
		}
	}
	return false
}

// WorkloadNominal is true when scoped pods and traffic paths look healthy now.
func WorkloadNominal(b model.EvidenceBundle) bool {
	if len(b.Pods) == 0 {
		return false
	}
	for _, w := range b.Workloads {
		if w.Replicas > 0 && w.Ready < w.Replicas {
			return false
		}
	}
	for _, svc := range b.Services {
		if svc.TotalEndpoints > 0 && svc.ReadyEndpoints < svc.TotalEndpoints {
			return false
		}
	}
	for _, p := range b.Pods {
		if !p.Ready || podActivelyFailing(p) {
			return false
		}
	}
	return true
}

func podActivelyFailing(p model.PodSummary) bool {
	// Pod Ready is the authoritative "running fine now" signal from Kubernetes.
	if p.Ready {
		return false
	}
	if p.Phase == "Failed" {
		return true
	}
	if !p.Ready && p.Phase != "Succeeded" && p.Phase != "Completed" {
		return true
	}
	for _, c := range p.Containers {
		if containerActivelyFailing(c) {
			return true
		}
	}
	return false
}

func containerActivelyFailing(c model.ContainerStatus) bool {
	reason := strings.ToLower(c.Reason)
	switch c.State {
	case "waiting":
		if isFailureReason(reason) {
			return true
		}
	case "terminated":
		if c.ExitCode == 137 || isFailureReason(reason) {
			return true
		}
	}
	if !c.Ready && c.State == "running" {
		return true
	}
	return false
}

func isFailureReason(r string) bool {
	if r == "" {
		return false
	}
	for _, kw := range []string{"crashloop", "oom", "error", "imagepull", "errimage", "backoff", "failed"} {
		if strings.Contains(r, kw) {
			return true
		}
	}
	return false
}

func podActivelyOOMKilled(p model.PodSummary) bool {
	for _, c := range p.Containers {
		if containerActivelyOOMKilled(c) {
			return true
		}
	}
	return false
}

func containerActivelyOOMKilled(c model.ContainerStatus) bool {
	reason := strings.ToLower(c.Reason)
	if c.State == "waiting" && strings.Contains(reason, "oom") {
		return true
	}
	if c.State == "terminated" && (c.ExitCode == 137 || strings.Contains(reason, "oom")) {
		return true
	}
	if !c.Ready && strings.Contains(strings.ToLower(c.LastReason), "oom") {
		return true
	}
	return false
}
