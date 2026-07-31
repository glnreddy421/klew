package details

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type podProvider struct{}

func (podProvider) Kind() string { return "Pod" }

func (podProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	pod, err := getPod(ctx, req)
	if err != nil {
		return nil, err
	}

	ready := false
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	d := &ObjectDetail{
		Title:    "Pod/" + pod.Name,
		Category: "runtime",
		Status:   statusFromPhase(string(pod.Status.Phase), ready),
		Summary: fields(
			"Phase", string(pod.Status.Phase),
			"Node", pod.Spec.NodeName,
			"Pod IP", pod.Status.PodIP,
			"QoS", string(pod.Status.QOSClass),
			"Service Account", pod.Spec.ServiceAccountName,
		),
	}

	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Phase", string(pod.Status.Phase),
		"Ready", boolStr(ready),
		"Start Time", fmtTime(pod.Status.StartTime),
		"Pod IP", pod.Status.PodIP,
		"Host IP", pod.Status.HostIP,
		"Nominated Node", pod.Status.NominatedNodeName,
	)))

	if rows := containerStateRows(pod.Status.ContainerStatuses); len(rows) > 0 {
		sections = append(sections, sectionTable("containerStates", "Container States", GroupStatus,
			[]string{"Name", "Ready", "Restarts", "State", "Reason", "Exit"}, rows))
	}
	if rows := restartHistoryRows(pod.Status.ContainerStatuses); len(rows) > 0 {
		sections = append(sections, sectionTable("restartHistory", "Restart History", GroupRuntime,
			[]string{"Name", "Restarts", "Last State", "Last Reason", "Last Exit"}, rows))
	}
	if rows := podConditionRows(pod.Status.Conditions); len(rows) > 0 {
		sections = append(sections, sectionTable("conditions", "Conditions", GroupStatus,
			[]string{"Type", "Status", "Reason", "Message"}, rows))
	}

	sections = append(sections, sectionFields("nodeAssignment", "Node Assignment", GroupRelationships, fields(
		"Node", pod.Spec.NodeName,
		"Nominated Node", pod.Status.NominatedNodeName,
		"Node Selector", selectorString(pod.Spec.NodeSelector),
		"Tolerations", fmt.Sprintf("%d", len(pod.Spec.Tolerations)),
	)))
	sections = append(sections, sectionFields("qos", "QoS", GroupRuntime, fields(
		"QoS Class", string(pod.Status.QOSClass),
		"Priority", fmtPriority(pod.Spec.Priority),
		"Priority Class", pod.Spec.PriorityClassName,
	)))

	if rows := containerResourceRows(pod); len(rows) > 0 {
		sections = append(sections, sectionTable("resources", "Resources", GroupRuntime,
			[]string{"Container", "Req CPU", "Req Mem", "Lim CPU", "Lim Mem"}, rows))
	}
	if rows := volumeRows(pod.Spec.Volumes); len(rows) > 0 {
		sections = append(sections, sectionTable("volumes", "Volumes", GroupSpec,
			[]string{"Name", "Source"}, rows))
	}
	if rows := containerRows(pod.Spec.InitContainers); len(rows) > 0 {
		sections = append(sections, sectionTable("initContainers", "Init Containers", GroupRuntime,
			[]string{"Name", "Image", "Req CPU", "Req Mem", "Lim CPU", "Lim Mem", "Ports"}, rows))
	}
	if rows := sidecarRows(pod.Spec.Containers); len(rows) > 0 {
		sections = append(sections, sectionTable("sidecars", "Sidecars", GroupRuntime,
			[]string{"Name", "Image", "Restart Policy"}, rows))
	}
	if rows := envRows(append(pod.Spec.InitContainers, pod.Spec.Containers...)); len(rows) > 0 {
		sections = append(sections, sectionTable("environment", "Environment", GroupSpec,
			[]string{"Container", "Name", "Value"}, rows))
	}
	if rows := imagePullSecretRows(pod.Spec.ImagePullSecrets); len(rows) > 0 {
		sections = append(sections, sectionTable("imagePullSecrets", "Image Pull Secrets", GroupRelationships,
			[]string{"Name"}, rows))
	}

	sections = append(sections, metaSections(pod.Labels, pod.Annotations, ownerRefsFromMeta(pod.OwnerReferences, pod.Namespace))...)
	if mf := managedFieldsSection(pod.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	d.Sections = sections
	return d, nil
}

func containerStateRows(sts []corev1.ContainerStatus) [][]string {
	var rows [][]string
	for _, s := range sts {
		state, reason, exit := describeContainerState(s.State)
		rows = append(rows, []string{
			s.Name, boolStr(s.Ready), fmtInt32(s.RestartCount), state, reason, exit,
		})
	}
	return rows
}

func restartHistoryRows(sts []corev1.ContainerStatus) [][]string {
	var rows [][]string
	for _, s := range sts {
		if s.RestartCount == 0 && s.LastTerminationState.Terminated == nil {
			continue
		}
		state, reason, exit := describeContainerState(s.LastTerminationState)
		rows = append(rows, []string{s.Name, fmtInt32(s.RestartCount), state, reason, exit})
	}
	return rows
}

func describeContainerState(st corev1.ContainerState) (state, reason, exit string) {
	switch {
	case st.Running != nil:
		return "running", "", ""
	case st.Waiting != nil:
		return "waiting", st.Waiting.Reason, ""
	case st.Terminated != nil:
		return "terminated", st.Terminated.Reason, fmtInt32(st.Terminated.ExitCode)
	default:
		return "", "", ""
	}
}

func containerResourceRows(pod *corev1.Pod) [][]string {
	var rows [][]string
	for _, c := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		reqCPU, reqMem, limCPU, limMem := resourceStrings(c.Resources)
		if reqCPU == "" && reqMem == "" && limCPU == "" && limMem == "" {
			continue
		}
		rows = append(rows, []string{c.Name, reqCPU, reqMem, limCPU, limMem})
	}
	return rows
}

func sidecarRows(containers []corev1.Container) [][]string {
	var rows [][]string
	for _, c := range containers {
		if c.RestartPolicy == nil || *c.RestartPolicy != corev1.ContainerRestartPolicyAlways {
			continue
		}
		rows = append(rows, []string{c.Name, c.Image, string(*c.RestartPolicy)})
	}
	return rows
}

func imagePullSecretRows(refs []corev1.LocalObjectReference) [][]string {
	var rows [][]string
	for _, r := range refs {
		rows = append(rows, []string{r.Name})
	}
	return rows
}

func fmtTime(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

func fmtPriority(p *int32) string {
	if p == nil {
		return ""
	}
	return fmtInt32(*p)
}
