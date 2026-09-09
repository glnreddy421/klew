package details

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
)

type configMapProvider struct{}

func (configMapProvider) Kind() string { return "ConfigMap" }

func (configMapProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	cm, err := getConfigMap(ctx, req)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(cm.Data)+len(cm.BinaryData))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	for k := range cm.BinaryData {
		keys = append(keys, k+" (binary)")
	}
	sort.Strings(keys)
	consumers := podsUsingConfig(req.Snapshot, "ConfigMap", cm.Name)
	detail := &ObjectDetail{
		Title:    "ConfigMap/" + cm.Name,
		Category: "config",
		Status:   StatusBadge{Tone: "healthy", Label: fmt.Sprintf("%d keys", len(keys))},
		Summary: fields(
			"Keys", fmtInt32(int32(len(keys))),
			"Consumers", fmtInt32(int32(len(consumers))),
		),
	}
	var sections []Section
	if len(keys) > 0 {
		var rows [][]string
		for _, k := range keys {
			size := ""
			value := ""
			if v, ok := cm.Data[k]; ok {
				value = v
				size = fmt.Sprintf("%d bytes", len(v))
			} else if b, ok := cm.BinaryData[strings.TrimSuffix(k, " (binary)")]; ok {
				value = base64.StdEncoding.EncodeToString(b)
				size = fmt.Sprintf("%d bytes", len(b))
			}
			rows = append(rows, []string{k, value, size})
		}
		sections = append(sections, sectionTable("dataKeys", "Data Keys", GroupSpec,
			[]string{"Key", "Value", "Size"}, rows))
	}
	if len(consumers) > 0 {
		var rows [][]string
		for _, n := range consumers {
			rows = append(rows, []string{n})
		}
		sections = append(sections, sectionTable("mountedBy", "Mounted By", GroupRelationships,
			[]string{"Pod"}, rows))
		sections = append(sections, sectionTable("consumers", "Consumers", GroupRelationships,
			[]string{"Pod"}, rows))
	}
	sections = append(sections, metaSections(cm.Labels, cm.Annotations, ownerRefsFromMeta(cm.OwnerReferences, cm.Namespace))...)
	if mf := managedFieldsSection(cm.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
