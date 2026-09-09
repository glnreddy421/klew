package details

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// genericBodySections surfaces top-level API fields that are not under spec/status.
// Many admission, config, and legacy kinds store their payload at the object root.
func genericBodySections(kind string, obj map[string]interface{}) []Section {
	if obj == nil {
		return nil
	}
	skip := map[string]struct{}{
		"apiVersion": {}, "kind": {}, "metadata": {}, "status": {}, "spec": {},
	}
	if kind == "ConfigMap" || kind == "Secret" {
		skip["data"] = struct{}{}
		skip["binaryData"] = struct{}{}
	}
	if strings.Contains(kind, "WebhookConfiguration") {
		skip["webhooks"] = struct{}{}
	}

	var keys []string
	for k := range obj {
		if _, omit := skip[k]; omit {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sections []Section
	for _, k := range keys {
		switch k {
		case "webhooks":
			if sec := webhookConfigurationSection(obj); !sec.Empty() {
				sections = append(sections, sec)
			}
		default:
			if sec := genericRootFieldSection(k, obj[k]); !sec.Empty() {
				sections = append(sections, sec)
			}
		}
	}
	return sections
}

func genericRootFieldSection(key string, value interface{}) Section {
	title := humanizeFieldKey(key)
	switch t := value.(type) {
	case map[string]interface{}:
		kv := flattenToMap(key, t)
		if len(kv) == 0 {
			return Section{}
		}
		return sectionKV(key, title, GroupSpec, kvMap(kv))
	case []interface{}:
		if sec := genericSliceSection(key, title, t); !sec.Empty() {
			return sec
		}
		return Section{}
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" {
			return Section{}
		}
		return sectionFields(key, title, GroupSpec, fields(title, truncate(text, 500)))
	}
}

func genericSliceSection(key, title string, items []interface{}) Section {
	if len(items) == 0 {
		return Section{}
	}
	if _, ok := items[0].(map[string]interface{}); ok {
		cols, rows := tableFromObjectSlice(items)
		if len(rows) > 0 {
			return sectionTable(key, title, GroupSpec, cols, rows)
		}
	}
	kv := map[string]string{}
	flattenSlice(key, items, kv)
	if len(kv) == 0 {
		return Section{}
	}
	return sectionKV(key, title, GroupSpec, kvMap(kv))
}

func tableFromObjectSlice(items []interface{}) ([]string, [][]string) {
	if len(items) == 0 {
		return nil, nil
	}
	if _, ok := items[0].(map[string]interface{}); !ok {
		return nil, nil
	}
	colSet := map[string]struct{}{}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		for k := range m {
			if k == "clientConfig" {
				colSet["Service"] = struct{}{}
				continue
			}
			if isScalarJSON(m[k]) {
				colSet[humanizeFieldKey(k)] = struct{}{}
			}
		}
	}
	if len(colSet) == 0 {
		return nil, nil
	}
	cols := make([]string, 0, len(colSet))
	for c := range colSet {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	var rows [][]string
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		row := make([]string, len(cols))
		for i, col := range cols {
			row[i] = scalarFromWebhookField(m, col)
		}
		rows = append(rows, row)
	}
	return cols, rows
}

func isScalarJSON(v interface{}) bool {
	switch v.(type) {
	case string, bool, float64, int64, int, nil:
		return true
	default:
		return false
	}
}

func scalarFromWebhookField(m map[string]interface{}, col string) string {
	key := strings.ToLower(col)
	switch key {
	case "service":
		return webhookServiceRef(m)
	default:
		if v, ok := m[key]; ok && isScalarJSON(v) {
			return truncate(fmt.Sprint(v), 240)
		}
		if v, ok := m[col]; ok && isScalarJSON(v) {
			return truncate(fmt.Sprint(v), 240)
		}
	}
	return ""
}

func webhookConfigurationSection(obj map[string]interface{}) Section {
	hooks, _, _ := unstructured.NestedSlice(obj, "webhooks")
	if len(hooks) == 0 {
		return Section{}
	}
	var rows [][]string
	for _, item := range hooks {
		h, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rows = append(rows, []string{
			nestedString(h, "name"),
			webhookServiceRef(h),
			nestedString(h, "clientConfig", "service", "path"),
			nestedString(h, "failurePolicy"),
			nestedString(h, "sideEffects"),
			webhookRulesSummary(h),
		})
	}
	return sectionTable("webhooks", "Webhooks", GroupSpec,
		[]string{"Name", "Service", "Path", "Failure Policy", "Side Effects", "Rules"}, rows)
}

func webhookServiceRef(h map[string]interface{}) string {
	if url := nestedString(h, "clientConfig", "url"); url != "" {
		return truncate(url, 120)
	}
	ns := nestedString(h, "clientConfig", "service", "namespace")
	name := nestedString(h, "clientConfig", "service", "name")
	if name == "" {
		return ""
	}
	if ns != "" {
		return ns + "/" + name
	}
	return name
}

func webhookRulesSummary(h map[string]interface{}) string {
	rules, _, _ := unstructured.NestedSlice(h, "rules")
	if len(rules) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rules))
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		resources, _, _ := unstructured.NestedStringSlice(rule, "resources")
		ops, _, _ := unstructured.NestedStringSlice(rule, "operations")
		groups, _, _ := unstructured.NestedStringSlice(rule, "apiGroups")
		versions, _, _ := unstructured.NestedStringSlice(rule, "apiVersions")
	 chunk := strings.Join(resources, ",")
		if len(ops) > 0 {
			chunk += " (" + strings.Join(ops, ",") + ")"
		}
		if len(groups) > 0 || len(versions) > 0 {
			chunk += " " + strings.Join(groups, ",") + "/" + strings.Join(versions, ",")
		}
		if chunk != "" {
			parts = append(parts, chunk)
		}
	}
	return strings.Join(parts, "; ")
}

func nestedString(obj map[string]interface{}, fields ...string) string {
	v, found, err := unstructured.NestedString(obj, fields...)
	if !found || err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

func nestedScalar(obj map[string]interface{}, fields ...string) string {
	v, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func humanizeFieldKey(key string) string {
	key = strings.ReplaceAll(key, "_", " ")
	if key == "" {
		return key
	}
	return strings.ToUpper(key[:1]) + key[1:]
}
