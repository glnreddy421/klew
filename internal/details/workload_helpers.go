package details

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func conditionRows(conds []metav1.Condition) [][]string {
	var rows [][]string
	for _, c := range conds {
		rows = append(rows, []string{c.Type, string(c.Status), c.Reason, truncate(c.Message, 120)})
	}
	return rows
}

func podConditionRows(conds []corev1.PodCondition) [][]string {
	var rows [][]string
	for _, c := range conds {
		rows = append(rows, []string{string(c.Type), string(c.Status), c.Reason, truncate(c.Message, 120)})
	}
	return rows
}

func containerRows(containers []corev1.Container) [][]string {
	var rows [][]string
	for _, c := range containers {
		reqCPU, reqMem, limCPU, limMem := resourceStrings(c.Resources)
		rows = append(rows, []string{
			c.Name, c.Image, reqCPU, reqMem, limCPU, limMem, joinPorts(c.Ports),
		})
	}
	return rows
}

func resourceStrings(r corev1.ResourceRequirements) (reqCPU, reqMem, limCPU, limMem string) {
	if q, ok := r.Requests[corev1.ResourceCPU]; ok {
		reqCPU = q.String()
	}
	if q, ok := r.Requests[corev1.ResourceMemory]; ok {
		reqMem = q.String()
	}
	if q, ok := r.Limits[corev1.ResourceCPU]; ok {
		limCPU = q.String()
	}
	if q, ok := r.Limits[corev1.ResourceMemory]; ok {
		limMem = q.String()
	}
	return
}

func joinPorts(ports []corev1.ContainerPort) string {
	var parts []string
	for _, p := range ports {
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("%d", p.ContainerPort)
		}
		proto := string(p.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		parts = append(parts, fmt.Sprintf("%s/%s", name, proto))
	}
	return strings.Join(parts, ", ")
}

func volumeRows(vols []corev1.Volume) [][]string {
	var rows [][]string
	for _, v := range vols {
		rows = append(rows, []string{v.Name, volumeSource(v)})
	}
	return rows
}

func volumeSource(v corev1.Volume) string {
	switch {
	case v.ConfigMap != nil:
		return "configMap:" + v.ConfigMap.Name
	case v.Secret != nil:
		return "secret:" + v.Secret.SecretName
	case v.PersistentVolumeClaim != nil:
		return "pvc:" + v.PersistentVolumeClaim.ClaimName
	case v.EmptyDir != nil:
		return "emptyDir"
	case v.HostPath != nil:
		return "hostPath:" + v.HostPath.Path
	case v.Projected != nil:
		return "projected"
	case v.DownwardAPI != nil:
		return "downwardAPI"
	case v.CSI != nil:
		return "csi:" + v.CSI.Driver
	default:
		return "other"
	}
}

func envRows(containers []corev1.Container) [][]string {
	return envRowsDetailed(containers, nil)
}

// envRowsDetailed emits Container, Name, Source, Value.
// Optional resolver may fill ConfigMap values only; secret refs stay masked in this table.
func envRowsDetailed(containers []corev1.Container, resolver envValueResolver) [][]string {
	var rows [][]string
	for _, c := range containers {
		for _, e := range c.Env {
			source, value := envVarSourceValue(e, resolver)
			rows = append(rows, []string{c.Name, e.Name, source, truncate(value, 120)})
		}
		for _, ef := range c.EnvFrom {
			source, value := envFromSourceValue(ef, resolver)
			prefix := ef.Prefix
			if prefix == "" {
				prefix = "*"
			}
			rows = append(rows, []string{c.Name, prefix, source, truncate(value, 120)})
		}
	}
	return rows
}

const secretEnvPlaceholder = "(secret — reveal below if permitted)"

type envValueResolver func(sourceKind, name, key string) (string, bool)

type secretResolveResult struct {
	value     string
	found     bool
	forbidden bool
}

type secretEnvResolver func(name, key string) secretResolveResult

func envVarSourceValue(e corev1.EnvVar, resolver envValueResolver) (source, value string) {
	if e.ValueFrom == nil {
		return "literal", e.Value
	}
	switch {
	case e.ValueFrom.ConfigMapKeyRef != nil:
		ref := e.ValueFrom.ConfigMapKeyRef
		source = "configMap:" + ref.Name + "/" + ref.Key
		if resolver != nil {
			if v, ok := resolver("ConfigMap", ref.Name, ref.Key); ok {
				return source, v
			}
		}
		return source, source
	case e.ValueFrom.SecretKeyRef != nil:
		ref := e.ValueFrom.SecretKeyRef
		source = "secret:" + ref.Name + "/" + ref.Key
		return source, secretEnvPlaceholder
	case e.ValueFrom.FieldRef != nil:
		return "field", e.ValueFrom.FieldRef.FieldPath
	case e.ValueFrom.ResourceFieldRef != nil:
		r := e.ValueFrom.ResourceFieldRef
		return "resource", r.Resource + " (" + r.ContainerName + ")"
	default:
		return "valueFrom", ""
	}
}

