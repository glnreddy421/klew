package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// failingPod picks the pod/container most likely responsible for the incident.
// Only unready or restarting pods qualify; with none, it returns empty names.
func failingPod(b model.EvidenceBundle) (pod, container string) {
	best := -1
	for _, p := range b.Pods {
		if !PodCountsAsUnready(p) && !podHasRestarts(p) {
			continue
		}
		for _, c := range p.Containers {
			score := int(c.RestartCount)
			reason := strings.ToLower(c.Reason + " " + c.LastReason + " " + c.State)
			switch {
			case strings.Contains(reason, "crashloop"):
				score += 20
			case strings.Contains(reason, "oom"):
				score += 18
			case strings.Contains(reason, "error"), strings.Contains(reason, "backoff"):
				score += 12
			case strings.Contains(reason, "imagepull"), strings.Contains(reason, "errimage"):
				score += 15
			case !c.Ready:
				score += 5
			}
			if score > best {
				best = score
				pod, container = p.Name, c.Name
			}
		}
	}
	return pod, container
}

func appendUniqueCheck(dst []string, s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return dst
	}
	for _, d := range dst {
		if d == s {
			return dst
		}
	}
	return append(dst, s)
}

// investigationStepsFor returns human-readable next steps for the leading hypothesis.
// These are shown in the Incident and Evidence tabs — not raw shell commands.
func investigationStepsFor(cat string, b model.EvidenceBundle) []string {
	pod, container := failingPod(b)
	workload := primaryWorkloadName(b)
	var out []string
	add := func(s string) { out = appendUniqueCheck(out, s) }

	switch cat {
	case "OOMKilled":
		oomN := oomPodCount(b)
		limit := podMemoryLimitMi(b, pod)
		switch {
		case oomN > 0 && limit > 0:
			add(fmt.Sprintf("Confirm %d pod(s) exceeded the %s memory limit (OOMKilled, exit 137)", oomN, formatMemMi(limit)))
		case oomN > 0:
			add(fmt.Sprintf("Confirm all %d failing pod(s) were OOMKilled (exit 137)", oomN))
		default:
			add("Confirm containers were killed for exceeding their memory limit")
		}
		if m := b.Metrics; m.Available && m.MemLimitMi > 0 && m.MemUsageMi > 0 {
			add(fmt.Sprintf("Compare live usage (%s) against the configured limit (%s)", formatMemMi(m.MemUsageMi), formatMemMi(m.MemLimitMi)))
		} else if !b.Metrics.Available {
			add("Establish whether usage was climbing toward the limit before each kill (live metrics unavailable)")
		}
		if pod != "" && container != "" {
			add(fmt.Sprintf("Review logs from %s/%s immediately before the OOM kill", pod, container))
		} else {
			add("Review logs from the last crash for memory growth or sudden allocation")
		}
		if hasRecentRollout(b) {
			add("Compare memory limits and container image between the new and previous revision")
		} else {
			add("Check for recent changes to memory limits, env vars, or workload traffic")
		}

	case "CrashLoopBackOff":
		if oomN := oomPodCount(b); oomN > 0 {
			add(fmt.Sprintf("Determine whether restarts on %d pod(s) are OOMKilled vs application exit errors", oomN))
		}
		switch {
		case pod != "" && container != "":
			add(fmt.Sprintf("Inspect last termination state on %s/%s (exit code and reason)", pod, container))
		case pod != "":
			add(fmt.Sprintf("Inspect last termination state on pod %s", pod))
		default:
			add("Inspect container exit reason and last termination state")
		}
		if pod != "" && container != "" {
			add(fmt.Sprintf("Read logs from %s/%s for the crash immediately before the restart", pod, container))
		} else {
			add("Read logs from the crash immediately before the last restart")
		}
		if hasRecentRollout(b) && workload != "" {
			add(fmt.Sprintf("Compare the current %s revision against the last healthy one", workload))
		}

	case "Service impact", "Readiness failed":
		add("Verify readiness probe configuration and recent probe failures")
		add("Confirm the service still has ready endpoints backing traffic")
		add("Check upstream dependencies the probe or app relies on")

	case "Rollout correlation":
		if workload != "" {
			add(fmt.Sprintf("Diff the current %s revision against the previous healthy revision", workload))
		} else {
			add("Diff the current rollout revision against the previous healthy revision")
		}
		add("Identify what changed in image, env, resources, or probes")
		add("Consider rolling back if the new revision correlates with failures")

	case "ImagePullBackOff":
		add("Verify the image tag exists in the registry and is accessible from the cluster")
		add("Check pull secrets and registry credentials referenced by the pod spec")
		if pod != "" {
			add(fmt.Sprintf("Read the pull error message on pod %s for the exact registry failure", pod))
		}

	case "Mount/config":
		add("Verify referenced ConfigMaps and Secrets exist with the expected keys")
		if pod != "" {
			add(fmt.Sprintf("Inspect mount and config errors on pod %s", pod))
		} else {
			add("Inspect mount and config errors on the failing pods")
		}

	case "Dependency failure":
		add("Confirm the dependency that is timing out is reachable and responsive")
		add("Check whether readiness failures track dependency timeouts in logs")
		if workload != "" {
			add(fmt.Sprintf("Validate %s connection settings and network path to dependencies", workload))
		}

	default:
		if workload != "" {
			add(fmt.Sprintf("Review recent events and pod status for %s", workload))
		} else {
			add("Review recent events and pod status for the affected workload")
		}
		if pod != "" {
			add(fmt.Sprintf("Inspect the failing pod %s and its container termination state", pod))
		}
	}
	return out
}

