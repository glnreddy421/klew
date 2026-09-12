package details

import (
	"context"
	"fmt"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

type genericProvider struct{}

func (genericProvider) Kind() string { return "_generic" }

func (genericProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	obj, err := fetchGeneric(ctx, req)
	if err != nil {
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
			return nil, wrapDetailErr(err, req.Ref.Kind, req.Ref.Name)
		}
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
	sections = append(sections, sectionFields("status", "Status", GroupSummary, fields(
		"API Version", obj.GetAPIVersion(),
		"Resource Version", obj.GetResourceVersion(),
		"Generation", fmtInt64(obj.GetGeneration()),
	)))
	if status, ok, _ := unstructured.NestedMap(obj.Object, "status"); ok && len(status) > 0 {
		kv := flattenToMap("", status)
		if len(kv) > 0 {
			sections = append(sections, sectionKV("statusFields", "Status Fields", GroupSummary, kvMap(kv)))
		}
	}
	if spec, ok, _ := unstructured.NestedMap(obj.Object, "spec"); ok && len(spec) > 0 {
		kv := flattenToMap("", spec)
		if len(kv) > 0 {
			sections = append(sections, sectionKV("spec", "Spec", GroupSpec, kvMap(kv)))
		}
	}
	// ConfigMap/Secret store payload at root — surface under Spec for inspector parity.
	if req.Ref.Kind == "ConfigMap" || req.Ref.Kind == "Secret" {
		if data, ok, _ := unstructured.NestedStringMap(obj.Object, "data"); ok && len(data) > 0 {
			kv := map[string]string{}
			for k, v := range data {
				kv["data."+k] = truncate(v, 240)
			}
			sections = append(sections, sectionKV("data", "Data", GroupSpec, kvMap(kv)))
		}
	}
	// Root-level body (webhooks, rules, etc.) for kinds without spec or with spec + root fields.
	sections = append(sections, genericBodySections(req.Ref.Kind, obj.Object)...)
	sections = append(sections, metaSections(obj.GetLabels(), obj.GetAnnotations(), ownerRefsFromMeta(obj.GetOwnerReferences(), obj.GetNamespace()))...)
	if mf := managedFieldsSection(obj.GetManagedFields()); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

type gvrCandidate struct {
	gvr        schema.GroupVersionResource
	namespaced bool
	priority   int
}

func fetchGeneric(ctx context.Context, req *Request) (*unstructured.Unstructured, error) {
	if req.Client == nil || req.Client.Config == nil || req.Client.Clientset == nil {
		return nil, fmt.Errorf("no client")
	}
	disco := req.Client.Clientset.Discovery()
	resourceLists, err := disco.ServerPreferredResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, err
	}
	kind := req.Ref.Kind
	var candidates []gvrCandidate
	for _, rl := range resourceLists {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range rl.APIResources {
			if r.Kind != kind || strings.Contains(r.Name, "/") {
				continue
			}
			candidates = append(candidates, gvrCandidate{
				gvr: schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: r.Name,
				},
				namespaced: r.Namespaced,
				priority:   apiGroupPriority(gv.Group),
			})
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no API resource found for kind %s", kind)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		gi := candidates[i].gvr.Group + "/" + candidates[i].gvr.Version + "/" + candidates[i].gvr.Resource
		gj := candidates[j].gvr.Group + "/" + candidates[j].gvr.Version + "/" + candidates[j].gvr.Resource
		return gi < gj
	})

	dyn, err := dynamic.NewForConfig(req.Client.Config)
	if err != nil {
		return nil, err
	}
	var last error
	for _, cand := range candidates {
		var ri dynamic.ResourceInterface
		if cand.namespaced {
			ri = dyn.Resource(cand.gvr).Namespace(nsOr(req, ""))
		} else {
			ri = dyn.Resource(cand.gvr)
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
	return nil, fmt.Errorf("unable to resolve kind %s", kind)
}

func apiGroupPriority(group string) int {
	switch group {
	case "":
		return 0
	case "apps", "batch", "networking.k8s.io", "policy", "autoscaling", "storage.k8s.io",
		"rbac.authorization.k8s.io", "discovery.k8s.io", "coordination.k8s.io", "scheduling.k8s.io",
		"node.k8s.io", "apiextensions.k8s.io", "apiregistration.k8s.io", "admissionregistration.k8s.io":
		return 1
	default:
		return 2
	}
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
		sectionFields("status", "Status", GroupSummary, fields(
			"Kind", req.Ref.Kind,
			"Name", req.Ref.Name,
			"Namespace", req.Ref.Namespace,
			"Note", "Live object fetch unavailable — showing investigation snapshot context only",
		)),
	}
	return detail
}

const flattenMaxKeys = 200

func flattenToMap(prefix string, m map[string]interface{}) map[string]string {
	out := map[string]string{}
	flattenMap(prefix, m, out)
	return out
}

func flattenMap(prefix string, m map[string]interface{}, out map[string]string) {
	if len(out) >= flattenMaxKeys {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(out) >= flattenMaxKeys {
			return
		}
		v := m[k]
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch t := v.(type) {
		case map[string]interface{}:
			flattenMap(key, t, out)
		case []interface{}:
			flattenSlice(key, t, out)
		case string:
			if strings.TrimSpace(t) != "" {
				out[key] = truncate(t, 240)
			}
		case bool:
			out[key] = boolStr(t)
		case float64:
			out[key] = fmt.Sprintf("%v", t)
		case int64:
			out[key] = fmtInt64(t)
		case nil:
		default:
			out[key] = truncate(fmt.Sprint(t), 240)
		}
	}
}

func flattenSlice(key string, items []interface{}, out map[string]string) {
	if len(out) >= flattenMaxKeys {
		return
	}
	if len(items) == 0 {
		out[key] = "[]"
		return
	}
	allStrings := true
	for _, item := range items {
		if _, ok := item.(string); !ok {
			allStrings = false
			break
		}
	}
	if allStrings {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			parts = append(parts, truncate(fmt.Sprint(item), 120))
		}
		out[key] = strings.Join(parts, ", ")
		return
	}
	limit := len(items)
	if limit > 12 {
		limit = 12
	}
	for i := 0; i < limit; i++ {
		if len(out) >= flattenMaxKeys {
			return
		}
		itemKey := fmt.Sprintf("%s[%d]", key, i)
		switch im := items[i].(type) {
		case map[string]interface{}:
			flattenMap(itemKey, im, out)
		default:
			out[itemKey] = truncate(fmt.Sprint(im), 240)
		}
	}
	if len(items) > limit {
		out[key+"._count"] = fmt.Sprintf("%d items", len(items))
	}
}