func envFromSourceValue(ef corev1.EnvFromSource, resolver envValueResolver) (source, value string) {
	switch {
	case ef.ConfigMapRef != nil:
		source = "configMap:" + ef.ConfigMapRef.Name
		if resolver != nil {
			if v, ok := resolver("ConfigMap", ef.ConfigMapRef.Name, ""); ok {
				return source, v
			}
		}
		return source, "all keys"
	case ef.SecretRef != nil:
		source = "secret:" + ef.SecretRef.Name
		return source, secretEnvPlaceholder
	default:
		return "envFrom", ""
	}
}

func envFromRef(vf *corev1.EnvVarSource) string {
	if vf == nil {
		return ""
	}
	switch {
	case vf.ConfigMapKeyRef != nil:
		return "configMap:" + vf.ConfigMapKeyRef.Name + "/" + vf.ConfigMapKeyRef.Key
	case vf.SecretKeyRef != nil:
		return "secret:" + vf.SecretKeyRef.Name + "/" + vf.SecretKeyRef.Key
	case vf.FieldRef != nil:
		return "field:" + vf.FieldRef.FieldPath
	case vf.ResourceFieldRef != nil:
		return "resource:" + vf.ResourceFieldRef.Resource
	default:
		return "valueFrom"
	}
}

func containerSpecRows(containers []corev1.Container) [][]string {
	var rows [][]string
	for _, c := range containers {
		pull := string(c.ImagePullPolicy)
		if pull == "" {
			pull = "IfNotPresent"
		}
		rows = append(rows, []string{
			c.Name,
			c.Image,
			pull,
			truncate(strings.Join(c.Command, " "), 80),
			truncate(strings.Join(c.Args, " "), 80),
			c.WorkingDir,
		})
	}
	return rows
}

func volumeMountRows(containers []corev1.Container) [][]string {
	var rows [][]string
	for _, c := range containers {
		for _, m := range c.VolumeMounts {
			sub := m.SubPath
			if sub == "" && m.SubPathExpr != "" {
				sub = m.SubPathExpr
			}
			rows = append(rows, []string{
				c.Name,
				m.Name,
				m.MountPath,
				sub,
				boolStr(m.ReadOnly),
			})
		}
	}
	return rows
}

func probeRows(containers []corev1.Container) [][]string {
	var rows [][]string
	for _, c := range containers {
		rows = append(rows, probeRow(c.Name, "Liveness", c.LivenessProbe)...)
		rows = append(rows, probeRow(c.Name, "Readiness", c.ReadinessProbe)...)
		rows = append(rows, probeRow(c.Name, "Startup", c.StartupProbe)...)
	}
	return rows
}

func probeRow(container, probeType string, p *corev1.Probe) [][]string {
	if p == nil {
		return nil
	}
	kind, target := describeProbeHandler(p.ProbeHandler)
	return [][]string{{
		container,
		probeType,
		kind,
		target,
		fmtInt32(p.InitialDelaySeconds),
		fmtInt32(p.PeriodSeconds),
		fmtInt32(p.TimeoutSeconds),
		fmtInt32(p.FailureThreshold),
	}}
}

func describeProbeHandler(h corev1.ProbeHandler) (kind, target string) {
	switch {
	case h.HTTPGet != nil:
		path := h.HTTPGet.Path
		if path == "" {
			path = "/"
		}
		return "HTTP", path + " :" + h.HTTPGet.Port.String()
	case h.TCPSocket != nil:
		return "TCP", h.TCPSocket.Port.String()
	case h.Exec != nil:
		return "Exec", truncate(strings.Join(h.Exec.Command, " "), 80)
	case h.GRPC != nil:
		svc := ""
		if h.GRPC.Service != nil {
			svc = *h.GRPC.Service
		}
		return "GRPC", fmt.Sprintf("%d/%s", h.GRPC.Port, svc)
	default:
		return "unknown", ""
	}
}

