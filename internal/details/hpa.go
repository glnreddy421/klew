package details

import (
	"context"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

type hpaProvider struct{}

func (hpaProvider) Kind() string { return "HorizontalPodAutoscaler" }

func (hpaProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	hpa, err := getHPA(ctx, req)
	if err != nil {
		return nil, err
	}
	tone := "healthy"
	if hpa.Status.DesiredReplicas > hpa.Status.CurrentReplicas {
		tone = "warning"
	}
	detail := &ObjectDetail{
		Title:    "HorizontalPodAutoscaler/" + hpa.Name,
		Category: "autoscaling",
		Status:   StatusBadge{Tone: tone, Label: fmt.Sprintf("%d→%d", hpa.Status.CurrentReplicas, hpa.Status.DesiredReplicas)},
		Summary: fields(
			"Current", fmtInt32(hpa.Status.CurrentReplicas),
			"Desired", fmtInt32(hpa.Status.DesiredReplicas),
			"Min", fmtInt32Ptr(hpa.Spec.MinReplicas),
			"Max", fmtInt32(hpa.Spec.MaxReplicas),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("currentReplicas", "Current Replicas", GroupStatus, fields(
		"Current", fmtInt32(hpa.Status.CurrentReplicas),
	)))
	sections = append(sections, sectionFields("desiredReplicas", "Desired Replicas", GroupStatus, fields(
		"Desired", fmtInt32(hpa.Status.DesiredReplicas),
		"Min", fmtInt32Ptr(hpa.Spec.MinReplicas),
		"Max", fmtInt32(hpa.Spec.MaxReplicas),
	)))
	if rows := hpaMetricRows(hpa.Spec.Metrics, hpa.Status.CurrentMetrics); len(rows) > 0 {
		sections = append(sections, sectionTable("targetMetrics", "Target Metrics", GroupSpec,
			[]string{"Type", "Name", "Target", "Current"}, rows))
	}
	if rows := hpaConditionRows(hpa.Status.Conditions); len(rows) > 0 {
		sections = append(sections, sectionTable("conditions", "Conditions", GroupStatus,
			[]string{"Type", "Status", "Reason", "Message"}, rows))
	}
	sections = append(sections, sectionFields("scaleTarget", "Scale Target", GroupRelationships, fields(
		"Kind", hpa.Spec.ScaleTargetRef.Kind,
		"Name", hpa.Spec.ScaleTargetRef.Name,
		"API Version", hpa.Spec.ScaleTargetRef.APIVersion,
	)))
	sections = append(sections, metaSections(hpa.Labels, hpa.Annotations, ownerRefsFromMeta(hpa.OwnerReferences, hpa.Namespace))...)
	if mf := managedFieldsSection(hpa.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func hpaConditionRows(conds []autoscalingv2.HorizontalPodAutoscalerCondition) [][]string {
	var rows [][]string
	for _, c := range conds {
		rows = append(rows, []string{string(c.Type), string(c.Status), c.Reason, truncate(c.Message, 120)})
	}
	return rows
}

func hpaMetricRows(spec []autoscalingv2.MetricSpec, current []autoscalingv2.MetricStatus) [][]string {
	curByType := map[string]string{}
	for _, c := range current {
		curByType[string(c.Type)] = metricStatusValue(c)
	}
	var rows [][]string
	for _, m := range spec {
		name, target := metricSpecInfo(m)
		rows = append(rows, []string{string(m.Type), name, target, curByType[string(m.Type)]})
	}
	return rows
}

func metricSpecInfo(m autoscalingv2.MetricSpec) (name, target string) {
	switch m.Type {
	case autoscalingv2.ResourceMetricSourceType:
		if m.Resource != nil {
			name = string(m.Resource.Name)
			target = metricTarget(m.Resource.Target)
		}
	case autoscalingv2.PodsMetricSourceType:
		if m.Pods != nil {
			name = m.Pods.Metric.Name
			target = metricTarget(m.Pods.Target)
		}
	case autoscalingv2.ObjectMetricSourceType:
		if m.Object != nil {
			name = m.Object.Metric.Name
			target = metricTarget(m.Object.Target)
		}
	case autoscalingv2.ExternalMetricSourceType:
		if m.External != nil {
			name = m.External.Metric.Name
			target = metricTarget(m.External.Target)
		}
	case autoscalingv2.ContainerResourceMetricSourceType:
		if m.ContainerResource != nil {
			name = m.ContainerResource.Container + "/" + string(m.ContainerResource.Name)
			target = metricTarget(m.ContainerResource.Target)
		}
	}
	return
}

func metricTarget(t autoscalingv2.MetricTarget) string {
	switch t.Type {
	case autoscalingv2.UtilizationMetricType:
		if t.AverageUtilization != nil {
			return fmt.Sprintf("%d%%", *t.AverageUtilization)
		}
	case autoscalingv2.ValueMetricType:
		if t.Value != nil {
			return t.Value.String()
		}
	case autoscalingv2.AverageValueMetricType:
		if t.AverageValue != nil {
			return "avg " + t.AverageValue.String()
		}
	}
	return ""
}

func metricStatusValue(c autoscalingv2.MetricStatus) string {
	switch c.Type {
	case autoscalingv2.ResourceMetricSourceType:
		if c.Resource != nil {
			if c.Resource.Current.AverageUtilization != nil {
				return fmt.Sprintf("%d%%", *c.Resource.Current.AverageUtilization)
			}
			if c.Resource.Current.AverageValue != nil {
				return c.Resource.Current.AverageValue.String()
			}
		}
	case autoscalingv2.PodsMetricSourceType:
		if c.Pods != nil && c.Pods.Current.AverageValue != nil {
			return c.Pods.Current.AverageValue.String()
		}
	case autoscalingv2.ContainerResourceMetricSourceType:
		if c.ContainerResource != nil && c.ContainerResource.Current.AverageUtilization != nil {
			return fmt.Sprintf("%d%%", *c.ContainerResource.Current.AverageUtilization)
		}
	}
	return ""
}
