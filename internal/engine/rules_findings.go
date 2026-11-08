package engine

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// Finding rules. Each reports what is wrong with one object and the spec and
// status facts behind it. Container termination messages are left out: they
// can hold application log text, which callers gate separately.

// podStateFindings reports every unready pod, worst first.
func podStateFindings(b model.EvidenceBundle) []model.Finding {
	pods := make([]model.PodSummary, 0, len(b.Pods))
	for _, p := range b.Pods {
		if PodCountsAsUnready(p) {
			pods = append(pods, p)
		}
	}
	sort.SliceStable(pods, func(i, j int) bool { return pods[i].RestartCount > pods[j].RestartCount })
	out := make([]model.Finding, 0, len(pods))
	events := eventsByPod(b)
	for _, p := range pods {
		state, sev, facts := podProblem(p, events[p.Name])
		out = append(out, model.Finding{
			Object:   model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
			Severity: sev, State: state, Facts: facts,
		})
	}
	return out
}

// podProblem names a pod's state and the facts a person would check first.
func podProblem(p model.PodSummary, events []model.EventRecord) (string, model.Severity, []string) {
	var facts []string
	add := func(format string, args ...any) { facts = append(facts, fmt.Sprintf(format, args...)) }
	if p.Unschedulable != "" {
		add("%s", p.Unschedulable)
		for _, c := range p.Containers {
			if c.RequestsCPU != "" || c.RequestsMem != "" {
				add("container %s requests cpu %s, memory %s", c.Name, orNone(c.RequestsCPU), orNone(c.RequestsMem))
			}
		}
		return "Unschedulable", model.SeverityCritical, facts
	}
	if strings.EqualFold(p.Phase, "Failed") {
		for _, c := range p.Containers {
			if c.State == "terminated" && c.ExitCode != 0 {
				add("container %s exited %d (%s)", c.Name, c.ExitCode, orNone(c.Reason))
			}
		}
		return "Failed", model.SeverityHigh, facts
	}
	for _, c := range p.Containers {
		if c.Ready {
			continue
		}
		state, sev := containerState(c, p.Phase)
		return state, sev, append(facts, containerFacts(events, c, state)...)
	}
	return orNone(p.Phase), model.SeverityWarning, facts
}

// containerFacts lists what explains a not-ready container's state.
func containerFacts(events []model.EventRecord, c model.ContainerStatus, state string) []string {
	var facts []string
	add := func(format string, args ...any) { facts = append(facts, fmt.Sprintf(format, args...)) }
	if c.LastReason != "" {
		add("container %s last exited %d (%s)", containerLabel(c), c.LastExitCode, c.LastReason)
		if c.LastReason == "OOMKilled" && c.LimitsMem != "" {
			add("memory limit %s, request %s", c.LimitsMem, orNone(c.RequestsMem))
		}
	}
	if msg := probeFailure(events, "Liveness"); msg != "" && c.LivenessProbe != "" {
		add("restarted by its liveness probe %s: %s", c.LivenessProbe, msg)
	}
	switch {
	case strings.Contains(state, "ImagePull") || strings.HasSuffix(state, "ErrImagePull") || strings.HasSuffix(state, "InvalidImageName"):
		add("image %s", c.Image)
	case state == "NotReady":
		if c.ReadinessProbe != "" {
			add("readiness probe %s", c.ReadinessProbe)
		}
		if msg := probeFailure(events, "Readiness"); msg != "" {
			add("%s", msg)
		}
		if len(c.Ports) > 0 {
			add("declared ports %s", strings.Join(c.Ports, ", "))
		}
	case strings.HasSuffix(state, "CreateContainerConfigError") && c.Message != "":
		add("%s", c.Message)
	}
	if c.LastReason != "" {
		if len(c.Mounts) > 0 {
			add("mounts %s", strings.Join(c.Mounts, ", "))
		} else {
			add("no volume mounts")
		}
	}
	return facts
}

