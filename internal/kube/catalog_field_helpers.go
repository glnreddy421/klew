package kube

import (
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/glnreddy421/klew/internal/model"
)

// firstStringFromObject returns the first non-empty string found at any path.
// Each path is a nested field chain, e.g. []string{"spec","reclaimPolicy"}.
func firstStringFromObject(obj map[string]interface{}, paths ...[]string) string {
	for _, path := range paths {
		if v := stringFromObject(obj, path...); v != "" {
			return v
		}
	}
	return ""
}

func catalogAccessModes(obj map[string]interface{}) []string {
	raw, found, err := unstructured.NestedSlice(obj, "spec", "accessModes")
	if !found || err != nil || len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func catalogStorageQuantity(obj map[string]interface{}, fields ...string) string {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil || val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case map[string]interface{}:
		if s, ok := v["storage"].(string); ok {
			return s
		}
	}
	return fmt.Sprintf("%v", val)
}

func catalogMapKeyCount(obj map[string]interface{}, fields ...string) int32 {
	val, found, err := unstructured.NestedMap(obj, fields...)
	if !found || err != nil || len(val) == 0 {
		return 0
	}
	return clampInt32(int64(len(val)))
}

func formatAccessModes(modes []string) string {
	if len(modes) == 0 {
		return ""
	}
	return strings.Join(modes, ",")
}

func catalogConfigMapDataEntries(obj map[string]interface{}) []model.CatalogDataEntry {
	if obj == nil {
		return nil
	}
	var out []model.CatalogDataEntry
	if data, found, _ := unstructured.NestedStringMap(obj, "data"); found {
		for k, v := range data {
			out = append(out, model.CatalogDataEntry{
				Key:       k,
				Value:     v,
				SizeBytes: clampInt32(int64(len(v))),
			})
		}
	}
	if bin, found, _ := unstructured.NestedStringMap(obj, "binaryData"); found {
		for k, v := range bin {
			size := clampInt32(int64(len(v)))
			if decoded, err := base64.StdEncoding.DecodeString(v); err == nil {
				size = clampInt32(int64(len(decoded)))
			}
			out = append(out, model.CatalogDataEntry{
				Key:       k + " (binary)",
				Value:     v,
				SizeBytes: size,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// clampInt32 narrows n to int32, saturating instead of wrapping on overflow.
func clampInt32(n int64) int32 {
	switch {
	case n > math.MaxInt32:
		return math.MaxInt32
	case n < math.MinInt32:
		return math.MinInt32
	}
	return int32(n)
}
