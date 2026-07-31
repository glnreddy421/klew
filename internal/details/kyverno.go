package details

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type kyvernoProvider struct {
	kind string
}

func (p *kyvernoProvider) Kind() string { return p.kind }

func (p *kyvernoProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	namespaced := p.kind == "Policy"
	resource := "clusterpolicies"
	if namespaced {
		resource = "policies"
	}
	obj, err := getUnstructured(ctx, req, schema.GroupVersionResource{
		Group: "kyverno.io", Version: "v1", Resource: resource,
	}, namespaced)
	if err != nil {
		return nil, err
	}
	return buildKyvernoPolicy(obj, p.kind), nil
}

func buildKyvernoPolicy(obj *unstructured.Unstructured, kind string) *ObjectDetail {
	detail := &ObjectDetail{
		Title:    kind + "/" + obj.GetName(),
		Category: "policy",
		Status:   StatusBadge{Tone: "healthy", Label: "Active"},
		Summary:  fields("Kind", kind),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Generation", fmtInt64(obj.GetGeneration()),
	)))
	if rules, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "rules"); ok {
		var rows [][]string
		for _, r := range rules {
			m, _ := r.(map[string]interface{})
			rows = append(rows, []string{fmt.Sprint(m["name"]), ruleKinds(m)})
		}
		if len(rows) > 0 {
			sections = append(sections, sectionTable("rules", "Rules", GroupSpec,
				[]string{"Name", "Match Kinds"}, rows))
		}
	}
	if validationFailureAction, ok, _ := unstructured.NestedString(obj.Object, "spec", "validationFailureAction"); ok {
		sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
			"Validation Failure Action", validationFailureAction,
		)))
	}
	sections = append(sections, metaSections(obj.GetLabels(), obj.GetAnnotations(), ownerRefsFromMeta(obj.GetOwnerReferences(), obj.GetNamespace()))...)
	if mf := managedFieldsSection(obj.GetManagedFields()); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail
}

func ruleKinds(m map[string]interface{}) string {
	match, _ := m["match"].(map[string]interface{})
	if match == nil {
		return ""
	}
	if resources, ok := match["resources"].(map[string]interface{}); ok {
		if kinds, ok := resources["kinds"].([]interface{}); ok {
			var parts []string
			for _, k := range kinds {
				parts = append(parts, fmt.Sprint(k))
			}
			return joinNonEmpty(parts...)
		}
	}
	return ""
}
