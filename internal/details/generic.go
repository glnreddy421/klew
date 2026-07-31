package details

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

type genericProvider struct{}

func (genericProvider) Kind() string { return "_generic" }

func (genericProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	obj, err := fetchGeneric(ctx, req)
	if err != nil {
		return buildFromSnapshot(req), nil
	}
	detail := &ObjectDetail{
		Title:    req.Ref.Kind + "/" + obj.GetName(),
		Category: categoryFor(req.Ref.Kind),
		Status:   StatusBadge{Tone: "unknown", Label: req.Ref.Kind},
		Summary: fields(
			"API Version", obj.GetAPIVersion(),
			"UID", string(obj.GetUID()),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"API Version", obj.GetAPIVersion(),
		"Resource Version", obj.GetResourceVersion(),
		"Generation", fmtInt64(obj.GetGeneration()),
	)))
	if status, ok, _ := unstructured.NestedMap(obj.Object, "status"); ok && len(status) > 0 {
		kv := map[string]string{}
		flattenMap("", status, kv)
		if len(kv) > 0 {
			sections = append(sections, sectionKV("statusFields", "Status Fields", GroupStatus, kvMap(kv)))
		}
	}
	if spec, ok, _ := unstructured.NestedMap(obj.Object, "spec"); ok && len(spec) > 0 {
		kv := map[string]string{}
		flattenMap("", spec, kv)
		if len(kv) > 40 {
			trimmed := map[string]string{}
			i := 0
			for k, v := range kv {
				if i >= 40 {
					break
				}
				trimmed[k] = v
				i++
			}
			kv = trimmed
		}
		if len(kv) > 0 {
			sections = append(sections, sectionKV("spec", "Spec", GroupSpec, kvMap(kv)))
		}
	}
	sections = append(sections, metaSections(obj.GetLabels(), obj.GetAnnotations(), ownerRefsFromMeta(obj.GetOwnerReferences(), obj.GetNamespace()))...)
	if mf := managedFieldsSection(obj.GetManagedFields()); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func fetchGeneric(ctx context.Context, req *Request) (*unstructured.Unstructured, error) {
	if req.Client == nil || req.Client.Config == nil || req.Client.Clientset == nil {
		return nil, fmt.Errorf("no client")
	}
	disco := req.Client.Clientset.Discovery()
	gr, err := restmapper.GetAPIGroupResources(disco)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)
	candidates := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: req.Ref.Kind},
		{Group: "apps", Version: "v1", Kind: req.Ref.Kind},
		{Group: "batch", Version: "v1", Kind: req.Ref.Kind},
		{Group: "networking.k8s.io", Version: "v1", Kind: req.Ref.Kind},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: req.Ref.Kind},
		{Group: "autoscaling", Version: "v2", Kind: req.Ref.Kind},
		{Group: "storage.k8s.io", Version: "v1", Kind: req.Ref.Kind},
		{Group: "discovery.k8s.io", Version: "v1", Kind: req.Ref.Kind},
		{Group: "gateway.networking.k8s.io", Version: "v1", Kind: req.Ref.Kind},
		{Group: "kyverno.io", Version: "v1", Kind: req.Ref.Kind},
	}
	dyn, err := dynamic.NewForConfig(req.Client.Config)
	if err != nil {
		return nil, err
	}
	var last error
	for _, cand := range candidates {
		if cand.Kind == "" {
			continue
		}
		mapping, err := mapper.RESTMapping(cand.GroupKind(), cand.Version)
		if err != nil {
			last = err
			continue
		}
		var ri dynamic.ResourceInterface
		if mapping.Scope.Name() == "namespace" {
			ri = dyn.Resource(mapping.Resource).Namespace(nsOr(req, ""))
		} else {
			ri = dyn.Resource(mapping.Resource)
		}
		obj, err := ri.Get(ctx, req.Ref.Name, metav1.GetOptions{})
		if err != nil {
			last = err
			continue
		}
		return obj, nil
	}
	if last != nil {
		return nil, last
	}
	return nil, fmt.Errorf("unable to resolve kind %s", req.Ref.Kind)
}

func buildFromSnapshot(req *Request) *ObjectDetail {
	detail := &ObjectDetail{
		Title:    req.Ref.Kind + "/" + req.Ref.Name,
		Category: categoryFor(req.Ref.Kind),
		Status:   StatusBadge{Tone: "unknown", Label: "Snapshot only"},
		Summary: fields(
			"Kind", req.Ref.Kind,
			"Name", req.Ref.Name,
			"Namespace", req.Ref.Namespace,
		),
	}
	detail.Sections = []Section{
		sectionFields("status", "Status", GroupStatus, fields(
			"Kind", req.Ref.Kind,
			"Name", req.Ref.Name,
			"Namespace", req.Ref.Namespace,
			"Note", "Live object fetch unavailable — showing investigation snapshot context only",
		)),
	}
	return detail
}

func flattenMap(prefix string, m map[string]interface{}, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch t := v.(type) {
		case map[string]interface{}:
			if len(out) > 60 {
				return
			}
			flattenMap(key, t, out)
		case []interface{}:
			out[key] = fmt.Sprintf("[%d items]", len(t))
		case string:
			if strings.TrimSpace(t) != "" {
				out[key] = truncate(t, 120)
			}
		case bool:
			out[key] = boolStr(t)
		case float64:
			out[key] = fmt.Sprintf("%v", t)
		case int64:
			out[key] = fmtInt64(t)
		case nil:
		default:
			out[key] = truncate(fmt.Sprint(t), 120)
		}
	}
}
