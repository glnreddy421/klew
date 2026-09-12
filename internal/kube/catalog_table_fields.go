package kube

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/glnreddy421/klew/internal/model"
)

// enrichCatalogTableFields fills CatalogEntity.TableFields for browse tables.
// Paths follow upstream API shapes (spec.*, status.*, or root — never guessed).
func enrichCatalogTableFields(entity *model.CatalogEntity, obj map[string]interface{}, resource string) {
	if entity == nil || obj == nil {
		return
	}
	fields := map[string]string{}
	switch resource {
	case "pods":
		catalogTablePod(fields, entity, obj)
	case "deployments", "statefulsets", "replicasets", "replicationcontrollers":
		catalogTableReplicaController(fields, entity, obj, resource)
	case "daemonsets":
		catalogTableDaemonSet(fields, entity, obj)
	case "jobs":
		catalogTableJob(fields, entity, obj)
	case "cronjobs":
		catalogTableCronJob(fields, entity, obj)
	case "services":
		catalogTableService(fields, entity, obj)
	case "endpoints", "endpointslices":
		catalogTableEndpoint(fields, entity, obj)
	case "ingresses":
		catalogTableIngress(fields, entity, obj)
	case "ingressclasses":
		catalogTableIngressClass(fields, entity, obj)
	case "networkpolicies":
		catalogTableNetworkPolicy(fields, entity, obj)
	case "persistentvolumeclaims":
		catalogTablePVC(fields, entity, obj)
	case "persistentvolumes":
		catalogTablePV(fields, entity, obj)
	case "storageclasses":
		catalogTableStorageClass(fields, entity, obj)
	case "configmaps":
		catalogTableConfigMap(fields, entity, obj)
	case "secrets":
		catalogTableSecret(fields, entity, obj)
	case "horizontalpodautoscalers":
		catalogTableHPA(fields, entity, obj)
	case "poddisruptionbudgets":
		catalogTablePDB(fields, entity, obj)
	case "leases":
		catalogTableLease(fields, entity, obj)
	case "nodes":
		catalogTableNode(entity, fields, obj)
	case "namespaces":
		catalogTableNamespace(fields, obj)
	case "events":
		catalogTableEvent(fields, obj)
	case "serviceaccounts":
		catalogTableServiceAccount(fields, obj)
	case "roles", "clusterroles":
		catalogTableRole(fields, obj)
	case "rolebindings", "clusterrolebindings":
		catalogTableRoleBinding(fields, obj)
	case "resourcequotas":
		catalogTableResourceQuota(fields, obj)
	case "limitranges":
		catalogTableLimitRange(fields, obj)
	case "priorityclasses":
		catalogTablePriorityClass(fields, obj)
	case "runtimeclasses":
		catalogTableRuntimeClass(fields, obj)
	case "volumeattachments":
		catalogTableVolumeAttachment(fields, obj)
	case "csidrivers":
		catalogTableCSIDriver(fields, obj)
	case "csinodes":
		catalogTableCSINode(fields, obj)
	case "customresourcedefinitions":
		catalogTableCRD(fields, obj)
	case "apiservices":
		catalogTableAPIService(fields, obj)
	case "mutatingwebhookconfigurations", "validatingwebhookconfigurations":
		catalogTableWebhookConfig(fields, obj)
	case "validatingadmissionpolicies":
		catalogTableValidatingAdmissionPolicy(fields, obj)
	case "validatingadmissionpolicybindings":
		catalogTableValidatingAdmissionPolicyBinding(fields, obj)
	}
	if len(fields) > 0 {
		entity.TableFields = fields
	}
}

func setField(fields map[string]string, key, value string) {
	if value != "" {
		fields[key] = value
	}
}

func setFieldBool(fields map[string]string, key string, value *bool) {
	if value == nil {
		return
	}
	if *value {
		fields[key] = "true"
	} else {
		fields[key] = "false"
	}
}