func primaryWorkloadName(b model.EvidenceBundle) string {
	if len(b.Workloads) > 0 {
		return b.Workloads[0].Name
	}
	return ""
}

func oomPodCount(b model.EvidenceBundle) int {
	n := 0
	for _, p := range b.Pods {
		if podActivelyOOMKilled(p) {
			n++
		}
	}
	return n
}

func podWasOOMKilled(p model.PodSummary) bool {
	return podActivelyOOMKilled(p)
}

func podMemoryLimitMi(b model.EvidenceBundle, podName string) int64 {
	for _, p := range b.Pods {
		if podName != "" && p.Name != podName {
			continue
		}
		var lim int64
		for _, c := range p.Containers {
			if v := parseMemQuantity(c.LimitsMem); v > 0 {
				lim += v
			}
		}
		if lim > 0 {
			return lim
		}
	}
	for _, p := range b.Pods {
		for _, c := range p.Containers {
			if v := parseMemQuantity(c.LimitsMem); v > 0 {
				return v
			}
		}
	}
	return 0
}

func parseMemQuantity(s string) int64 {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasSuffix(s, "Mi"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(s, "Mi"), 10, 64)
		return n
	case strings.HasSuffix(s, "Gi"):
		f, _ := strconv.ParseFloat(strings.TrimSuffix(s, "Gi"), 64)
		return int64(f * 1024)
	default:
		return 0
	}
}

func formatMemMi(mi int64) string {
	if mi >= 1024 {
		return fmt.Sprintf("%.1fGi", float64(mi)/1024)
	}
	return fmt.Sprintf("%dMi", mi)
}

func hasRecentRollout(b model.EvidenceBundle) bool {
	if len(b.ReplicaSets) < 2 {
		return false
	}
	readyOld, readyNew := 0, 0
	for _, rs := range b.ReplicaSets {
		if rs.Ready > 0 {
			readyOld++
		}
		if rs.Replicas > rs.Ready {
			readyNew++
		}
	}
	return readyOld > 0 && readyNew > 0
}

