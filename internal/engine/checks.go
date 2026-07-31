package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

// failingPod picks the pod/container most likely responsible for the incident.
func failingPod(b model.EvidenceBundle) (pod, container string) {
	best := -1
	for _, p := range b.Pods {
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

func workloadSelector(b model.EvidenceBundle) string {
	if len(b.Workloads) > 0 && b.Workloads[0].Selector != "" {
		return b.Workloads[0].Selector
	}
	for _, p := range b.Pods {
		if len(p.Labels) == 0 {
			continue
		}
		parts := make([]string, 0, len(p.Labels))
		for k, v := range p.Labels {
			parts = append(parts, k+"="+v)
		}
		if len(parts) > 0 {
			return strings.Join(parts, ",")
		}
	}
	return ""
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
		if oomN > 0 && limit > 0 {
			add(fmt.Sprintf("Confirm %d pod(s) exceeded the %s memory limit (OOMKilled, exit 137)", oomN, formatMemMi(limit)))
		} else if oomN > 0 {
			add(fmt.Sprintf("Confirm all %d failing pod(s) were OOMKilled (exit 137)", oomN))
		} else {
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
		if pod != "" && container != "" {
			add(fmt.Sprintf("Inspect last termination state on %s/%s (exit code and reason)", pod, container))
		} else if pod != "" {
			add(fmt.Sprintf("Inspect last termination state on pod %s", pod))
		} else {
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
		add("Confirm dependency reachability and latency (e.g. redis, database)")
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

// kubectlChecksFor returns copy-pasteable verification commands scoped to the bundle.
func kubectlChecksFor(b model.EvidenceBundle, cat string) []string {
	ns := b.Namespace
	if ns == "" || ns == "*" {
		ns = "<namespace>"
	}
	var out []string
	pod, container := failingPod(b)
	selector := workloadSelector(b)

	add := func(s string) { out = appendUniqueCheck(out, s) }

	add(fmt.Sprintf("kubectl -n %s get deploy,rs,pod,svc,events", ns))

	if pod != "" {
		add(fmt.Sprintf("kubectl -n %s describe pod %s", ns, pod))
		if container != "" {
			add(fmt.Sprintf("kubectl -n %s logs %s -c %s --tail=200", ns, pod, container))
			add(fmt.Sprintf("kubectl -n %s logs %s -c %s --previous --tail=200", ns, pod, container))
		} else {
			add(fmt.Sprintf("kubectl -n %s logs %s --all-containers --tail=200", ns, pod))
			add(fmt.Sprintf("kubectl -n %s logs %s --all-containers --previous --tail=200", ns, pod))
		}
	} else if selector != "" {
		add(fmt.Sprintf("kubectl -n %s get pods -l %s", ns, selector))
		add(fmt.Sprintf("kubectl -n %s describe pod -l %s", ns, selector))
	}

	if len(b.Workloads) > 0 {
		w := b.Workloads[0]
		switch w.Kind {
		case "Deployment":
			add(fmt.Sprintf("kubectl -n %s rollout history deploy/%s", ns, w.Name))
			add(fmt.Sprintf("kubectl -n %s logs deploy/%s --all-containers --tail=200", ns, w.Name))
		case "StatefulSet":
			add(fmt.Sprintf("kubectl -n %s logs sts/%s --all-containers --tail=200", ns, w.Name))
		}
	}

	switch cat {
	case "ImagePullBackOff":
		if pod != "" && container != "" {
			add(fmt.Sprintf("kubectl -n %s get pod %s -o jsonpath='{.status.containerStatuses[?(@.name==%q)].state.waiting.message}'", ns, pod, container))
		}
	case "Mount/config":
		if pod != "" {
			add(fmt.Sprintf("kubectl -n %s get pod %s -o yaml | grep -E 'configMap|secret'", ns, pod))
		}
	}

	return out
}

// fixActionsFor returns remediation steps to restore service health (read-only guidance).
func fixActionsFor(cat string, b model.EvidenceBundle) []string {
	ns := b.Namespace
	if ns == "" || ns == "*" {
		ns = "<namespace>"
	}
	var out []string
	add := func(s string) { out = appendUniqueCheck(out, s) }

	workload := ""
	kind := "deployment"
	if len(b.Workloads) > 0 {
		workload = b.Workloads[0].Name
		switch b.Workloads[0].Kind {
		case "StatefulSet":
			kind = "statefulset"
		case "DaemonSet":
			kind = "daemonset"
		}
	}

	switch cat {
	case "CrashLoopBackOff":
		if workload != "" {
			add(fmt.Sprintf("kubectl -n %s rollout undo %s/%s", ns, kind, workload))
			add(fmt.Sprintf("kubectl -n %s patch %s %s --type=json -p='[{\"op\":\"remove\",\"path\":\"/spec/template/spec/containers/0/command\"}]'", ns, kind, workload))
		}
		add("klew-labs: cd scenarios/05-crashloop && ./cleanup.sh")
	case "OOMKilled":
		if workload != "" {
			add(fmt.Sprintf("kubectl -n %s rollout undo %s/%s", ns, kind, workload))
			add(fmt.Sprintf("kubectl -n %s set resources %s/%s -c app --limits=memory=512Mi", ns, kind, workload))
		}
		add("klew-labs: cd scenarios/04-oomkill && ./cleanup.sh")
	case "Rollout correlation":
		if workload != "" {
			add(fmt.Sprintf("kubectl -n %s rollout undo %s/%s", ns, kind, workload))
			add(fmt.Sprintf("kubectl -n %s rollout status %s/%s", ns, kind, workload))
		}
	case "ImagePullBackOff":
		if workload != "" {
			add(fmt.Sprintf("kubectl -n %s rollout undo %s/%s", ns, kind, workload))
			add(fmt.Sprintf("kubectl -n %s set image %s/%s app=<known-good-image>", ns, kind, workload))
		}
	case "Mount/config", "Readiness failed", "Service impact":
		if workload != "" {
			add(fmt.Sprintf("kubectl -n %s rollout undo %s/%s", ns, kind, workload))
		}
	case "Dependency failure":
		add("Restore dependency reachability (e.g. redis) then restart affected pods")
		if workload != "" {
			add(fmt.Sprintf("kubectl -n %s rollout restart %s/%s", ns, kind, workload))
		}
	}
	return out
}