func securityContextRows(containers []corev1.Container) [][]string {
	var rows [][]string
	for _, c := range containers {
		sc := c.SecurityContext
		if sc == nil {
			continue
		}
		rows = append(rows, []string{
			c.Name,
			fmtOptionalInt64(sc.RunAsUser),
			fmtOptionalInt64(sc.RunAsGroup),
			boolStr(sc.Privileged != nil && *sc.Privileged),
			boolStr(sc.ReadOnlyRootFilesystem != nil && *sc.ReadOnlyRootFilesystem),
		})
	}
	return rows
}

func fmtOptionalInt64(p *int64) string {
	if p == nil {
		return ""
	}
	return fmtInt64(*p)
}

func initContainerStateRows(sts []corev1.ContainerStatus) [][]string {
	var rows [][]string
	for _, s := range sts {
		state, reason, exit := describeContainerState(s.State)
		rows = append(rows, []string{
			s.Name, boolStr(s.Ready), fmtInt32(s.RestartCount), state, reason, exit, s.Image,
		})
	}
	return rows
}

func resolvedSecretEnvRows(containers []corev1.Container, resolver secretEnvResolver) ([][]string, []string) {
	if resolver == nil {
		return nil, nil
	}
	var rows [][]string
	var denied int
	for _, c := range containers {
		for _, e := range c.Env {
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
				continue
			}
			ref := e.ValueFrom.SecretKeyRef
			result := resolver(ref.Name, ref.Key)
			if result.forbidden {
				denied++
				continue
			}
			if !result.found || result.value == "" {
				continue
			}
			rows = append(rows, []string{c.Name, e.Name, ref.Name + "/" + ref.Key, result.value})
		}
	}
	var notes []string
	if denied > 0 {
		notes = append(notes, fmt.Sprintf(
			"%d secret environment variable(s) could not be resolved — insufficient permission to get Secrets.",
			denied,
		))
	}
	return rows, notes
}

func podTemplateSections(tpl *corev1.PodTemplateSpec, group string) []Section {
	if tpl == nil {
		return nil
	}
	var out []Section
	spec := tpl.Spec
	out = append(out, sectionFields("podTemplate", "Pod Template", group, fields(
		"Service Account", spec.ServiceAccountName,
		"Restart Policy", string(spec.RestartPolicy),
		"DNS Policy", string(spec.DNSPolicy),
		"Scheduler", spec.SchedulerName,
		"Priority Class", spec.PriorityClassName,
		"Node Selector", selectorString(spec.NodeSelector),
	)))
	if rows := containerRows(spec.Containers); len(rows) > 0 {
		out = append(out, sectionTable("containers", "Containers", group,
			[]string{"Name", "Image", "Req CPU", "Req Mem", "Lim CPU", "Lim Mem", "Ports"}, rows))
	}
	if rows := containerRows(spec.InitContainers); len(rows) > 0 {
		out = append(out, sectionTable("initContainers", "Init Containers", GroupRuntime,
			[]string{"Name", "Image", "Req CPU", "Req Mem", "Lim CPU", "Lim Mem", "Ports"}, rows))
	}
	if rows := volumeRows(spec.Volumes); len(rows) > 0 {
		out = append(out, sectionTable("volumes", "Volumes", group,
			[]string{"Name", "Source"}, rows))
	}
	if rows := volumeMountRows(append(spec.InitContainers, spec.Containers...)); len(rows) > 0 {
		out = append(out, sectionTable("volumeMounts", "Volume Mounts", group,
			[]string{"Container", "Volume", "Mount Path", "Sub Path", "Read Only"}, rows))
	}
	if rows := probeRows(append(spec.InitContainers, spec.Containers...)); len(rows) > 0 {
		out = append(out, sectionTable("probes", "Probes", group,
			[]string{"Container", "Probe", "Type", "Target", "Initial Delay", "Period", "Timeout", "Failures"}, rows))
	}
	if rows := envRowsDetailed(spec.Containers, nil); len(rows) > 0 {
		out = append(out, sectionTable("environment", "Environment", group,
			[]string{"Container", "Name", "Source", "Value"}, rows))
	}
	return out
}

func int32Or(p *int32, def int32) int32 {
	if p == nil {
		return def
	}
	return *p
}

func managedFieldsSection(mf []metav1.ManagedFieldsEntry) Section {
	if len(mf) == 0 {
		return Section{}
	}
	var rows [][]string
	for _, m := range mf {
		rows = append(rows, []string{m.Manager, string(m.Operation), m.APIVersion, string(m.FieldsType)})
	}
	return sectionTable("managedFields", "Managed Fields", GroupMetadata,
		[]string{"Manager", "Operation", "API Version", "Fields Type"}, rows)
}
