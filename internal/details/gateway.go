package details

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type gatewayProvider struct{}

func (gatewayProvider) Kind() string { return "Gateway" }

func (gatewayProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	obj, err := getUnstructured(ctx, req, schema.GroupVersionResource{
		Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways",
	}, true)
	if err != nil {
		// try v1beta1
		obj, err = getUnstructured(ctx, req, schema.GroupVersionResource{
			Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "gateways",
		}, true)
		if err != nil {
			return nil, err
		}
	}
	return buildGatewayLike(obj, "Gateway"), nil
}

func buildGatewayLike(obj *unstructured.Unstructured, kind string) *ObjectDetail {
	class, _, _ := unstructured.NestedString(obj.Object, "spec", "gatewayClassName")
	detail := &ObjectDetail{
		Title:    kind + "/" + obj.GetName(),
		Category: "network",
		Status:   StatusBadge{Tone: "healthy", Label: "Configured"},
		Summary:  fields("GatewayClass", class),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"GatewayClass", class,
	)))
	if listeners, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "listeners"); ok {
		var rows [][]string
		for _, l := range listeners {
			m, _ := l.(map[string]interface{})
			rows = append(rows, []string{
				fmt.Sprint(m["name"]),
				fmt.Sprint(m["protocol"]),
				fmt.Sprint(m["port"]),
				fmt.Sprint(m["hostname"]),
			})
		}
		if len(rows) > 0 {
			sections = append(sections, sectionTable("listeners", "Listeners", GroupSpec,
				[]string{"Name", "Protocol", "Port", "Hostname"}, rows))
		}
	}
	if addrs, ok, _ := unstructured.NestedSlice(obj.Object, "status", "addresses"); ok {
		var rows [][]string
		for _, a := range addrs {
			m, _ := a.(map[string]interface{})
			rows = append(rows, []string{fmt.Sprint(m["type"]), fmt.Sprint(m["value"])})
		}
		if len(rows) > 0 {
			sections = append(sections, sectionTable("addresses", "Addresses", GroupStatus,
				[]string{"Type", "Value"}, rows))
		}
	}
	sections = append(sections, metaSections(obj.GetLabels(), obj.GetAnnotations(), ownerRefsFromMeta(obj.GetOwnerReferences(), obj.GetNamespace()))...)
	if mf := managedFieldsSection(obj.GetManagedFields()); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail
}
