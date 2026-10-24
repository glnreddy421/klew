package changes

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/glnreddy421/klew/internal/model"
)

// Areas group diffs for summaries and the investigation pin.
const (
	AreaImage           = "image"
	AreaEnv             = "env"
	AreaEnvFrom         = "envFrom"
	AreaResources       = "resources"
	AreaProbes          = "probes"
	AreaCommand         = "command"
	AreaArgs            = "args"
	AreaPorts           = "ports"
	AreaVolumes         = "volumes"
	AreaContainers      = "containers"
	AreaServiceAccount  = "serviceAccount"
	AreaScheduling      = "scheduling"
	AreaSecurityContext = "securityContext"
	AreaLabels          = "labels"
	AreaAnnotations     = "annotations"
	AreaRestart         = "restart"
	AreaValues          = "values"
	AreaChart           = "chart"
)

const (
	podTemplateHashLabel = "pod-template-hash"
	controllerRevLabel   = "controller-revision-hash"
	restartedAtKey       = "kubectl.kubernetes.io/restartedAt"
)

// DiffPodTemplates compares two pod templates field by field. prev may be nil
// (first revision → no diffs).
func DiffPodTemplates(prev, next *corev1.PodTemplateSpec) []model.FieldDiff {
	if prev == nil || next == nil {
		return nil
	}
	var out []model.FieldDiff
	out = append(out, diffContainers("containers", prev.Spec.Containers, next.Spec.Containers)...)
	out = append(out, diffContainers("initContainers", prev.Spec.InitContainers, next.Spec.InitContainers)...)

	ps, ns := &prev.Spec, &next.Spec
	out = appendChanged(out, "serviceAccountName", ps.ServiceAccountName, ns.ServiceAccountName, AreaServiceAccount)
	out = append(out, diffStringMap("nodeSelector", ps.NodeSelector, ns.NodeSelector, AreaScheduling, nil)...)
	out = appendChanged(out, "tolerations", tolerationsString(ps.Tolerations), tolerationsString(ns.Tolerations), AreaScheduling)
	out = appendChanged(out, "affinity", jsonString(ps.Affinity), jsonString(ns.Affinity), AreaScheduling)
	out = appendChanged(out, "securityContext", jsonString(ps.SecurityContext), jsonString(ns.SecurityContext), AreaSecurityContext)
	out = append(out, diffVolumes(ps.Volumes, ns.Volumes)...)

	out = append(out, diffStringMap("labels", stripKeys(prev.Labels, podTemplateHashLabel, controllerRevLabel),
		stripKeys(next.Labels, podTemplateHashLabel, controllerRevLabel), AreaLabels, nil)...)
	out = append(out, diffAnnotations(prev.Annotations, next.Annotations)...)
	return out
}

func diffContainers(prefix string, prev, next []corev1.Container) []model.FieldDiff {
	var out []model.FieldDiff
	prevBy := map[string]corev1.Container{}
	for _, c := range prev {
		prevBy[c.Name] = c
	}
	nextBy := map[string]corev1.Container{}
	for _, c := range next {
		nextBy[c.Name] = c
	}
	for _, c := range prev {
		if _, ok := nextBy[c.Name]; !ok {
			out = append(out, model.FieldDiff{Path: fmt.Sprintf("%s[%s]", prefix, c.Name), Before: c.Image, Kind: model.DiffRemoved, Area: AreaContainers})
		}
	}
	for _, c := range next {
		p, ok := prevBy[c.Name]
		if !ok {
			out = append(out, model.FieldDiff{Path: fmt.Sprintf("%s[%s]", prefix, c.Name), After: c.Image, Kind: model.DiffAdded, Area: AreaContainers})
			continue
		}
		base := fmt.Sprintf("%s[%s]", prefix, c.Name)
		out = appendChanged(out, base+".image", p.Image, c.Image, AreaImage)
		out = appendChanged(out, base+".command", strings.Join(p.Command, " "), strings.Join(c.Command, " "), AreaCommand)
		out = appendChanged(out, base+".args", strings.Join(p.Args, " "), strings.Join(c.Args, " "), AreaArgs)
		out = appendChanged(out, base+".ports", portsString(p.Ports), portsString(c.Ports), AreaPorts)
		out = append(out, diffEnv(base, p.Env, c.Env)...)
		out = appendChanged(out, base+".envFrom", envFromString(p.EnvFrom), envFromString(c.EnvFrom), AreaEnvFrom)
		out = append(out, diffResources(base, p.Resources, c.Resources)...)
		out = appendChanged(out, base+".livenessProbe", probeString(p.LivenessProbe), probeString(c.LivenessProbe), AreaProbes)
		out = appendChanged(out, base+".readinessProbe", probeString(p.ReadinessProbe), probeString(c.ReadinessProbe), AreaProbes)
		out = appendChanged(out, base+".startupProbe", probeString(p.StartupProbe), probeString(c.StartupProbe), AreaProbes)
		out = append(out, diffMounts(base, p.VolumeMounts, c.VolumeMounts)...)
		out = appendChanged(out, base+".securityContext", jsonString(p.SecurityContext), jsonString(c.SecurityContext), AreaSecurityContext)
	}
	return out
}