// containerState names a not-ready container's state; init containers get an
// "Init:" prefix, as kubectl shows them.
func containerState(c model.ContainerStatus, phase string) (string, model.Severity) {
	state, sev := orNone(phase), model.SeverityWarning
	switch {
	case c.State == "waiting" && c.Reason != "":
		state = c.Reason
		if isFailureReason(strings.ToLower(c.Reason)) {
			sev = model.SeverityCritical
		}
	case c.State == "terminated":
		state, sev = orNone(c.Reason), model.SeverityHigh
	case c.State == "running":
		state, sev = "NotReady", model.SeverityHigh
	}
	if c.Init {
		state = "Init:" + state
	}
	return state, sev
}

func containerLabel(c model.ContainerStatus) string {
	if c.Init {
		return c.Name + " (init)"
	}
	return c.Name
}

// currentEventWindow bounds the events rules treat as describing the present.
const currentEventWindow = time.Hour

// currentEvent reports whether e is a live event seen within the hour before
// the snapshot. Events recorded from history describe the past: a probe that
// failed yesterday, or a CronJob run that has since recovered.
func currentEvent(b model.EvidenceBundle, e model.EventRecord) bool {
	if e.Recorded {
		return false
	}
	if b.CollectedAt.IsZero() {
		return true
	}
	t := eventTime(e)
	return t.IsZero() || !t.Before(model.TimestampFrom(b.CollectedAt.Time().Add(-currentEventWindow)))
}

// eventsByPod indexes the bundle's current events by the pod they are about.
func eventsByPod(b model.EvidenceBundle) map[string][]model.EventRecord {
	out := map[string][]model.EventRecord{}
	for _, e := range b.Events {
		if !currentEvent(b, e) {
			continue
		}
		if e.InvolvedObject.Kind == "Pod" || e.InvolvedObject.Kind == "" {
			out[e.InvolvedObject.Name] = append(out[e.InvolvedObject.Name], e)
		}
	}
	return out
}

// probeFailure returns the newest "<kind> probe failed" message among a pod's events.
func probeFailure(events []model.EventRecord, kind string) string {
	var msg string
	var newest model.Timestamp
	for _, e := range events {
		if e.Reason != "Unhealthy" || !strings.HasPrefix(e.Message, kind+" probe failed") {
			continue
		}
		if t := eventTime(e); msg == "" || t.After(newest) {
			msg, newest = e.Message, t
		}
	}
	return msg
}

func eventTime(e model.EventRecord) model.Timestamp {
	if !e.LastSeen.IsZero() {
		return e.LastSeen
	}
	return e.Timestamp
}

// livenessFindings reports pods that look Ready but whose liveness probe keeps
// restarting a container. Unready pods are covered by pod-state.
func livenessFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	events := eventsByPod(b)
	for _, p := range b.Pods {
		if PodCountsAsUnready(p) {
			continue
		}
		msg := probeFailure(events[p.Name], "Liveness")
		if msg == "" {
			continue
		}
		for _, c := range p.Containers {
			if c.Init || c.LivenessProbe == "" || c.RestartCount == 0 {
				continue
			}
			out = append(out, model.Finding{
				Object:   model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
				Severity: model.SeverityHigh, State: "LivenessRestarts",
				Facts: []string{
					fmt.Sprintf("container %s restarted %d times by its liveness probe %s", c.Name, c.RestartCount, c.LivenessProbe),
					msg,
				},
			})
			break
		}
	}
	return out
}

// podCreationFindings reports ReplicaSets whose pods the API rejects, e.g. by
// a LimitRange, ResourceQuota or admission webhook. No pod exists to show it.
func podCreationFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, rs := range b.ReplicaSets {
		if rs.CreateFailure == "" {
			continue
		}
		out = append(out, model.Finding{
			Object:   model.ObjectRef{Kind: "ReplicaSet", Name: rs.Name, Namespace: rs.Namespace},
			Severity: model.SeverityCritical, State: "PodCreationRejected", Facts: []string{rs.CreateFailure},
		})
	}
	return out
}

// jobFindings reports Jobs that have failed for good.
func jobFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, j := range b.Jobs {
		if j.FailedReason == "" {
			continue
		}
		facts := []string{fmt.Sprintf("%d failed pods, %d succeeded", j.Failed, j.Succeeded)}
		if j.FailedMessage != "" {
			facts = append([]string{j.FailedMessage}, facts...)
		}
		out = append(out, model.Finding{
			Object:   model.ObjectRef{Kind: "Job", Name: j.Name, Namespace: j.Namespace},
			Severity: model.SeverityHigh, State: j.FailedReason, Facts: facts,
		})
	}
	return out
}