// fixActionsFor returns remediation commands for a person to review (never
// run). They name the workload and container that own the failing pod, and
// suggest a rollback only when a Deployment has a previous revision to return to.
func fixActionsFor(cat string, b model.EvidenceBundle) []string {
	ns := b.Namespace
	if ns == "" || ns == "*" {
		ns = "<namespace>"
	}
	var out []string
	add := func(s string) { out = appendUniqueCheck(out, s) }

	pod, container := failingPod(b)
	kind, name := failingWorkload(b, pod)
	if cat == "Dependency failure" {
		add("Restore reachability of the dependency that is timing out, then restart the affected pods")
	}
	if name == "" {
		return out
	}
	ref := strings.ToLower(kind) + "/" + name
	undo := func() {
		if kind == "Deployment" && deploymentRevisions(b, name) >= 2 {
			add(fmt.Sprintf("kubectl -n %s rollout undo %s", ns, ref))
		}
	}

	switch cat {
	case "CrashLoopBackOff", "Mount/config", "Readiness failed", "Service impact":
		undo()
	case "Rollout correlation":
		undo()
		add(fmt.Sprintf("kubectl -n %s rollout status %s", ns, ref))
	case "OOMKilled":
		if container != "" {
			limit := "<higher-limit>"
			if mi := containerMemLimitMi(b, pod, container); mi > 0 {
				limit = formatMemMi(mi * 2)
			}
			add(fmt.Sprintf("kubectl -n %s set resources %s -c %s --limits=memory=%s", ns, ref, container, limit))
		}
		undo()
	case "ImagePullBackOff":
		if container != "" {
			add(fmt.Sprintf("kubectl -n %s set image %s %s=<known-good-image>", ns, ref, container))
		}
		undo()
	case "Dependency failure":
		add(fmt.Sprintf("kubectl -n %s rollout restart %s", ns, ref))
	}
	return out
}

// failingWorkload resolves the workload that owns pod. With no owner to follow
// it falls back to the only collected workload, or returns empty names.
func failingWorkload(b model.EvidenceBundle, pod string) (kind, name string) {
	for _, p := range b.Pods {
		if p.Name == pod && len(p.OwnerRefs) > 0 {
			return PodWorkload(b, p)
		}
	}
	if len(b.Workloads) == 1 {
		return b.Workloads[0].Kind, b.Workloads[0].Name
	}
	return "", ""
}

// PodWorkload returns the Deployment, StatefulSet, DaemonSet, Job or CronJob
// that owns p, following a ReplicaSet to its Deployment and a Job to its
// CronJob. Other owners and unowned pods return empty names.
func PodWorkload(b model.EvidenceBundle, p model.PodSummary) (kind, name string) {
	if len(p.OwnerRefs) == 0 {
		return "", ""
	}
	o := p.OwnerRefs[0]
	switch o.Kind {
	case "StatefulSet", "DaemonSet":
		return o.Kind, o.Name
	case "Job":
		for _, j := range b.Jobs {
			if j.Name == o.Name && j.CronJob != "" {
				return "CronJob", j.CronJob
			}
		}
		return "Job", o.Name
	case "ReplicaSet":
		for _, rs := range b.ReplicaSets {
			if rs.Name == o.Name && rs.DeploymentOwner != "" {
				return "Deployment", rs.DeploymentOwner
			}
		}
		if hash := p.Labels["pod-template-hash"]; hash != "" && strings.HasSuffix(o.Name, "-"+hash) {
			return "Deployment", strings.TrimSuffix(o.Name, "-"+hash)
		}
	}
	return "", ""
}

// deploymentRevisions counts the collected ReplicaSets of a Deployment.
func deploymentRevisions(b model.EvidenceBundle, name string) int {
	n := 0
	for _, rs := range b.ReplicaSets {
		if rs.DeploymentOwner == name {
			n++
		}
	}
	return n
}

func containerMemLimitMi(b model.EvidenceBundle, pod, container string) int64 {
	for _, p := range b.Pods {
		if p.Name != pod {
			continue
		}
		for _, c := range p.Containers {
			if c.Name == container {
				return parseMemQuantity(c.LimitsMem)
			}
		}
	}
	return 0
}

func podHasRestarts(p model.PodSummary) bool {
	if p.RestartCount > 0 {
		return true
	}
	for _, c := range p.Containers {
		if c.RestartCount > 0 {
			return true
		}
	}
	return false
}
