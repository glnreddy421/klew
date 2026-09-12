package kube

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/glnreddy421/klew/internal/model"
)

func applyCatalogScheduling(entity *model.CatalogEntity, fields map[string]string, spec map[string]interface{}) {
	if entity == nil {
		return
	}
	sched := buildCatalogScheduling(spec)
	if sched == nil {
		return
	}
	entity.Scheduling = sched
	if ns := schedulingNodeSelectorLabel(sched); ns != "" {
		setField(fields, "nodeSelector", ns)
	}
	if n := len(sched.Tolerations); n > 0 {
		setField(fields, "tolerations", schedulingTolerationsLabel(sched))
		setField(fields, "tolerationsCount", fmt.Sprintf("%d", n))
	}
	if aff := schedulingAffinityLabel(sched); aff != "" {
		setField(fields, "affinity", aff)
	}
}

func applyCatalogNodeTaints(entity *model.CatalogEntity, fields map[string]string, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	taints := catalogTaintRows(obj)
	if len(taints) == 0 {
		return
	}
	if entity.Scheduling == nil {
		entity.Scheduling = &model.CatalogScheduling{}
	}
	entity.Scheduling.Taints = taints
	setField(fields, "taints", schedulingTaintsLabel(taints))
}

func buildCatalogScheduling(spec map[string]interface{}) *model.CatalogScheduling {
	if len(spec) == 0 {
		return nil
	}
	sched := &model.CatalogScheduling{
		NodeSelector: catalogNodeSelectorRows(spec),
		Tolerations:  catalogTolerationRows(spec),
		Affinity:     catalogAffinityRows(spec),
	}
	if len(sched.NodeSelector) == 0 && len(sched.Tolerations) == 0 && len(sched.Affinity) == 0 {
		return nil
	}
	return sched
}

func catalogNodeSelectorRows(spec map[string]interface{}) []model.CatalogKVPair {
	raw, found, _ := unstructured.NestedStringMap(spec, "nodeSelector")
	if !found || len(raw) == 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sortStrings(keys)
	out := make([]model.CatalogKVPair, 0, len(keys))
	for _, k := range keys {
		out = append(out, model.CatalogKVPair{Key: k, Value: raw[k]})
	}
	return out
}

func catalogTolerationRows(spec map[string]interface{}) []model.CatalogTolerationRow {
	raw, found, _ := unstructured.NestedSlice(spec, "tolerations")
	if !found || len(raw) == 0 {
		return nil
	}
	out := make([]model.CatalogTolerationRow, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _, _ := unstructured.NestedString(m, "key")
		operator, _, _ := unstructured.NestedString(m, "operator")
		value, _, _ := unstructured.NestedString(m, "value")
		effect, _, _ := unstructured.NestedString(m, "effect")
		seconds, _, _ := unstructured.NestedInt64(m, "tolerationSeconds")
		sec := ""
		if seconds > 0 {
			sec = fmt.Sprintf("%d", seconds)
		}
		out = append(out, model.CatalogTolerationRow{
			Key:      key,
			Operator: operator,
			Value:    value,
			Effect:   effect,
			Seconds:  sec,
		})
	}
	return out
}

func catalogTaintRows(obj map[string]interface{}) []model.CatalogTaintRow {
	raw, found, _ := unstructured.NestedSlice(obj, "spec", "taints")
	if !found || len(raw) == 0 {
		return nil
	}
	out := make([]model.CatalogTaintRow, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _, _ := unstructured.NestedString(m, "key")
		value, _, _ := unstructured.NestedString(m, "value")
		effect, _, _ := unstructured.NestedString(m, "effect")
		out = append(out, model.CatalogTaintRow{
			Key:    key,
			Value:  value,
			Effect: effect,
		})
	}
	return out
}

