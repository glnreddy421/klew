package details

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
)

type nodeProvider struct{}

func (nodeProvider) Kind() string { return "Node" }

func (nodeProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	node, err := getNode(ctx, req)
	if err != nil {
		return nil, err
	}
	ready := false
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}
	tone, label := "healthy", "Ready"
	if !ready {
		tone, label = "critical", "NotReady"
	}
	detail := &ObjectDetail{
		Title:    "Node/" + node.Name,
		Category: "cluster",
		Status:   StatusBadge{Tone: tone, Label: label},
		Summary: fields(
			"OS", node.Status.NodeInfo.OperatingSystem,
			"Kernel", node.Status.NodeInfo.KernelVersion,
			"Kubelet", node.Status.NodeInfo.KubeletVersion,
			"Runtime", node.Status.NodeInfo.ContainerRuntimeVersion,
		),
	}
	var sections []Section
	if rows := nodeConditionRows(node.Status.Conditions); len(rows) > 0 {
		sections = append(sections, sectionTable("conditions", "Conditions", GroupStatus,
			[]string{"Type", "Status", "Reason", "Message"}, rows))
	}
	if rows := resourceListRows(node.Status.Capacity); len(rows) > 0 {
		sections = append(sections, sectionTable("capacity", "Capacity", GroupRuntime,
			[]string{"Resource", "Quantity"}, rows))
	}
	if rows := resourceListRows(node.Status.Allocatable); len(rows) > 0 {
		sections = append(sections, sectionTable("allocatable", "Allocatable", GroupRuntime,
			[]string{"Resource", "Quantity"}, rows))
	}
	if rows := nodeAddressRows(node.Status.Addresses); len(rows) > 0 {
		sections = append(sections, sectionTable("addresses", "Addresses", GroupStatus,
			[]string{"Type", "Address"}, rows))
	}
	var podRows [][]string
	for _, p := range req.Snapshot.Pods {
		if p.Node == node.Name {
			podRows = append(podRows, []string{p.Namespace, p.Name, p.Phase, boolStr(p.Ready)})
		}
	}
	if len(podRows) > 0 {
		sections = append(sections, sectionTable("podsScheduled", "Pods Scheduled", GroupRelationships,
			[]string{"Namespace", "Name", "Phase", "Ready"}, podRows))
	}
	if rows := taintRows(node.Spec.Taints); len(rows) > 0 {
		sections = append(sections, sectionTable("taints", "Taints", GroupSpec,
			[]string{"Key", "Value", "Effect"}, rows))
	}
	sections = append(sections, sectionFields("runtime", "Runtime", GroupRuntime, fields(
		"Architecture", node.Status.NodeInfo.Architecture,
		"OS Image", node.Status.NodeInfo.OSImage,
		"Operating System", node.Status.NodeInfo.OperatingSystem,
		"Kernel", node.Status.NodeInfo.KernelVersion,
		"Kubelet", node.Status.NodeInfo.KubeletVersion,
		"Kube-Proxy", node.Status.NodeInfo.KubeProxyVersion,
		"Container Runtime", node.Status.NodeInfo.ContainerRuntimeVersion,
		"Machine ID", node.Status.NodeInfo.MachineID,
		"System UUID", node.Status.NodeInfo.SystemUUID,
		"Boot ID", node.Status.NodeInfo.BootID,
	)))
	sections = append(sections, metaSections(node.Labels, node.Annotations, nil)...)
	if mf := managedFieldsSection(node.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func nodeConditionRows(conds []corev1.NodeCondition) [][]string {
	var rows [][]string
	for _, c := range conds {
		rows = append(rows, []string{string(c.Type), string(c.Status), c.Reason, truncate(c.Message, 120)})
	}
	return rows
}

func resourceListRows(rl corev1.ResourceList) [][]string {
	keys := make([]string, 0, len(rl))
	for k := range rl {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	var rows [][]string
	for _, k := range keys {
		q := rl[corev1.ResourceName(k)]
		rows = append(rows, []string{k, (&q).String()})
	}
	return rows
}

func nodeAddressRows(addrs []corev1.NodeAddress) [][]string {
	var rows [][]string
	for _, a := range addrs {
		rows = append(rows, []string{string(a.Type), a.Address})
	}
	return rows
}

func taintRows(taints []corev1.Taint) [][]string {
	var rows [][]string
	for _, t := range taints {
		rows = append(rows, []string{t.Key, t.Value, string(t.Effect)})
	}
	return rows
}
