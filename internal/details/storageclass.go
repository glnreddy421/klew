package details

import (
	"context"
	"strings"
)

type storageClassProvider struct{}

func (storageClassProvider) Kind() string { return "StorageClass" }

func (storageClassProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	sc, err := getStorageClass(ctx, req)
	if err != nil {
		return nil, err
	}
	reclaim := ""
	if sc.ReclaimPolicy != nil {
		reclaim = string(*sc.ReclaimPolicy)
	}
	binding := ""
	if sc.VolumeBindingMode != nil {
		binding = string(*sc.VolumeBindingMode)
	}
	detail := &ObjectDetail{
		Title:    "StorageClass/" + sc.Name,
		Category: "storage",
		Status:   StatusBadge{Tone: "healthy", Label: "Available"},
		Summary: fields(
			"Provisioner", sc.Provisioner,
			"Reclaim Policy", reclaim,
			"Binding Mode", binding,
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupSummary, fields(
		"Provisioner", sc.Provisioner,
		"Allow Volume Expansion", boolStrPtr(sc.AllowVolumeExpansion),
	)))
	defaultClass := ""
	if v, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]; ok {
		defaultClass = strings.TrimSpace(v)
	} else if v, ok := sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"]; ok {
		defaultClass = strings.TrimSpace(v)
	}
	sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
		"Provisioner", sc.Provisioner,
		"Reclaim Policy", reclaim,
		"Volume Binding Mode", binding,
		"Default Class", defaultClass,
		"Allow Volume Expansion", boolStrPtr(sc.AllowVolumeExpansion),
	)))
	if kv := kvMap(sc.Parameters); len(kv) > 0 {
		sections = append(sections, sectionKV("parameters", "Parameters", GroupSpec, kv))
	}
	sections = append(sections, metaSections(sc.Labels, sc.Annotations, ownerRefsFromMeta(sc.OwnerReferences, ""))...)
	if mf := managedFieldsSection(sc.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func boolStrPtr(p *bool) string {
	if p == nil {
		return ""
	}
	return boolStr(*p)
}