func setFieldCount(fields map[string]string, key string, count *int32) {
	if count != nil {
		fields[key] = fmt.Sprintf("%d", *count)
	}
}

func catalogTablePod(fields map[string]string, entity *model.CatalogEntity, obj map[string]interface{}) {
	setField(fields, "status", entityStatusHint(obj, "pods"))
	if entity.RestartCount != nil {
		setField(fields, "restarts", fmt.Sprintf("%d", *entity.RestartCount))
	}
	setField(fields, "node", entity.NodeName)
	setField(fields, "qos", entity.QOSClass)
	catalogTableScheduling(entity, fields, podSpecMap(obj, "pods"))
}

func catalogTableReplicaController(fields map[string]string, entity *model.CatalogEntity, obj map[string]interface{}, resource string) {
	setField(fields, "status", entityStatusHint(obj, "deployments"))
	setFieldCount(fields, "desired", entity.DesiredReplicas)
	setFieldCount(fields, "current", entity.CurrentReplicas)
	setFieldCount(fields, "ready", entity.ReadyReplicas)
	if entity.ReadyReplicas != nil && entity.CurrentReplicas != nil {
		setField(fields, "pods", fmt.Sprintf("%d/%d", *entity.ReadyReplicas, *entity.CurrentReplicas))
	}
	if entity.CurrentReplicas != nil {
		setField(fields, "replicas", fmt.Sprintf("%d", *entity.CurrentReplicas))
	}
	catalogTableScheduling(entity, fields, podSpecMap(obj, resource))
}

func catalogTableDaemonSet(fields map[string]string, entity *model.CatalogEntity, obj map[string]interface{}) {
	setFieldCount(fields, "desired", entity.DesiredReplicas)
	setFieldCount(fields, "current", entity.CurrentReplicas)
	setFieldCount(fields, "ready", entity.ReadyReplicas)
	setFieldCount(fields, "updated", entity.UpdatedReplicas)
	setFieldCount(fields, "available", entity.AvailableReplicas)
	setFieldCount(fields, "misscheduled", entity.Misscheduled)
	catalogTableScheduling(entity, fields, podSpecMap(obj, "daemonsets"))
}

func catalogTableJob(fields map[string]string, entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity.Succeeded != nil && entity.Completions != nil {
		setField(fields, "completions", fmt.Sprintf("%d/%d", *entity.Succeeded, *entity.Completions))
	} else {
		setField(fields, "completions", entityStatusHint(obj, "jobs"))
	}
	setField(fields, "duration", entity.JobDuration)
	if len(entity.Conditions) > 0 {
		types := make([]string, 0, len(entity.Conditions))
		for _, cond := range entity.Conditions {
			if cond.Type != "" {
				types = append(types, cond.Type)
			}
		}
		setField(fields, "conditions", strings.Join(types, ", "))
	}
	catalogTableScheduling(entity, fields, podSpecMap(obj, "jobs"))
}

func catalogTableCronJob(fields map[string]string, entity *model.CatalogEntity, obj map[string]interface{}) {
	setField(fields, "schedule", entity.Schedule)
	if entity.Suspend != nil {
		setField(fields, "suspend", fmt.Sprintf("%t", *entity.Suspend))
	}
	setFieldCount(fields, "active", entity.ActiveJobs)
	setField(fields, "lastSchedule", entity.LastScheduleTime)
	catalogTableScheduling(entity, fields, podSpecMap(obj, "cronjobs"))
}

func catalogTableService(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "type", entity.ServiceType)
	setField(fields, "clusterIP", entity.ClusterIP)
	if len(entity.Ports) > 0 {
		setField(fields, "ports", strings.Join(entity.Ports, ", "))
	}
	if len(entity.ExternalIPs) > 0 {
		setField(fields, "externalIP", strings.Join(entity.ExternalIPs, ", "))
	}
	setField(fields, "selector", entity.Selector)
	if entity.ReadyEndpoints != nil && entity.TotalEndpoints != nil {
		setField(fields, "status", fmt.Sprintf("%d/%d ready", *entity.ReadyEndpoints, *entity.TotalEndpoints))
	}
}

