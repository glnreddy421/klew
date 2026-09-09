package kube

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/glnreddy421/klew/internal/model"
)

func enrichBuiltinCatalogEntity(resource string, entity *model.CatalogEntity, obj map[string]interface{}) {
	switch resource {
	case "replicationcontrollers":
		enrichWorkloadCatalogEntity(entity, obj)
	case "persistentvolumeclaims":
		enrichPVCCatalogEntity(entity, obj)
	case "persistentvolumes":
		enrichPVCatalogEntity(entity, obj)
	case "configmaps":
		enrichConfigMapCatalogEntity(entity, obj)
	case "secrets":
		enrichSecretCatalogEntity(entity, obj)
	case "horizontalpodautoscalers":
		enrichHPACatalogEntity(entity, obj)
	case "poddisruptionbudgets":
		enrichPDBCatalogEntity(entity, obj)
	case "leases":
		enrichLeaseCatalogEntity(entity, obj)
	}
}

func enrichLeaseCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.LeaseHolder = stringFromObject(obj, "spec", "holderIdentity")
	if entity.LeaseHolder != "" {
		entity.StatusHint = entity.LeaseHolder
	}
}

func enrichPVCCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	if phase := stringFromObject(obj, "status", "phase"); phase != "" {
		entity.StatusHint = phase
	}
	entity.VolumeName = stringFromObject(obj, "spec", "volumeName")
	entity.StorageClassName = stringFromObject(obj, "spec", "storageClassName")
	entity.Capacity = catalogStorageQuantity(obj, "status", "capacity", "storage")
	if entity.Capacity == "" {
		entity.Capacity = catalogStorageQuantity(obj, "spec", "resources", "requests", "storage")
	}
	entity.AccessModes = catalogAccessModes(obj)
}

func enrichPVCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	if phase := stringFromObject(obj, "status", "phase"); phase != "" {
		entity.StatusHint = phase
	}
	entity.Capacity = catalogStorageQuantity(obj, "spec", "capacity", "storage")
	entity.AccessModes = catalogAccessModes(obj)
	entity.ReclaimPolicy = stringFromObject(obj, "spec", "persistentVolumeReclaimPolicy")
	entity.StorageClassName = stringFromObject(obj, "spec", "storageClassName")
	claimNS := stringFromObject(obj, "spec", "claimRef", "namespace")
	claimName := stringFromObject(obj, "spec", "claimRef", "name")
	switch {
	case claimName != "" && claimNS != "":
		entity.ClaimRef = claimNS + "/" + claimName
	case claimName != "":
		entity.ClaimRef = claimName
	}
}

func enrichConfigMapCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.ConfigMapData = catalogConfigMapDataEntries(obj)
	count := int32(len(entity.ConfigMapData))
	entity.DataKeys = &count
	if count > 0 {
		entity.StatusHint = fmt.Sprintf("%d keys", count)
	}
}

func enrichSecretCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.SecretType = stringFromObject(obj, "type")
	count := catalogMapKeyCount(obj, "data")
	entity.DataKeys = &count
	if entity.SecretType != "" {
		entity.StatusHint = entity.SecretType
	}
}

func enrichHPACatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.ScaleTarget = catalogHPAScaleTarget(obj)
	entity.MinReplicas = int32FromObject(obj, "spec", "minReplicas")
	entity.MaxReplicas = int32FromObject(obj, "spec", "maxReplicas")
	entity.CurrentReplicas = int32FromObject(obj, "status", "currentReplicas")
	entity.DesiredReplicas = int32FromObject(obj, "status", "desiredReplicas")
	entity.MetricsSummary = catalogHPAMetricsSummary(obj)
	if cur, des := entity.CurrentReplicas, entity.DesiredReplicas; cur != nil && des != nil {
		entity.StatusHint = fmt.Sprintf("%d→%d", *cur, *des)
	}
}

func catalogHPAScaleTarget(obj map[string]interface{}) string {
	kind := stringFromObject(obj, "spec", "scaleTargetRef", "kind")
	name := stringFromObject(obj, "spec", "scaleTargetRef", "name")
	if kind == "" && name == "" {
		return ""
	}
	if kind == "" {
		return name
	}
	if name == "" {
		return kind
	}
	return kind + "/" + name
}

func catalogHPAMetricsSummary(obj map[string]interface{}) string {
	raw, found, err := unstructured.NestedSlice(obj, "status", "currentMetrics")
	if !found || err != nil || len(raw) == 0 {
		return ""
	}
	parts := make([]string, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		switch {
		case m["resource"] != nil:
			rm, _ := m["resource"].(map[string]interface{})
			name := stringFromMap(rm, "name")
			cur, _ := rm["current"].(map[string]interface{})
			val := stringFromMap(cur, "averageUtilization")
			if name != "" && val != "" {
				parts = append(parts, fmt.Sprintf("%s:%s%%", name, val))
			} else if name != "" {
				parts = append(parts, name)
			}
		case m["pods"] != nil:
			pm, _ := m["pods"].(map[string]interface{})
			cur, _ := pm["current"].(map[string]interface{})
			val := stringFromMap(cur, "averageValue")
			if val != "" {
				parts = append(parts, "pods:"+val)
			} else {
				parts = append(parts, "pods")
			}
		case m["object"] != nil:
			parts = append(parts, "object")
		case m["external"] != nil:
			parts = append(parts, "external")
		}
	}
	return strings.Join(parts, ", ")
}

func enrichPDBCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	minAvail := firstStringFromObject(obj,
		[]string{"spec", "minAvailable"},
		[]string{"status", "desiredHealthy"},
	)
	maxUnavail := stringFromObject(obj, "spec", "maxUnavailable")
	allowed := int32FromObject(obj, "status", "disruptionsAllowed")
	if minAvail != "" {
		entity.StatusHint = "minAvailable=" + minAvail
	} else if maxUnavail != "" {
		entity.StatusHint = "maxUnavailable=" + maxUnavail
	}
	if minAvail != "" {
		entity.PDBMinAvailable = minAvail
	}
	if maxUnavail != "" {
		entity.PDBMaxUnavailable = maxUnavail
	}
	if allowed != nil {
		entity.PDBDisruptionsAllowed = allowed
	}
}