// diffEnv compares env vars by name. Refs are shown as refs; plain values
// pass through RedactValue.
func diffEnv(base string, prev, next []corev1.EnvVar) []model.FieldDiff {
	prevBy := map[string]corev1.EnvVar{}
	for _, e := range prev {
		prevBy[e.Name] = e
	}
	nextBy := map[string]corev1.EnvVar{}
	for _, e := range next {
		nextBy[e.Name] = e
	}
	var out []model.FieldDiff
	for _, e := range prev {
		if _, ok := nextBy[e.Name]; !ok {
			out = append(out, model.FieldDiff{Path: base + ".env." + e.Name, Before: envValue(e), Kind: model.DiffRemoved, Area: AreaEnv})
		}
	}
	for _, e := range next {
		p, ok := prevBy[e.Name]
		if !ok {
			out = append(out, model.FieldDiff{Path: base + ".env." + e.Name, After: envValue(e), Kind: model.DiffAdded, Area: AreaEnv})
			continue
		}
		before, after := envValue(p), envValue(e)
		if envRaw(p) != envRaw(e) {
			out = append(out, model.FieldDiff{Path: base + ".env." + e.Name, Before: before, After: after, Kind: model.DiffChanged, Area: AreaEnv})
		}
	}
	return out
}

// envValue is the display value: a reference for valueFrom, a redacted plain value otherwise.
func envValue(e corev1.EnvVar) string {
	if vf := e.ValueFrom; vf != nil {
		switch {
		case vf.SecretKeyRef != nil:
			return fmt.Sprintf("secret/%s:%s", vf.SecretKeyRef.Name, vf.SecretKeyRef.Key)
		case vf.ConfigMapKeyRef != nil:
			return fmt.Sprintf("configmap/%s:%s", vf.ConfigMapKeyRef.Name, vf.ConfigMapKeyRef.Key)
		case vf.FieldRef != nil:
			return "field:" + vf.FieldRef.FieldPath
		case vf.ResourceFieldRef != nil:
			return fmt.Sprintf("resource:%s/%s", vf.ResourceFieldRef.ContainerName, vf.ResourceFieldRef.Resource)
		}
		return "valueFrom"
	}
	return RedactValue(e.Name, e.Value)
}

// envRaw compares the real definition (including secret-looking values) so a
// change is detected even when both sides display as redacted.
func envRaw(e corev1.EnvVar) string {
	if e.ValueFrom != nil {
		return "ref:" + jsonString(e.ValueFrom)
	}
	return "val:" + e.Value
}