func catalogTableEndpoint(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "endpoints", entity.EndpointSummary)
	setField(fields, "service", entity.ServiceName)
	setField(fields, "addressType", entity.AddressType)
}

func catalogTableIngress(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	if len(entity.LoadBalancers) > 0 {
		setField(fields, "loadBalancers", strings.Join(entity.LoadBalancers, ", "))
	}
	setField(fields, "rules", entity.IngressRulesSummary)
}

func catalogTableIngressClass(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "controller", entity.IngressController)
	setField(fields, "apiGroup", entity.ParameterAPIGroup)
	setField(fields, "scope", entity.ParameterScope)
	setField(fields, "parameterKind", entity.ParameterKind)
	setField(fields, "namespace", entity.ParameterNamespace)
}

func catalogTableNetworkPolicy(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	if len(entity.PolicyTypes) > 0 {
		setField(fields, "policyTypes", strings.Join(entity.PolicyTypes, ", "))
	}
}

func catalogTablePVC(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "status", entity.StatusHint)
	setField(fields, "volume", entity.VolumeName)
	setField(fields, "capacity", entity.Capacity)
	setField(fields, "accessModes", formatAccessModes(entity.AccessModes))
	setField(fields, "storageClass", entity.StorageClassName)
}

func catalogTablePV(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "status", entity.StatusHint)
	setField(fields, "capacity", entity.Capacity)
	setField(fields, "accessModes", formatAccessModes(entity.AccessModes))
	setField(fields, "reclaimPolicy", entity.ReclaimPolicy)
	setField(fields, "claim", entity.ClaimRef)
	setField(fields, "storageClass", entity.StorageClassName)
}

func catalogTableStorageClass(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "provisioner", entity.Provisioner)
	setField(fields, "reclaimPolicy", entity.ReclaimPolicy)
	setField(fields, "volumeBindingMode", entity.VolumeBindingMode)
	setFieldBool(fields, "defaultClass", entity.IsDefault)
}

func catalogTableConfigMap(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	if len(entity.ConfigMapData) > 0 {
		parts := make([]string, 0, len(entity.ConfigMapData))
		for _, e := range entity.ConfigMapData {
			parts = append(parts, fmt.Sprintf("%s (%d bytes)", e.Key, e.SizeBytes))
		}
		setField(fields, "dataKeys", strings.Join(parts, ", "))
	}
}

func catalogTableSecret(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "secretType", entity.SecretType)
	setFieldCount(fields, "keys", entity.DataKeys)
}

func catalogTableHPA(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "scaleTarget", entity.ScaleTarget)
	setField(fields, "targets", entity.MetricsSummary)
	setFieldCount(fields, "minPods", entity.MinReplicas)
	setFieldCount(fields, "maxPods", entity.MaxReplicas)
	if entity.CurrentReplicas != nil && entity.DesiredReplicas != nil {
		setField(fields, "hpaReplicas", fmt.Sprintf("%d/%d", *entity.CurrentReplicas, *entity.DesiredReplicas))
	}
}

func catalogTablePDB(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "minAvailable", entity.PDBMinAvailable)
	setField(fields, "maxUnavailable", entity.PDBMaxUnavailable)
	setFieldCount(fields, "allowedDisruptions", entity.PDBDisruptionsAllowed)
}

func catalogTableLease(fields map[string]string, entity *model.CatalogEntity, _ map[string]interface{}) {
	setField(fields, "holder", entity.LeaseHolder)
}

func catalogTableNode(entity *model.CatalogEntity, fields map[string]string, obj map[string]interface{}) {
	setField(fields, "status", catalogNodeReadyStatus(obj))
	setField(fields, "roles", catalogNodeRoles(obj))
	setField(fields, "version", stringFromObject(obj, "status", "nodeInfo", "kubeletVersion"))
	applyCatalogNodeTaints(entity, fields, obj)
}

