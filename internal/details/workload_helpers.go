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
	var rows [][]string
	for _, c := range containers {
		for _, e := range c.Env {
			val := e.Value
			if e.ValueFrom != nil {
				val = envFromRef(e.ValueFrom)
			}
			rows = append(rows, []string{c.Name, e.Name, truncate(val, 80)})
		}
		for _, ef := range c.EnvFrom {
			src := ""
			if ef.ConfigMapRef != nil {
				src = "configMap:" + ef.ConfigMapRef.Name
			} else if ef.SecretRef != nil {
				src = "secret:" + ef.SecretRef.Name
			}
			rows = append(rows, []string{c.Name, ef.Prefix + "*", src})
		}
	}
	return rows
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
	if rows := envRows(spec.Containers); len(rows) > 0 {
		out = append(out, sectionTable("environment", "Environment", group,
			[]string{"Container", "Name", "Value"}, rows))
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
