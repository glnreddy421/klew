package details

import (
	"context"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
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

	allContainers := append(append([]corev1.Container{}, pod.Spec.InitContainers...), pod.Spec.Containers...)
	configResolver := podConfigMapResolver(ctx, req, pod.Namespace)
	secretResolver := podSecretResolver(ctx, req, pod.Namespace)

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
	if rows := initContainerStateRows(pod.Status.InitContainerStatuses); len(rows) > 0 {
		sections = append(sections, sectionTable("initContainerStates", "Init Container States", GroupStatus,
			[]string{"Name", "Ready", "Restarts", "State", "Reason", "Exit", "Image"}, rows))
	}
	if rows := restartHistoryRows(pod.Status.ContainerStatuses); len(rows) > 0 {
		sections = append(sections, sectionTable("restartHistory", "Restart History", GroupRuntime,
			[]string{"Name", "Restarts", "Last State", "Last Reason", "Last Exit"}, rows))
	}
	if rows := podConditionRows(pod.Status.Conditions); len(rows) > 0 {
		sections = append(sections, sectionTable("conditions", "Conditions", GroupStatus,
			[]string{"Type", "Status", "Reason", "Message"}, rows))
	}

	sections = append(sections, sectionFields("podSpec", "Pod Spec", GroupSpec, fields(
		"Restart Policy", string(pod.Spec.RestartPolicy),
		"DNS Policy", string(pod.Spec.DNSPolicy),
		"Scheduler", pod.Spec.SchedulerName,
		"Host Network", boolStr(pod.Spec.HostNetwork),
		"Termination Grace", fmtOptionalInt64(pod.Spec.TerminationGracePeriodSeconds),
		"Automount SA Token", boolPtrStr(pod.Spec.AutomountServiceAccountToken),
	)))

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

	if rows := containerRows(pod.Spec.Containers); len(rows) > 0 {
		sections = append(sections, sectionTable("containers", "Containers", GroupSpec,
			[]string{"Name", "Image", "Req CPU", "Req Mem", "Lim CPU", "Lim Mem", "Ports"}, rows))
	}
	if rows := containerSpecRows(allContainers); len(rows) > 0 {
		sections = append(sections, sectionTable("containerSpec", "Container Spec", GroupSpec,
			[]string{"Name", "Image", "Pull Policy", "Command", "Args", "Working Dir"}, rows))
	}
	if rows := containerResourceRows(pod); len(rows) > 0 {
		sections = append(sections, sectionTable("resources", "Resources", GroupRuntime,
			[]string{"Container", "Req CPU", "Req Mem", "Lim CPU", "Lim Mem"}, rows))
	}
	if rows := volumeRows(pod.Spec.Volumes); len(rows) > 0 {
		sections = append(sections, sectionTable("volumes", "Volumes", GroupSpec,
			[]string{"Name", "Source"}, rows))
	}
	if rows := volumeMountRows(allContainers); len(rows) > 0 {
		sections = append(sections, sectionTable("volumeMounts", "Volume Mounts", GroupSpec,
			[]string{"Container", "Volume", "Mount Path", "Sub Path", "Read Only"}, rows))
	}
	if rows := probeRows(allContainers); len(rows) > 0 {
		sections = append(sections, sectionTable("probes", "Probes", GroupSpec,
			[]string{"Container", "Probe", "Type", "Target", "Initial Delay", "Period", "Timeout", "Failures"}, rows))
	}
	if rows := securityContextRows(allContainers); len(rows) > 0 {
		sections = append(sections, sectionTable("securityContext", "Security Context", GroupSpec,
			[]string{"Container", "Run As User", "Run As Group", "Privileged", "Read Only Root FS"}, rows))
	}
	if rows := containerRows(pod.Spec.InitContainers); len(rows) > 0 {
		sections = append(sections, sectionTable("initContainers", "Init Containers", GroupRuntime,
			[]string{"Name", "Image", "Req CPU", "Req Mem", "Lim CPU", "Lim Mem", "Ports"}, rows))
	}
	if rows := sidecarRows(pod.Spec.Containers); len(rows) > 0 {
		sections = append(sections, sectionTable("sidecars", "Sidecars", GroupRuntime,
			[]string{"Name", "Image", "Restart Policy"}, rows))
	}
	if rows := envRowsDetailed(allContainers, configResolver); len(rows) > 0 {
		sections = append(sections, sectionTable("environment", "Environment", GroupSpec,
			[]string{"Container", "Name", "Source", "Value"}, rows))
	}
	if rows, notes := resolvedSecretEnvRows(allContainers, secretResolver); len(rows) > 0 {
		sec := sectionSensitiveTable("resolvedSecretEnv", "Resolved Secret Environment", GroupSpec,
			[]string{"Container", "Name", "Secret Key", "Value"}, rows, 3)
		sec.Notes = notes
		sections = append(sections, sec)
	} else if len(notes) > 0 {
		sections = append(sections, Section{
			ID:    "resolvedSecretEnv",
			Title: "Secret Environment",
			Group: GroupSpec,
			Notes: notes,
		})
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

func podConfigMapResolver(ctx context.Context, req *Request, ns string) envValueResolver {
	cache := map[string]*corev1.ConfigMap{}
	return func(sourceKind, name, key string) (string, bool) {
		if sourceKind != "ConfigMap" || name == "" || key == "" {
			return "", false
		}
		cm, ok := cache[name]
		if !ok {
			fetched, err := fetchConfigMap(ctx, req, ns, name)
			if err != nil || fetched == nil {
				return "", false
			}
			cache[name] = fetched
			cm = fetched
		}
		raw, ok := cm.Data[key]
		if !ok {
			return "", false
		}
		return raw, true
	}
}

func podSecretResolver(ctx context.Context, req *Request, ns string) secretEnvResolver {
	cache := map[string]secretFetchResult{}
	return func(name, key string) secretResolveResult {
		if name == "" || key == "" {
			return secretResolveResult{}
		}
		entry, ok := cache[name]
		if !ok {
			sec, err := fetchSecret(ctx, req, ns, name)
			entry = secretFetchResult{secret: sec, err: err}
			cache[name] = entry
		}
		if entry.err != nil {
			if k8serrors.IsForbidden(entry.err) || k8serrors.IsUnauthorized(entry.err) {
				return secretResolveResult{forbidden: true}
			}
			return secretResolveResult{}
		}
		raw, ok := entry.secret.Data[key]
		if !ok {
			return secretResolveResult{}
		}
		return secretResolveResult{
			value: base64.StdEncoding.EncodeToString(raw),
			found: true,
		}
	}
}

type secretFetchResult struct {
	secret *corev1.Secret
	err    error
}

func fetchConfigMap(ctx context.Context, req *Request, ns, name string) (*corev1.ConfigMap, error) {
	sub := *req
	sub.Ref = model.ObjectRef{Kind: "ConfigMap", Name: name, Namespace: ns}
	return getConfigMap(ctx, &sub)
}

func fetchSecret(ctx context.Context, req *Request, ns, name string) (*corev1.Secret, error) {
	sub := *req
	sub.Ref = model.ObjectRef{Kind: "Secret", Name: name, Namespace: ns}
	return getSecret(ctx, &sub)
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

func boolPtrStr(p *bool) string {
	if p == nil {
		return ""
	}
	return boolStr(*p)
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