func catalogNodeReadyStatus(obj map[string]interface{}) string {
	conds, _, _ := unstructured.NestedSlice(obj, "status", "conditions")
	for _, item := range conds {
		cm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if stringFromMap(cm, "type") == "Ready" {
			return stringFromMap(cm, "status")
		}
	}
	return ""
}

func catalogNodeRoles(obj map[string]interface{}) string {
	labels, _, _ := unstructured.NestedStringMap(obj, "metadata", "labels")
	if len(labels) == 0 {
		return ""
	}
	var roles []string
	for k := range labels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			roles = append(roles, strings.TrimPrefix(k, "node-role.kubernetes.io/"))
		}
	}
	sort.Strings(roles)
	return strings.Join(roles, ", ")
}

func catalogTableNamespace(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "status", stringFromObject(obj, "status", "phase"))
}

func catalogTableEvent(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "type", stringFromObject(obj, "type"))
	setField(fields, "reason", stringFromObject(obj, "reason"))
	invKind := stringFromObject(obj, "involvedObject", "kind")
	invName := stringFromObject(obj, "involvedObject", "name")
	if invKind != "" && invName != "" {
		setField(fields, "object", invKind+"/"+invName)
	}
	msg := stringFromObject(obj, "message")
	if len(msg) > 120 {
		msg = msg[:117] + "..."
	}
	setField(fields, "message", msg)
	setField(fields, "lastSeen", firstStringFromObject(obj,
		[]string{"lastTimestamp"},
		[]string{"eventTime"},
		[]string{"series", "lastObservedTime"},
	))
}

func catalogTableServiceAccount(fields map[string]string, obj map[string]interface{}) {
	secrets, _, _ := unstructured.NestedSlice(obj, "secrets")
	setField(fields, "secrets", fmt.Sprintf("%d", len(secrets)))
}

func catalogTableRole(fields map[string]string, obj map[string]interface{}) {
	rules, _, _ := unstructured.NestedSlice(obj, "rules")
	setField(fields, "rules", fmt.Sprintf("%d", len(rules)))
}

func catalogTableRoleBinding(fields map[string]string, obj map[string]interface{}) {
	kind := stringFromObject(obj, "roleRef", "kind")
	name := stringFromObject(obj, "roleRef", "name")
	if kind != "" && name != "" {
		setField(fields, "role", kind+"/"+name)
	}
	subjects, _, _ := unstructured.NestedSlice(obj, "subjects")
	setField(fields, "subjects", fmt.Sprintf("%d", len(subjects)))
}

func catalogTableResourceQuota(fields map[string]string, obj map[string]interface{}) {
	hard, _, _ := unstructured.NestedStringMap(obj, "status", "hard")
	used, _, _ := unstructured.NestedStringMap(obj, "status", "used")
	if len(hard) == 0 {
		return
	}
	parts := make([]string, 0, len(hard))
	for k, h := range hard {
		u := used[k]
		if u != "" {
			parts = append(parts, fmt.Sprintf("%s:%s/%s", k, u, h))
		} else {
			parts = append(parts, fmt.Sprintf("%s:%s", k, h))
		}
	}
	sort.Strings(parts)
	setField(fields, "hard", strings.Join(parts, ", "))
}

func catalogTableLimitRange(fields map[string]string, obj map[string]interface{}) {
	limits, _, _ := unstructured.NestedSlice(obj, "spec", "limits")
	setField(fields, "limits", fmt.Sprintf("%d", len(limits)))
}

func catalogTablePriorityClass(fields map[string]string, obj map[string]interface{}) {
	if v := int32FromObject(obj, "value"); v != nil {
		setField(fields, "value", fmt.Sprintf("%d", *v))
	}
	if v, found, _ := unstructured.NestedBool(obj, "globalDefault"); found {
		setField(fields, "globalDefault", fmt.Sprintf("%t", v))
	}
}

func catalogTableRuntimeClass(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "handler", stringFromObject(obj, "handler"))
}

