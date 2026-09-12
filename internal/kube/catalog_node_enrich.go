package kube

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/glnreddy421/klew/internal/model"
)

func enrichNodeCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	roles := catalogNodeRoles(obj)
	version := stringFromObject(obj, "status", "nodeInfo", "kubeletVersion")
	ready := catalogNodeReadyBool(obj)
	taintCount := catalogNodeTaintCount(obj)
	taintsSummary := catalogNodeTaintsSummary(obj)

	res := &model.NodeCatalogResources{
		Roles:          roles,
		KubeletVersion: version,
		Ready:          ready,
		TaintsSummary:  taintsSummary,
	}
	if taintCount != nil {
		res.TaintCount = taintCount
	}
	if q := quantityFromNested(obj, "status", "capacity", "cpu"); q != nil {
		milli := q.MilliValue()
		res.CapacityCPUMilli = &milli
	}
	if q := quantityFromNested(obj, "status", "allocatable", "cpu"); q != nil {
		milli := q.MilliValue()
		res.AllocatableCPUMilli = &milli
	}
	if q := quantityFromNested(obj, "status", "capacity", "memory"); q != nil {
		bytes := q.Value()
		res.CapacityMemoryBytes = &bytes
	}
	if q := quantityFromNested(obj, "status", "allocatable", "memory"); q != nil {
		bytes := q.Value()
		res.AllocatableMemoryBytes = &bytes
	}
	if q := quantityFromNested(obj, "status", "capacity", "ephemeral-storage"); q != nil {
		bytes := q.Value()
		res.CapacityDiskBytes = &bytes
	}
	if q := quantityFromNested(obj, "status", "allocatable", "ephemeral-storage"); q != nil {
		bytes := q.Value()
		res.AllocatableDiskBytes = &bytes
	}
	entity.NodeResources = res

	if ready != nil {
		if *ready {
			entity.StatusHint = "Ready"
		} else {
			entity.StatusHint = "NotReady"
		}
	}
}

func catalogNodeReadyBool(obj map[string]interface{}) *bool {
	conds, _, _ := unstructured.NestedSlice(obj, "status", "conditions")
	for _, item := range conds {
		cm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if stringFromMap(cm, "type") == "Ready" {
			v := stringFromMap(cm, "status") == "True"
			return &v
		}
	}
	return nil
}

func catalogNodeTaintCount(obj map[string]interface{}) *int32 {
	taints, found, _ := unstructured.NestedSlice(obj, "spec", "taints")
	if !found {
		zero := int32(0)
		return &zero
	}
	n := int32(len(taints))
	return &n
}

func catalogNodeTaintsSummary(obj map[string]interface{}) string {
	taints, found, _ := unstructured.NestedSlice(obj, "spec", "taints")
	if !found || len(taints) == 0 {
		return ""
	}
	parts := make([]string, 0, len(taints))
	for _, item := range taints {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _, _ := unstructured.NestedString(m, "key")
		value, _, _ := unstructured.NestedString(m, "value")
		effect, _, _ := unstructured.NestedString(m, "effect")
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
	if len(summary) > 160 {
		return summary[:157] + "…"
	}
	return summary
}

func quantityFromNested(obj map[string]interface{}, fields ...string) *resource.Quantity {
	raw, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil || raw == nil {
		return nil
	}
	var text string
	switch v := raw.(type) {
	case string:
		text = v
	default:
		text = stringFromObject(obj, fields...)
	}
	if text == "" {
		return nil
	}
	q, err := resource.ParseQuantity(text)
	if err != nil {
		return nil
	}
	return &q
}
