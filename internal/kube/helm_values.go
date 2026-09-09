package kube

import (
	"fmt"
	"reflect"

	"sigs.k8s.io/yaml"
)

func marshalHelmValuesYAML(values map[string]interface{}) string {
	if len(values) == 0 {
		return "# (empty)\n"
	}
	out, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Sprintf("# failed to marshal values: %v\n", err)
	}
	return string(out)
}

// helmUserSuppliedValues returns keys in config that differ from chart defaults.
func helmUserSuppliedValues(config, defaults map[string]interface{}) map[string]interface{} {
	if len(config) == 0 {
		return map[string]interface{}{}
	}
	if len(defaults) == 0 {
		return cloneMap(config)
	}
	return diffHelmValueMaps(defaults, config)
}

func diffHelmValueMaps(base, overlay map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for key, overlayVal := range overlay {
		baseVal, ok := base[key]
		if !ok {
			out[key] = overlayVal
			continue
		}
		overlayMap, overlayIsMap := overlayVal.(map[string]interface{})
		baseMap, baseIsMap := baseVal.(map[string]interface{})
		if overlayIsMap && baseIsMap {
			nested := diffHelmValueMaps(baseMap, overlayMap)
			if len(nested) > 0 {
				out[key] = nested
			}
			continue
		}
		if !reflect.DeepEqual(normalizeHelmScalar(baseVal), normalizeHelmScalar(overlayVal)) {
			out[key] = overlayVal
		}
	}
	return out
}

func normalizeHelmScalar(v interface{}) interface{} {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) {
			return int64(x)
		}
		return x
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, item := range x {
			out[i] = normalizeHelmScalar(item)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, item := range x {
			out[k] = normalizeHelmScalar(item)
		}
		return out
	default:
		return v
	}
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
