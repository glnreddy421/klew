package details

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type pvcProvider struct{}

func (pvcProvider) Kind() string { return "PersistentVolumeClaim" }

func (pvcProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	pvc, err := getPVC(ctx, req)
	if err != nil {
		return nil, err
	}
	phase := string(pvc.Status.Phase)
	tone := "healthy"
	switch phase {
	case "Pending":
		tone = "warning"
	case "Lost":
		tone = "critical"
	}
	sc := ""
	if pvc.Spec.StorageClassName != nil {
		sc = *pvc.Spec.StorageClassName
	}
	cap := ""
	if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		cap = q.String()
	} else if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		cap = q.String()
	}
	modes := make([]string, 0, len(pvc.Spec.AccessModes))
	for _, m := range pvc.Spec.AccessModes {
		modes = append(modes, string(m))
	}
	usedBy := podsUsingConfig(req.Snapshot, "PersistentVolumeClaim", pvc.Name)
	detail := &ObjectDetail{
		Title:    "PersistentVolumeClaim/" + pvc.Name,
		Category: "storage",
		Status:   StatusBadge{Tone: tone, Label: phase},
		Summary: fields(
			"Phase", phase,
			"Capacity", cap,
			"StorageClass", sc,
			"Volume", pvc.Spec.VolumeName,
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Phase", phase,
		"Capacity", cap,
	)))
	sections = append(sections, sectionFields("capacity", "Capacity", GroupRuntime, fields("Storage", cap)))
	sections = append(sections, sectionFields("storageClass", "StorageClass", GroupRelationships, fields("Name", sc)))
	vm := ""
	if pvc.Spec.VolumeMode != nil {
		vm = string(*pvc.Spec.VolumeMode)
	}
	sections = append(sections, sectionFields("volume", "Volume", GroupRelationships, fields(
		"PersistentVolume", pvc.Spec.VolumeName,
		"Volume Mode", vm,
	)))
	sections = append(sections, sectionFields("accessModes", "Access Modes", GroupSpec, fields(
		"Modes", strings.Join(modes, ", "),
	)))
	if len(usedBy) > 0 {
		var rows [][]string
		for _, n := range usedBy {
			rows = append(rows, []string{n})
		}
		sections = append(sections, sectionTable("usedByPods", "Used By Pods", GroupRelationships,
			[]string{"Pod"}, rows))
	}
	sections = append(sections, metaSections(pvc.Labels, pvc.Annotations, ownerRefsFromMeta(pvc.OwnerReferences, pvc.Namespace))...)
	if mf := managedFieldsSection(pvc.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