func envFromString(in []corev1.EnvFromSource) string {
	var parts []string
	for _, ef := range in {
		switch {
		case ef.SecretRef != nil:
			parts = append(parts, ef.Prefix+"secret/"+ef.SecretRef.Name)
		case ef.ConfigMapRef != nil:
			parts = append(parts, ef.Prefix+"configmap/"+ef.ConfigMapRef.Name)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func diffResources(base string, prev, next corev1.ResourceRequirements) []model.FieldDiff {
	var out []model.FieldDiff
	for _, kind := range []struct {
		name       string
		prev, next corev1.ResourceList
	}{{"requests", prev.Requests, next.Requests}, {"limits", prev.Limits, next.Limits}} {
		keys := map[string]bool{}
		for k := range kind.prev {
			keys[string(k)] = true
		}
		for k := range kind.next {
			keys[string(k)] = true
		}
		names := make([]string, 0, len(keys))
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			pv, pok := kind.prev[corev1.ResourceName(k)]
			nv, nok := kind.next[corev1.ResourceName(k)]
			before, after := "", ""
			if pok {
				before = pv.String()
			}
			if nok {
				after = nv.String()
			}
			out = appendChanged(out, fmt.Sprintf("%s.resources.%s.%s", base, kind.name, k), before, after, AreaResources)
		}
	}
	return out
}

func diffMounts(base string, prev, next []corev1.VolumeMount) []model.FieldDiff {
	str := func(ms []corev1.VolumeMount) map[string]string {
		m := map[string]string{}
		for _, v := range ms {
			s := v.MountPath
			if v.SubPath != "" {
				s += " (subPath " + v.SubPath + ")"
			}
			if v.ReadOnly {
				s += " ro"
			}
			m[v.Name] = s
		}
		return m
	}
	return diffStringMap(base+".volumeMounts", str(prev), str(next), AreaVolumes, nil)
}

func diffVolumes(prev, next []corev1.Volume) []model.FieldDiff {
	str := func(vs []corev1.Volume) map[string]string {
		m := map[string]string{}
		for _, v := range vs {
			m[v.Name] = volumeSource(v)
		}
		return m
	}
	return diffStringMap("volumes", str(prev), str(next), AreaVolumes, nil)
}

// volumeSource names the source and its reference, never its content.
func volumeSource(v corev1.Volume) string {
	s := v.VolumeSource
	switch {
	case s.ConfigMap != nil:
		return "configmap/" + s.ConfigMap.Name
	case s.Secret != nil:
		return "secret/" + s.Secret.SecretName
	case s.PersistentVolumeClaim != nil:
		return "pvc/" + s.PersistentVolumeClaim.ClaimName
	case s.EmptyDir != nil:
		return "emptyDir"
	case s.HostPath != nil:
		return "hostPath " + s.HostPath.Path
	case s.Projected != nil:
		return "projected"
	case s.DownwardAPI != nil:
		return "downwardAPI"
	case s.CSI != nil:
		return "csi/" + s.CSI.Driver
	case s.Ephemeral != nil:
		return "ephemeral"
	default:
		return "other"
	}
}

func diffAnnotations(prev, next map[string]string) []model.FieldDiff {
	var out []model.FieldDiff
	if p, n := prev[restartedAtKey], next[restartedAtKey]; p != n && n != "" {
		out = append(out, model.FieldDiff{Path: "Restarted (rollout restart)", Before: p, After: n, Kind: model.DiffChanged, Area: AreaRestart})
	}
	out = append(out, diffStringMap("annotations", stripKeys(prev, restartedAtKey), stripKeys(next, restartedAtKey), AreaAnnotations, RedactValue)...)
	return out
}

// diffStringMap compares maps key by key, in key order. redact (optional) masks values.
func diffStringMap(base string, prev, next map[string]string, area string, redact func(k, v string) string) []model.FieldDiff {
	keys := map[string]bool{}
	for k := range prev {
		keys[k] = true
	}
	for k := range next {
		keys[k] = true
	}
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)
	show := func(k, v string) string {
		if redact != nil {
			return redact(k, v)
		}
		return v
	}
	var out []model.FieldDiff
	for _, k := range names {
		pv, pok := prev[k]
		nv, nok := next[k]
		path := base + "." + k
		switch {
		case pok && !nok:
			out = append(out, model.FieldDiff{Path: path, Before: show(k, pv), Kind: model.DiffRemoved, Area: area})
		case !pok && nok:
			out = append(out, model.FieldDiff{Path: path, After: show(k, nv), Kind: model.DiffAdded, Area: area})
		case pv != nv:
			out = append(out, model.FieldDiff{Path: path, Before: show(k, pv), After: show(k, nv), Kind: model.DiffChanged, Area: area})
		}
	}
	return out
}

// appendChanged adds a diff when before != after (added/removed when one side is empty).
func appendChanged(out []model.FieldDiff, path, before, after, area string) []model.FieldDiff {
	if before == after {
		return out
	}
	kind := model.DiffChanged
	switch {
	case before == "":
		kind = model.DiffAdded
	case after == "":
		kind = model.DiffRemoved
	}
	return append(out, model.FieldDiff{Path: path, Before: before, After: after, Kind: kind, Area: area})
}

func stripKeys(m map[string]string, drop ...string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	for _, d := range drop {
		delete(out, d)
	}
	return out
}

func portsString(ports []corev1.ContainerPort) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		s := fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol)
		if p.Protocol == "" {
			s = fmt.Sprintf("%d", p.ContainerPort)
		}
		if p.Name != "" {
			s = p.Name + "=" + s
		}
		parts = append(parts, s)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func probeString(p *corev1.Probe) string {
	if p == nil {
		return ""
	}
	h := p.ProbeHandler
	var target string
	switch {
	case h.HTTPGet != nil:
		target = fmt.Sprintf("http %s:%s", h.HTTPGet.Path, h.HTTPGet.Port.String())
	case h.TCPSocket != nil:
		target = "tcp :" + h.TCPSocket.Port.String()
	case h.GRPC != nil:
		target = fmt.Sprintf("grpc :%d", h.GRPC.Port)
	case h.Exec != nil:
		target = "exec " + strings.Join(h.Exec.Command, " ")
	}
	return fmt.Sprintf("%s delay=%ds period=%ds timeout=%ds failure=%d",
		target, p.InitialDelaySeconds, p.PeriodSeconds, p.TimeoutSeconds, p.FailureThreshold)
}

func tolerationsString(ts []corev1.Toleration) string {
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, fmt.Sprintf("%s%s%s:%s", t.Key, opString(t.Operator), t.Value, t.Effect))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func opString(op corev1.TolerationOperator) string {
	if op == corev1.TolerationOpExists {
		return " exists"
	}
	return "="
}

func jsonString(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" || string(b) == "{}" {
		return ""
	}
	return string(b)
}
