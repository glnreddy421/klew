package details

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type webhookConfigurationProvider struct {
	kind     string
	mutating bool
}

func (p webhookConfigurationProvider) Kind() string { return p.kind }

func (p webhookConfigurationProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	obj, err := getWebhookConfiguration(ctx, req, p.mutating)
	if err != nil {
		return nil, err
	}
	hooks, _, _ := unstructured.NestedSlice(obj.Object, "webhooks")
	detail := &ObjectDetail{
		Title:    p.kind + "/" + obj.GetName(),
		Category: "config",
		Status:   StatusBadge{Tone: "healthy", Label: fmt.Sprintf("%d webhooks", len(hooks))},
		Summary: fields(
			"Webhooks", fmtInt32(int32(len(hooks))),
			"API Version", obj.GetAPIVersion(),
		),
	}
	var sections []Section
	if sec := webhookConfigurationSection(obj.Object); !sec.Empty() {
		sections = append(sections, sec)
	}
	for i, item := range hooks {
		h, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id := fmt.Sprintf("webhook-%d", i)
		title := nestedString(h, "name")
		if title == "" {
			title = fmt.Sprintf("Webhook %d", i+1)
		}
		sections = append(sections, sectionFields(id, title, GroupSpec, fields(
			"Name", nestedString(h, "name"),
			"Service", webhookServiceRef(h),
			"Path", nestedString(h, "clientConfig", "service", "path"),
			"Port", nestedScalar(h, "clientConfig", "service", "port"),
			"Failure Policy", nestedString(h, "failurePolicy"),
			"Match Policy", nestedString(h, "matchPolicy"),
			"Side Effects", nestedString(h, "sideEffects"),
			"Timeout Seconds", nestedString(h, "timeoutSeconds"),
			"Admission Review Versions", joinNestedStringSlice(h, "admissionReviewVersions"),
			"Rules", webhookRulesSummary(h),
		)))
	}
	sections = append(sections, genericBodySections(p.kind, obj.Object)...)
	sections = append(sections, metaSections(obj.GetLabels(), obj.GetAnnotations(), ownerRefsFromMeta(obj.GetOwnerReferences(), ""))...)
	if mf := managedFieldsSection(obj.GetManagedFields()); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func getWebhookConfiguration(ctx context.Context, req *Request, mutating bool) (*unstructured.Unstructured, error) {
	resource := "validatingwebhookconfigurations"
	if mutating {
		resource = "mutatingwebhookconfigurations"
	}
	gvr := schema.GroupVersionResource{
		Group:    "admissionregistration.k8s.io",
		Version:  "v1",
		Resource: resource,
	}
	return getUnstructured(ctx, req, gvr, false)
}

func joinNestedStringSlice(obj map[string]interface{}, fields ...string) string {
	vals, found, err := unstructured.NestedStringSlice(obj, fields...)
	if !found || err != nil || len(vals) == 0 {
		return ""
	}
	return stringsJoin(vals)
}

func stringsJoin(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	out := vals[0]
	for i := 1; i < len(vals); i++ {
		out += ", " + vals[i]
	}
	return out
}