func catalogTableVolumeAttachment(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "attacher", stringFromObject(obj, "spec", "attacher"))
	setField(fields, "node", stringFromObject(obj, "spec", "nodeName"))
	setField(fields, "pv", stringFromObject(obj, "spec", "source", "persistentVolumeName"))
	setField(fields, "status", catalogVolumeAttachmentStatus(obj))
}

func catalogVolumeAttachmentStatus(obj map[string]interface{}) string {
	if v := stringFromObject(obj, "status", "attachError", "message"); v != "" {
		return "Error"
	}
	if attached, found, _ := unstructured.NestedBool(obj, "status", "attached"); found {
		if attached {
			return "Attached"
		}
		return "Detached"
	}
	return ""
}

func catalogTableCSIDriver(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "attachRequired", catalogBoolString(obj, "spec", "attachRequired"))
	setField(fields, "podInfoOnMount", catalogBoolString(obj, "spec", "podInfoOnMount"))
	setField(fields, "storageCapacity", catalogBoolString(obj, "spec", "storageCapacity"))
}

func catalogBoolString(obj map[string]interface{}, fields ...string) string {
	if v, found, err := unstructured.NestedBool(obj, fields...); found && err == nil {
		return fmt.Sprintf("%t", v)
	}
	return ""
}

func catalogTableCSINode(fields map[string]string, obj map[string]interface{}) {
	drivers, _, _ := unstructured.NestedSlice(obj, "spec", "drivers")
	setField(fields, "drivers", fmt.Sprintf("%d", len(drivers)))
}

func catalogTableCRD(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "group", stringFromObject(obj, "spec", "group"))
	versions, _, _ := unstructured.NestedSlice(obj, "spec", "versions")
	if len(versions) > 0 {
		if vm, ok := versions[0].(map[string]interface{}); ok {
			setField(fields, "version", stringFromMap(vm, "name"))
		}
	}
	setField(fields, "scope", stringFromObject(obj, "spec", "scope"))
}

func catalogTableAPIService(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "service", catalogAPIServiceName(obj))
	setField(fields, "available", catalogAPIServiceAvailable(obj))
}

func catalogAPIServiceName(obj map[string]interface{}) string {
	name := stringFromObject(obj, "spec", "service", "name")
	ns := stringFromObject(obj, "spec", "service", "namespace")
	if name == "" {
		return ""
	}
	if ns != "" {
		return ns + "/" + name
	}
	return name
}

func catalogAPIServiceAvailable(obj map[string]interface{}) string {
	conds, _, _ := unstructured.NestedSlice(obj, "status", "conditions")
	for _, item := range conds {
		cm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if stringFromMap(cm, "type") == "Available" {
			return stringFromMap(cm, "status")
		}
	}
	return ""
}

func catalogTableWebhookConfig(fields map[string]string, obj map[string]interface{}) {
	hooks, _, _ := unstructured.NestedSlice(obj, "webhooks")
	setField(fields, "webhooks", fmt.Sprintf("%d", len(hooks)))
}

func catalogTableValidatingAdmissionPolicy(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "failurePolicy", stringFromObject(obj, "spec", "failurePolicy"))
	setField(fields, "matchConstraints", catalogValidationActions(obj))
}

func catalogValidationActions(obj map[string]interface{}) string {
	// count match constraints resource rules if present
	rules, _, _ := unstructured.NestedSlice(obj, "spec", "matchConstraints", "resourceRules")
	return fmt.Sprintf("%d rules", len(rules))
}

func catalogTableValidatingAdmissionPolicyBinding(fields map[string]string, obj map[string]interface{}) {
	setField(fields, "policy", stringFromObject(obj, "spec", "policyName"))
	setField(fields, "validationActions", strings.Join(catalogStringSlice(obj, "spec", "validationActions"), ", "))
}

func catalogStringSlice(obj map[string]interface{}, fields ...string) []string {
	raw, found, err := unstructured.NestedStringSlice(obj, fields...)
	if !found || err != nil {
		return nil
	}
	return raw
}