// deploymentConditionFindings reports False Deployment conditions such as
// ProgressDeadlineExceeded. They add context to the pods' own findings.
func deploymentConditionFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, w := range b.Workloads {
		for _, c := range w.FailedConditions {
			state := "ConditionFalse"
			if parts := strings.SplitN(c, ": ", 3); len(parts) >= 2 && parts[1] != "" {
				state = parts[1]
			}
			out = append(out, model.Finding{
				Object:   model.ObjectRef{Kind: w.Kind, Name: w.Name, Namespace: w.Namespace},
				Severity: model.SeverityWarning, State: state, Facts: []string{c},
			})
		}
	}
	return out
}

// serviceSelectorFindings reports Services with a selector but no endpoints at
// all, and names the pods that come closest to matching it.
func serviceSelectorFindings(b model.EvidenceBundle) []model.Finding {
	if !allowedCheck(b.Permissions, "endpointslices", "list") {
		return nil
	}
	var out []model.Finding
	for _, svc := range b.Services {
		if svc.Selector == "" || svc.TotalEndpoints > 0 {
			continue
		}
		want, err := labels.ConvertSelectorToLabelsMap(svc.Selector)
		if err != nil {
			continue
		}
		f := model.Finding{
			Object:   model.ObjectRef{Kind: "Service", Name: svc.Name, Namespace: svc.Namespace},
			Severity: model.SeverityHigh, State: "SelectorMatchesNoPods",
		}
		sel := labels.SelectorFromSet(want)
		matching := 0
		for _, p := range b.Pods {
			if sel.Matches(labels.Set(p.Labels)) {
				matching++
			}
		}
		if matching > 0 {
			f.State = "NoEndpoints"
			f.Facts = []string{fmt.Sprintf("selector %s matches %d pods, none with an address yet", svc.Selector, matching)}
		} else {
			f.Facts = append([]string{fmt.Sprintf("selector %s matches no pods", svc.Selector)}, nearestPodMismatch(b.Pods, want)...)
		}
		out = append(out, f)
	}
	return out
}

// nearestPodMismatch describes how the pod sharing the most selector labels
// differs from the selector, e.g. "pod gateway-abc has tier=edge, not tier=frontend".
func nearestPodMismatch(pods []model.PodSummary, want labels.Set) []string {
	var best *model.PodSummary
	bestShared := 0
	for i, p := range pods {
		shared := 0
		for k, v := range want {
			if p.Labels[k] == v {
				shared++
			}
		}
		if shared > bestShared {
			best, bestShared = &pods[i], shared
		}
	}
	if best == nil {
		return nil
	}
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var diffs []string
	for _, k := range keys {
		got, ok := best.Labels[k]
		switch {
		case !ok:
			diffs = append(diffs, fmt.Sprintf("no %s label (selector wants %s)", k, want[k]))
		case got != want[k]:
			diffs = append(diffs, fmt.Sprintf("%s=%s, not %s=%s", k, got, k, want[k]))
		}
	}
	return []string{fmt.Sprintf("closest pod %s has %s", best.Name, strings.Join(diffs, "; "))}
}

// serviceUnreadyFindings reports Services whose endpoints exist but none are ready.
func serviceUnreadyFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, svc := range b.Services {
		if svc.TotalEndpoints > 0 && svc.ReadyEndpoints == 0 {
			out = append(out, model.Finding{
				Object:   model.ObjectRef{Kind: "Service", Name: svc.Name, Namespace: svc.Namespace},
				Severity: model.SeverityHigh, State: "NoReadyEndpoints",
				Facts: []string{fmt.Sprintf("0/%d endpoints ready", svc.TotalEndpoints)},
			})
		}
	}
	return out
}

func allowedCheck(perms []model.PermissionCheck, resource, verb string) bool {
	for _, p := range perms {
		if p.Resource == resource && p.Verb == verb {
			return p.Allowed
		}
	}
	return false
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
