package details

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type replicationControllerProvider struct{}

func (replicationControllerProvider) Kind() string { return "ReplicationController" }

func (replicationControllerProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	rc, err := getReplicationController(ctx, req)
	if err != nil {
		return nil, err
	}
	desired := int32Or(rc.Spec.Replicas, 1)
	ready := rc.Status.ReadyReplicas >= desired
	detail := &ObjectDetail{
		Title:    "ReplicationController/" + rc.Name,
		Category: "workload",
		Status:   replicaStatus(rc.Status.ReadyReplicas, desired, ready),
		Summary: fields(
			"Replicas", fmt.Sprintf("%d/%d", rc.Status.ReadyReplicas, desired),
			"Available", fmtInt32(rc.Status.AvailableReplicas),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupSummary, fields(
		"Replicas", fmtInt32(rc.Status.Replicas),
		"Ready", fmtInt32(rc.Status.ReadyReplicas),
		"Available", fmtInt32(rc.Status.AvailableReplicas),
		"Fully Labeled", fmtInt32(rc.Status.FullyLabeledReplicas),
		"Observed Generation", fmtInt64(rc.Status.ObservedGeneration),
	)))
	sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
		"Min Ready Seconds", fmtInt32(rc.Spec.MinReadySeconds),
		"Replicas", fmtInt32(int32Or(rc.Spec.Replicas, 1)),
	)))
	if rc.Spec.Selector != nil {
		if sel := selectorString(rc.Spec.Selector); sel != "" {
			sections = append(sections, sectionFields("selector", "Selector", GroupSpec, fields("Match Labels", sel)))
		}
	}
	sections = append(sections, podTemplateSections(rc.Spec.Template, GroupSpec)...)
	sections = append(sections, metaSections(rc.Labels, rc.Annotations, ownerRefsFromMeta(rc.OwnerReferences, rc.Namespace))...)
	if mf := managedFieldsSection(rc.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

type ingressClassProvider struct{}

func (ingressClassProvider) Kind() string { return "IngressClass" }

func (ingressClassProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	ic, err := getIngressClass(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "IngressClass/" + ic.Name,
		Category: "network",
		Status:   StatusBadge{Tone: "healthy", Label: "Available"},
		Summary: fields(
			"Controller", ic.Spec.Controller,
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
		"Controller", ic.Spec.Controller,
	)))
	if ic.Spec.Parameters != nil {
		sections = append(sections, sectionFields("parameters", "Parameters", GroupSpec, fields(
			"API Group", stringPtr(ic.Spec.Parameters.APIGroup),
			"Kind", ic.Spec.Parameters.Kind,
			"Scope", stringPtr(ic.Spec.Parameters.Scope),
			"Namespace", stringPtr(ic.Spec.Parameters.Namespace),
		)))
	}
	sections = append(sections, metaSections(ic.Labels, ic.Annotations, ownerRefsFromMeta(ic.OwnerReferences, ""))...)
	if mf := managedFieldsSection(ic.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

type pdbProvider struct{}

func (pdbProvider) Kind() string { return "PodDisruptionBudget" }

func (pdbProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	pdb, err := getPodDisruptionBudget(ctx, req)
	if err != nil {
		return nil, err
	}
	minAvail := ""
	if pdb.Spec.MinAvailable != nil {
		minAvail = pdb.Spec.MinAvailable.String()
	}
	maxUnavail := ""
	if pdb.Spec.MaxUnavailable != nil {
		maxUnavail = pdb.Spec.MaxUnavailable.String()
	}
	detail := &ObjectDetail{
		Title:    "PodDisruptionBudget/" + pdb.Name,
		Category: "workload",
		Status:   StatusBadge{Tone: "healthy", Label: fmt.Sprintf("%d allowed", pdb.Status.DisruptionsAllowed)},
		Summary: fields(
			"Min Available", minAvail,
			"Max Unavailable", maxUnavail,
			"Disruptions Allowed", fmtInt32(pdb.Status.DisruptionsAllowed),
			"Current Healthy", fmtInt32(pdb.Status.CurrentHealthy),
		),
	}
	var sections []Section
	evictionPolicy := ""
	if pdb.Spec.UnhealthyPodEvictionPolicy != nil {
		evictionPolicy = string(*pdb.Spec.UnhealthyPodEvictionPolicy)
	}
	sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
		"Min Available", minAvail,
		"Max Unavailable", maxUnavail,
		"Selector", labelSelectorSummary(pdb.Spec.Selector),
		"Unhealthy Pod Eviction Policy", evictionPolicy,
	)))
	sections = append(sections, sectionFields("status", "Status", GroupSummary, fields(
		"Disruptions Allowed", fmtInt32(pdb.Status.DisruptionsAllowed),
		"Current Healthy", fmtInt32(pdb.Status.CurrentHealthy),
		"Desired Healthy", fmtInt32(pdb.Status.DesiredHealthy),
		"Expected Pods", fmtInt32(pdb.Status.ExpectedPods),
		"Observed Generation", fmtInt64(pdb.Status.ObservedGeneration),
	)))
	if len(pdb.Status.Conditions) > 0 {
		var rows [][]string
		for _, c := range pdb.Status.Conditions {
			rows = append(rows, []string{string(c.Type), string(c.Status), c.Reason, truncate(c.Message, 120)})
		}
		sections = append(sections, sectionTable("conditions", "Conditions", GroupSummary,
			[]string{"Type", "Status", "Reason", "Message"}, rows))
	}
	sections = append(sections, metaSections(pdb.Labels, pdb.Annotations, ownerRefsFromMeta(pdb.OwnerReferences, pdb.Namespace))...)
	if mf := managedFieldsSection(pdb.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

type leaseProvider struct{}

func (leaseProvider) Kind() string { return "Lease" }

func (leaseProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	lease, err := getLease(ctx, req)
	if err != nil {
		return nil, err
	}
	holder := stringPtr(lease.Spec.HolderIdentity)
	detail := &ObjectDetail{
		Title:    "Lease/" + lease.Name,
		Category: "cluster",
		Status:   StatusBadge{Tone: "healthy", Label: holder},
		Summary: fields(
			"Holder", holder,
			"Renew Time", formatMicroTime(lease.Spec.RenewTime),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
		"Holder Identity", holder,
		"Lease Duration Seconds", fmtInt32Ptr(lease.Spec.LeaseDurationSeconds),
		"Acquire Time", formatMicroTime(lease.Spec.AcquireTime),
		"Renew Time", formatMicroTime(lease.Spec.RenewTime),
		"Lease Transitions", fmtInt32Ptr(lease.Spec.LeaseTransitions),
	)))
	sections = append(sections, metaSections(lease.Labels, lease.Annotations, ownerRefsFromMeta(lease.OwnerReferences, lease.Namespace))...)
	if mf := managedFieldsSection(lease.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

type resourceQuotaProvider struct{}

func (resourceQuotaProvider) Kind() string { return "ResourceQuota" }

func (resourceQuotaProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	rq, err := getResourceQuota(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "ResourceQuota/" + rq.Name,
		Category: "cluster",
		Status:   StatusBadge{Tone: "healthy", Label: "Active"},
		Summary:  fields("Hard Limits", fmtInt32(int32(len(rq.Spec.Hard)))),
	}
	var sections []Section
	if len(rq.Spec.Hard) > 0 {
		var rows [][]string
		for _, k := range sortedResourceNames(rq.Spec.Hard) {
			hard := quantityString(rq.Spec.Hard[k])
			used := ""
			if u, ok := rq.Status.Used[k]; ok {
				used = quantityString(u)
			}
			rows = append(rows, []string{string(k), used, hard})
		}
		sections = append(sections, sectionTable("hard", "Hard / Used", GroupSpec,
			[]string{"Resource", "Used", "Hard"}, rows))
	}
	if len(rq.Spec.Scopes) > 0 {
		scopes := make([]string, 0, len(rq.Spec.Scopes))
		for _, s := range rq.Spec.Scopes {
			scopes = append(scopes, string(s))
		}
		sections = append(sections, sectionFields("scopes", "Scopes", GroupSpec, fields(
			"Scopes", strings.Join(scopes, ", "),
		)))
	}
	sections = append(sections, metaSections(rq.Labels, rq.Annotations, ownerRefsFromMeta(rq.OwnerReferences, rq.Namespace))...)
	if mf := managedFieldsSection(rq.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

type limitRangeProvider struct{}

func (limitRangeProvider) Kind() string { return "LimitRange" }

func (limitRangeProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	lr, err := getLimitRange(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "LimitRange/" + lr.Name,
		Category: "cluster",
		Status:   StatusBadge{Tone: "healthy", Label: fmt.Sprintf("%d limits", len(lr.Spec.Limits))},
		Summary:  fields("Limits", fmtInt32(int32(len(lr.Spec.Limits)))),
	}
	var sections []Section
	if len(lr.Spec.Limits) > 0 {
		var rows [][]string
		for _, lim := range lr.Spec.Limits {
			rows = append(rows, []string{
				string(lim.Type),
				resourceListSummary(lim.Default),
				resourceListSummary(lim.DefaultRequest),
				resourceListSummary(lim.Max),
				resourceListSummary(lim.Min),
			})
		}
		sections = append(sections, sectionTable("limits", "Limits", GroupSpec,
			[]string{"Type", "Default", "Default Request", "Max", "Min"}, rows))
	}
	sections = append(sections, metaSections(lr.Labels, lr.Annotations, ownerRefsFromMeta(lr.OwnerReferences, lr.Namespace))...)
	if mf := managedFieldsSection(lr.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func labelSelectorSummary(sel *metav1.LabelSelector) string {
	if sel == nil {
		return ""
	}
	parts := make([]string, 0, len(sel.MatchLabels)+len(sel.MatchExpressions))
	if len(sel.MatchLabels) > 0 {
		parts = append(parts, selectorString(sel.MatchLabels))
	}
	for _, expr := range sel.MatchExpressions {
		parts = append(parts, fmt.Sprintf("%s %s %s", expr.Key, expr.Operator, strings.Join(expr.Values, ",")))
	}
	return strings.Join(parts, "; ")
}

func sortedResourceNames(m corev1.ResourceList) []corev1.ResourceName {
	names := make([]corev1.ResourceName, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}

func resourceListSummary(list corev1.ResourceList) string {
	if len(list) == 0 {
		return ""
	}
	parts := make([]string, 0, len(list))
	for _, k := range sortedResourceNames(list) {
		parts = append(parts, fmt.Sprintf("%s=%s", k, quantityString(list[k])))
	}
	return strings.Join(parts, ", ")
}

func quantityString(q resource.Quantity) string {
	return q.String()
}

func stringPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func fmtInt64Ptr(p *int64) string {
	if p == nil {
		return ""
	}
	return fmtInt64(*p)
}

func formatMicroTime(t *metav1.MicroTime) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Time.Format("2006-01-02 15:04:05 MST")
}
