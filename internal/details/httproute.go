package details

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type httpRouteProvider struct{}

func (httpRouteProvider) Kind() string { return "HTTPRoute" }

func (httpRouteProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	obj, err := getUnstructured(ctx, req, schema.GroupVersionResource{
		Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes",
	}, true)
	if err != nil {
		obj, err = getUnstructured(ctx, req, schema.GroupVersionResource{
			Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "httproutes",
		}, true)
		if err != nil {
			return nil, err
		}
	}
	hostnames, _, _ := unstructured.NestedStringSlice(obj.Object, "spec", "hostnames")
	detail := &ObjectDetail{
		Title:    "HTTPRoute/" + obj.GetName(),
		Category: "network",
		Status:   StatusBadge{Tone: "healthy", Label: "Configured"},
		Summary:  fields("Hosts", strings.Join(hostnames, ", ")),
	}
	var sections []Section
	if len(hostnames) > 0 {
		var rows [][]string
		for _, h := range hostnames {
			rows = append(rows, []string{h})
		}
		sections = append(sections, sectionTable("hosts", "Hosts", GroupSpec, []string{"Hostname"}, rows))
	}
	if parentRefs, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "parentRefs"); ok {
		var rows [][]string
		for _, p := range parentRefs {
			m, _ := p.(map[string]interface{})
			rows = append(rows, []string{
				fmt.Sprint(m["kind"]),
				fmt.Sprint(m["name"]),
				fmt.Sprint(m["namespace"]),
			})
		}
		if len(rows) > 0 {
			sections = append(sections, sectionTable("parentRefs", "Parent Refs", GroupRelationships,
				[]string{"Kind", "Name", "Namespace"}, rows))
		}
	}
	if rules, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "rules"); ok {
		var rows [][]string
		for i, r := range rules {
			m, _ := r.(map[string]interface{})
			backend := ""
			if bks, ok := m["backendRefs"].([]interface{}); ok && len(bks) > 0 {
				if bm, ok := bks[0].(map[string]interface{}); ok {
					backend = fmt.Sprintf("%s:%v", bm["name"], bm["port"])
				}
			}
			rows = append(rows, []string{fmt.Sprintf("%d", i), backend})
		}
		if len(rows) > 0 {
			sections = append(sections, sectionTable("rules", "Rules", GroupSpec,
				[]string{"Index", "Backend"}, rows))
		}
	}
	sections = append(sections, metaSections(obj.GetLabels(), obj.GetAnnotations(), ownerRefsFromMeta(obj.GetOwnerReferences(), obj.GetNamespace()))...)
	if mf := managedFieldsSection(obj.GetManagedFields()); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
