package details

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type pvProvider struct{}

func (pvProvider) Kind() string { return "PersistentVolume" }

func (pvProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	pv, err := getPV(ctx, req)
	if err != nil {
		return nil, err
	}
	phase := string(pv.Status.Phase)
	tone := "healthy"
	if phase == "Failed" || phase == "Released" {
		tone = "warning"
	}
	cap := ""
	if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		cap = q.String()
	}
	claim := ""
	if pv.Spec.ClaimRef != nil {
		claim = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
	}
	csi := ""
	if pv.Spec.CSI != nil {
		csi = pv.Spec.CSI.Driver
	}
	detail := &ObjectDetail{
		Title:    "PersistentVolume/" + pv.Name,
		Category: "storage",
		Status:   StatusBadge{Tone: tone, Label: phase},
		Summary: fields(
			"Phase", phase,
			"Claim", claim,
			"StorageClass", pv.Spec.StorageClassName,
			"Capacity", cap,
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Phase", phase,
		"Message", pv.Status.Message,
		"Reason", pv.Status.Reason,
	)))
	sections = append(sections, sectionFields("claim", "Claim", GroupRelationships, fields(
		"Claim Ref", claim,
	)))
	sections = append(sections, sectionFields("storageClass", "StorageClass", GroupRelationships, fields(
		"Name", pv.Spec.StorageClassName,
	)))
	sections = append(sections, sectionFields("capacity", "Capacity", GroupRuntime, fields("Storage", cap)))
	if na := nodeAffinityString(pv.Spec.NodeAffinity); na != "" {
		sections = append(sections, sectionFields("nodeAffinity", "Node Affinity", GroupSpec, fields("Rules", na)))
	}
	sections = append(sections, sectionFields("csiDriver", "CSI Driver", GroupSpec, fields(
		"Driver", csi,
		"Volume Handle", csiVolumeHandle(pv),
	)))
	sections = append(sections, sectionFields("reclaimPolicy", "Reclaim Policy", GroupSpec, fields(
		"Policy", string(pv.Spec.PersistentVolumeReclaimPolicy),
		"Access Modes", joinAccessModes(pv.Spec.AccessModes),
		"Volume Mode", volumeModeString(pv.Spec.VolumeMode),
		"Mount Options", strings.Join(pv.Spec.MountOptions, ", "),
		"Volume Source", pvSourceSummary(pv),
	)))
	sections = append(sections, metaSections(pv.Labels, pv.Annotations, ownerRefsFromMeta(pv.OwnerReferences, ""))...)
	if mf := managedFieldsSection(pv.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func csiVolumeHandle(pv *corev1.PersistentVolume) string {
	if pv.Spec.CSI != nil {
		return pv.Spec.CSI.VolumeHandle
	}
	return ""
}

func joinAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
	parts := make([]string, 0, len(modes))
	for _, m := range modes {
		parts = append(parts, string(m))
	}
	return strings.Join(parts, ", ")
}

func volumeModeString(mode *corev1.PersistentVolumeMode) string {
	if mode == nil {
		return ""
	}
	return string(*mode)
}

func pvSourceSummary(pv *corev1.PersistentVolume) string {
	spec := pv.Spec
	switch {
	case spec.HostPath != nil:
		return fmt.Sprintf("hostPath: %s", spec.HostPath.Path)
	case spec.NFS != nil:
		return fmt.Sprintf("nfs: %s:%s", spec.NFS.Server, spec.NFS.Path)
	case spec.CSI != nil:
		return fmt.Sprintf("csi: %s (%s)", spec.CSI.Driver, spec.CSI.VolumeHandle)
	case spec.Local != nil:
		return fmt.Sprintf("local: %s", spec.Local.Path)
	case spec.GCEPersistentDisk != nil:
		return fmt.Sprintf("gcePersistentDisk: %s", spec.GCEPersistentDisk.PDName)
	case spec.AWSElasticBlockStore != nil:
		return fmt.Sprintf("awsElasticBlockStore: %s", spec.AWSElasticBlockStore.VolumeID)
	case spec.AzureDisk != nil:
		return fmt.Sprintf("azureDisk: %s", spec.AzureDisk.DiskName)
	case spec.CephFS != nil:
		return fmt.Sprintf("cephfs: %s", spec.CephFS.Path)
	case spec.Glusterfs != nil:
		return fmt.Sprintf("glusterfs: %s", spec.Glusterfs.Path)
	default:
		return ""
	}
}

func nodeAffinityString(na *corev1.VolumeNodeAffinity) string {
	if na == nil || na.Required == nil {
		return ""
	}
	var parts []string
	for _, term := range na.Required.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			parts = append(parts, fmt.Sprintf("%s %s %s", expr.Key, expr.Operator, strings.Join(expr.Values, ",")))
		}
	}
	return strings.Join(parts, "; ")
}