func catalogAffinityRows(spec map[string]interface{}) []model.CatalogAffinityRuleRow {
	aff, found, _ := unstructured.NestedMap(spec, "affinity")
	if !found || len(aff) == 0 {
		return nil
	}
	var out []model.CatalogAffinityRuleRow
	if nodeAff, ok := aff["nodeAffinity"].(map[string]interface{}); ok {
		if req, ok := nodeAff["requiredDuringSchedulingIgnoredDuringExecution"].(map[string]interface{}); ok {
			for _, term := range catalogNodeSelectorTerms(req) {
				out = append(out, model.CatalogAffinityRuleRow{
					Type:  "Node required",
					Match: term,
				})
			}
		}
		if pref, ok := nodeAff["preferredDuringSchedulingIgnoredDuringExecution"].([]interface{}); ok {
			for _, item := range pref {
				pm, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				weight, _, _ := unstructured.NestedInt64(pm, "weight")
				prefMap, _, _ := unstructured.NestedMap(pm, "preference")
				for _, term := range catalogNodeSelectorTerms(prefMap) {
					out = append(out, model.CatalogAffinityRuleRow{
						Type:   "Node preferred",
						Weight: fmt.Sprintf("%d", weight),
						Match:  term,
					})
				}
			}
		}
	}
	if podAff, ok := aff["podAffinity"].(map[string]interface{}); ok {
		out = append(out, catalogPodAffinityRuleRows("Pod required", "Pod preferred", podAff)...)
	}
	if podAff, ok := aff["podAntiAffinity"].(map[string]interface{}); ok {
		out = append(out, catalogPodAffinityRuleRows("Pod anti required", "Pod anti preferred", podAff)...)
	}
	return out
}

func catalogPodAffinityRuleRows(requiredLabel, preferredLabel string, aff map[string]interface{}) []model.CatalogAffinityRuleRow {
	var out []model.CatalogAffinityRuleRow
	if raw, found, _ := unstructured.NestedSlice(aff, "requiredDuringSchedulingIgnoredDuringExecution"); found {
		for _, item := range raw {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			out = append(out, catalogPodAffinityRuleRow(requiredLabel, 0, m))
		}
	}
	if raw, found, _ := unstructured.NestedSlice(aff, "preferredDuringSchedulingIgnoredDuringExecution"); found {
		for _, item := range raw {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			weight, _, _ := unstructured.NestedInt64(m, "weight")
			term, _, _ := unstructured.NestedMap(m, "podAffinityTerm")
			out = append(out, catalogPodAffinityRuleRow(preferredLabel, weight, term))
		}
	}
	return out
}

func catalogPodAffinityRuleRow(label string, weight int64, term map[string]interface{}) model.CatalogAffinityRuleRow {
	topology, _, _ := unstructured.NestedString(term, "topologyKey")
	ns, _, _ := unstructured.NestedStringSlice(term, "namespaces")
	match := catalogLabelSelectorSummary(term)
	return model.CatalogAffinityRuleRow{
		Type:       label,
		Weight:     weightLabel(weight),
		Topology:   topology,
		Namespaces: strings.Join(ns, ","),
		Match:      match,
	}
}

func weightLabel(weight int64) string {
	if weight <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", weight)
}

func schedulingNodeSelectorLabel(sched *model.CatalogScheduling) string {
	if sched == nil || len(sched.NodeSelector) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sched.NodeSelector))
	for _, kv := range sched.NodeSelector {
		parts = append(parts, kv.Key+"="+kv.Value)
	}
	return truncateSchedulingLabel(strings.Join(parts, ", "))
}

func schedulingTolerationsLabel(sched *model.CatalogScheduling) string {
	if sched == nil || len(sched.Tolerations) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sched.Tolerations))
	for _, t := range sched.Tolerations {
		part := t.Key
		if t.Value != "" {
			part += "=" + t.Value
		}
		if t.Effect != "" {
			part += ":" + t.Effect
		}
		parts = append(parts, part)
	}
	return truncateSchedulingLabel(strings.Join(parts, ", "))
}

func schedulingTaintsLabel(taints []model.CatalogTaintRow) string {
	if len(taints) == 0 {
		return ""
	}
	parts := make([]string, 0, len(taints))
	for _, t := range taints {
		part := t.Key
		if t.Value != "" {
			part += "=" + t.Value
		}
		if t.Effect != "" {
			part += ":" + t.Effect
		}
		parts = append(parts, part)
	}
	return truncateSchedulingLabel(strings.Join(parts, ", "))
}

func schedulingAffinityLabel(sched *model.CatalogScheduling) string {
	if sched == nil || len(sched.Affinity) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sched.Affinity))
	for _, rule := range sched.Affinity {
		var bits []string
		if rule.Topology != "" {
			bits = append(bits, rule.Topology)
		}
		if rule.Match != "" {
			bits = append(bits, rule.Match)
		}
		line := rule.Type
		if rule.Weight != "" {
			line += "(" + rule.Weight + ")"
		}
		if len(bits) > 0 {
			line += ": " + strings.Join(bits, " ")
		}
		parts = append(parts, line)
	}
	return truncateSchedulingLabel(strings.Join(parts, "; "))
}

func truncateSchedulingLabel(s string) string {
	if len(s) <= 120 {
		return s
	}
	return s[:117] + "…"
}
