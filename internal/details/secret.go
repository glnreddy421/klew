package details

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
)

type secretProvider struct{}

func (secretProvider) Kind() string { return "Secret" }

func (secretProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	sec, err := getSecret(ctx, req)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(sec.Data))
	for k := range sec.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	consumers := podsUsingConfig(req.Snapshot, "Secret", sec.Name)
	detail := &ObjectDetail{
		Title:    "Secret/" + sec.Name,
		Category: "config",
		Status:   StatusBadge{Tone: "healthy", Label: string(sec.Type)},
		Summary: fields(
			"Type", string(sec.Type),
			"Keys", fmtInt32(int32(len(keys))),
			"Consumers", fmtInt32(int32(len(consumers))),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("type", "Type", GroupSummary, fields("Type", string(sec.Type))))
	if len(keys) > 0 {
		var rows [][]string
		for _, k := range keys {
			raw := sec.Data[k]
			rows = append(rows, []string{
				k,
				base64.StdEncoding.EncodeToString(raw),
				fmt.Sprintf("%d bytes", len(raw)),
			})
		}
		sections = append(sections, sectionSensitiveTable("keys", "Keys", GroupSpec,
			[]string{"Key", "Value", "Size"}, rows, 1))
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
	sections = append(sections, metaSections(sec.Labels, sec.Annotations, ownerRefsFromMeta(sec.OwnerReferences, sec.Namespace))...)
	if mf := managedFieldsSection(sec.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
