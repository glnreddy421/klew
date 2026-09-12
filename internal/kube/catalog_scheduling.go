package kube

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/glnreddy421/klew/internal/model"
)

func catalogTableScheduling(entity *model.CatalogEntity, fields map[string]string, spec map[string]interface{}) {
	applyCatalogScheduling(entity, fields, spec)
}

func podSpecMap(obj map[string]interface{}, resource string) map[string]interface{} {
	if obj == nil {
		return nil
	}
	switch resource {
	case "pods":
		spec, _, _ := unstructured.NestedMap(obj, "spec")
		return spec
	case "deployments", "statefulsets", "replicasets", "replicationcontrollers", "daemonsets":
		spec, _, _ := unstructured.NestedMap(obj, "spec", "template", "spec")
		return spec
	case "jobs":
		spec, _, _ := unstructured.NestedMap(obj, "spec", "template", "spec")
		return spec
	case "cronjobs":
		spec, _, _ := unstructured.NestedMap(obj, "spec", "jobTemplate", "spec", "template", "spec")
		return spec
	default:
		return nil
	}
}

func catalogNodeSelectorString(spec map[string]interface{}) string {
	raw, found, _ := unstructured.NestedStringMap(spec, "nodeSelector")
	if !found || len(raw) == 0 {
		return ""
	}
	parts := make([]string, 0, len(raw))
	for k, v := range raw {
		parts = append(parts, k+"="+v)
	}
	sortStrings(parts)
	return strings.Join(parts, ",")
}

func catalogTolerationsSummary(spec map[string]interface{}) (int, string) {
	raw, found, _ := unstructured.NestedSlice(spec, "tolerations")
	if !found || len(raw) == 0 {
		return 0, ""
	}
	parts := make([]string, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _, _ := unstructured.NestedString(m, "key")
		effect, _, _ := unstructured.NestedString(m, "effect")
		value, _, _ := unstructured.NestedString(m, "value")
		part := key
		if value != "" {
			part += "=" + value
		}
		if effect != "" {
			part += ":" + effect
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	sortStrings(parts)
	summary := strings.Join(parts, ", ")
	if len(summary) > 120 {
		summary = summary[:117] + "…"
	}
	return len(raw), summary
}

func catalogAffinitySummary(spec map[string]interface{}) string {
	aff, found, _ := unstructured.NestedMap(spec, "affinity")
	if !found || len(aff) == 0 {
		return ""
	}
	var parts []string
	if nodeAff, ok := aff["nodeAffinity"].(map[string]interface{}); ok {
		if req, ok := nodeAff["requiredDuringSchedulingIgnoredDuringExecution"].(map[string]interface{}); ok {
			for _, term := range catalogNodeSelectorTerms(req) {
				parts = append(parts, "node required: "+term)
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
					parts = append(parts, fmt.Sprintf("node preferred(%d): %s", weight, term))
				}
			}
		}
	}
	if podAff, ok := aff["podAffinity"].(map[string]interface{}); ok {
		parts = append(parts, catalogPodAffinityParts("pod required", podAff, "requiredDuringSchedulingIgnoredDuringExecution")...)
		parts = append(parts, catalogPodAffinityParts("pod preferred", podAff, "preferredDuringSchedulingIgnoredDuringExecution")...)
	}
	if podAff, ok := aff["podAntiAffinity"].(map[string]interface{}); ok {
		parts = append(parts, catalogPodAffinityParts("pod anti required", podAff, "requiredDuringSchedulingIgnoredDuringExecution")...)
		parts = append(parts, catalogPodAffinityParts("pod anti preferred", podAff, "preferredDuringSchedulingIgnoredDuringExecution")...)
	}
	summary := strings.Join(parts, "; ")
	if len(summary) > 160 {
		return summary[:157] + "…"
	}
	return summary
}

func catalogNodeSelectorTerms(termMap map[string]interface{}) []string {
	if len(termMap) == 0 {
		return nil
	}
	raw, _, _ := unstructured.NestedSlice(termMap, "nodeSelectorTerms")
	if len(raw) == 0 {
		return nil
	}
	var out []string
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if s := catalogMatchRequirements(m); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func catalogPodAffinityParts(prefix string, aff map[string]interface{}, field string) []string {
	raw, found, _ := unstructured.NestedSlice(aff, field)
	if !found || len(raw) == 0 {
		return nil
	}
	var out []string
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		label := prefix
		if field == "preferredDuringSchedulingIgnoredDuringExecution" {
			if w, _, _ := unstructured.NestedInt64(m, "weight"); w > 0 {
				label = fmt.Sprintf("%s(%d)", prefix, w)
			}
			m, _, _ = unstructured.NestedMap(m, "podAffinityTerm")
		}
		topology, _, _ := unstructured.NestedString(m, "topologyKey")
		match := catalogLabelSelectorSummary(m)
		var bits []string
		if topology != "" {
			bits = append(bits, topology)
		}
		if match != "" {
			bits = append(bits, match)
		}
		if len(bits) == 0 {
			out = append(out, label)
			continue
		}
		out = append(out, label+": "+strings.Join(bits, " "))
	}
	return out
}

func catalogMatchRequirements(term map[string]interface{}) string {
	var parts []string
	for _, field := range []string{"matchExpressions", "matchFields"} {
		raw, _, _ := unstructured.NestedSlice(term, field)
		for _, item := range raw {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			key, _, _ := unstructured.NestedString(m, "key")
			op, _, _ := unstructured.NestedString(m, "operator")
			values, _, _ := unstructured.NestedStringSlice(m, "values")
			if key == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s %s [%s]", key, op, strings.Join(values, ",")))
		}
	}
	return strings.Join(parts, ", ")
}

func catalogLabelSelectorSummary(term map[string]interface{}) string {
	sel, _, _ := unstructured.NestedMap(term, "labelSelector")
	if len(sel) == 0 {
		return ""
	}
	labels, _, _ := unstructured.NestedStringMap(sel, "matchLabels")
	if len(labels) > 0 {
		parts := make([]string, 0, len(labels))
		for k, v := range labels {
			parts = append(parts, k+"="+v)
		}
		sortStrings(parts)
		return strings.Join(parts, ",")
	}
	return catalogMatchRequirements(sel)
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		j := i
		for j > 0 && values[j-1] > values[j] {
			values[j-1], values[j] = values[j], values[j-1]
			j--
		}
	}
}
