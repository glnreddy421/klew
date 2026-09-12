package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/glnreddy421/klew/internal/model"
)

const catalogEntityListLimit = 500
const catalogEntityMultiTotalLimit = 2000

// CatalogEntityScope selects which namespace(s) to list entities from.
type CatalogEntityScope struct {
	Namespace     string
	AllNamespaces bool
	Namespaces    []string
	ClusterScoped bool
}

// ListCatalogEntities lists lightweight entity references for a discovered GVR.
func ListCatalogEntities(ctx context.Context, client *Client, resourceID string, scope CatalogEntityScope) (model.CatalogEntityList, error) {
	empty := model.CatalogEntityList{AccessState: model.ResourceAccessUnknown}
	if client == nil || client.Clientset == nil {
		return empty, fmt.Errorf("kubernetes client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	switch resourceID {
	case VirtualHelmReleasesResourceID:
		return ListHelmReleaseEntities(ctx, client, scope)
	}
	group, version, resource, err := ParseResourceID(resourceID)
	if err != nil {
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}, nil
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}

	if scope.ClusterScoped {
		return listCatalogEntitiesOnce(ctx, client, gvr, resourceID, resource, "")
	}
	if scope.AllNamespaces {
		return listCatalogEntitiesOnce(ctx, client, gvr, resourceID, resource, "")
	}
	if len(scope.Namespaces) > 1 {
		return listCatalogEntitiesMulti(ctx, client, gvr, resourceID, resource, scope.Namespaces)
	}
	ns := scope.Namespace
	if ns == "" && len(scope.Namespaces) == 1 {
		ns = scope.Namespaces[0]
	}
	return listCatalogEntitiesOnce(ctx, client, gvr, resourceID, resource, ns)
}

func listCatalogEntitiesMulti(ctx context.Context, client *Client, gvr schema.GroupVersionResource, resourceID, resource string, namespaces []string) (model.CatalogEntityList, error) {
	seen := make(map[string]struct{})
	out := make([]model.CatalogEntity, 0)
	var lastErr string
	allowed := 0
	forbidden := 0
	for _, ns := range namespaces {
		if ctx.Err() != nil {
			break
		}
		part, err := listCatalogEntitiesOnce(ctx, client, gvr, resourceID, resource, ns)
		if err != nil {
			return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}, err
		}
		switch part.AccessState {
		case model.ResourceAccessAllowed:
			allowed++
			for _, entity := range part.Entities {
				key := entity.UID
				if key == "" {
					key = entity.Namespace + "/" + entity.Name
				}
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, entity)
				if len(out) >= catalogEntityMultiTotalLimit {
					break
				}
			}
		case model.ResourceAccessForbidden:
			forbidden++
		default:
			if part.Error != "" {
				lastErr = part.Error
			}
		}
		if len(out) >= catalogEntityMultiTotalLimit {
			break
		}
	}
	if allowed == 0 && forbidden > 0 {
		return model.CatalogEntityList{AccessState: model.ResourceAccessForbidden}, nil
	}
	if allowed == 0 && lastErr != "" {
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: lastErr}, nil
	}
	if allowed == 0 {
		return model.CatalogEntityList{AccessState: model.ResourceAccessUnavailable}, nil
	}
	return model.CatalogEntityList{
		Entities:    out,
		AccessState: model.ResourceAccessAllowed,
	}, nil
}

func listCatalogEntitiesOnce(ctx context.Context, client *Client, gvr schema.GroupVersionResource, resourceID, resource, namespace string) (model.CatalogEntityList, error) {
	reqCtx, cancel := context.WithTimeout(ctx, catalogRequestTimeout)
	defer cancel()

	dyn, err := dynamicClient(client)
	if err != nil {
		return model.CatalogEntityList{AccessState: model.ResourceAccessUnknown}, err
	}
	var res dynamic.ResourceInterface
	if namespace != "" {
		res = dyn.Resource(gvr).Namespace(namespace)
	} else {
		res = dyn.Resource(gvr)
	}
	ul, err := res.List(reqCtx, metav1.ListOptions{Limit: catalogEntityListLimit})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return model.CatalogEntityList{AccessState: model.ResourceAccessForbidden}, nil
		}
		if apierrors.IsNotFound(err) {
			return model.CatalogEntityList{AccessState: model.ResourceAccessUnavailable}, nil
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || reqCtx.Err() != nil {
			return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: "list timed out (API throttling or slow cluster)"}, nil
		}
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}, nil
	}
	var serviceEndpointCounts map[string]serviceEndpointCount
	if resource == "services" && namespace != "" && client.Clientset != nil {
		serviceEndpointCounts = catalogServiceEndpointCountsByName(reqCtx, client, namespace)
	}

	out := make([]model.CatalogEntity, 0, len(ul.Items))
	for _, item := range ul.Items {
		entity := model.CatalogEntity{
			ResourceID:      resourceID,
			Name:            item.GetName(),
			Namespace:       item.GetNamespace(),
			UID:             string(item.GetUID()),
			ResourceVersion: item.GetResourceVersion(),
			Kind:            item.GetKind(),
			APIVersion:      item.GetAPIVersion(),
			StatusHint:      entityStatusHint(item.Object, resource),
			NodeName:        entityNodeName(item.Object, resource),
		}
		if ts := item.GetCreationTimestamp(); !ts.IsZero() {
			entity.CreationTimestamp = ts.Format(time.RFC3339)
		}
		if entity.Kind == "" {
			entity.Kind = stringFromObject(item.Object, "kind")
		}
		if entity.Kind == "" {
			entity.Kind = catalogKindForResource(resource)
		}
		switch resource {
		case "pods":
			enrichPodCatalogEntity(&entity, item.Object)
		case "deployments":
			enrichWorkloadCatalogEntity(&entity, item.Object)
			enrichDeploymentCatalogEntity(&entity, item.Object)
		case "statefulsets", "replicasets":
			enrichWorkloadCatalogEntity(&entity, item.Object)
		case "daemonsets":
			enrichDaemonSetCatalogEntity(&entity, item.Object)
		case "jobs":
			enrichJobCatalogEntity(&entity, item.Object)
		case "cronjobs":
			enrichCronJobCatalogEntity(&entity, item.Object)
		case "services":
			enrichServiceCatalogEntity(&entity, item.Object)
			if counts, ok := serviceEndpointCounts[entity.Name]; ok {
				applyServiceEndpointCounts(&entity, counts)
			}
		case "endpoints":
			enrichEndpointsCatalogEntity(&entity, item.Object)
		case "endpointslices":
			enrichEndpointSliceCatalogEntity(&entity, item.Object)
		case "ingresses":
			enrichIngressCatalogEntity(&entity, item.Object)
		case "ingressclasses":
			enrichIngressClassCatalogEntity(&entity, item.Object)
		case "networkpolicies":
			enrichNetworkPolicyCatalogEntity(&entity, item.Object)
		case "storageclasses":
			enrichStorageClassCatalogEntity(&entity, item.Object)
		case "nodes":
			enrichNodeCatalogEntity(&entity, item.Object)
		default:
			enrichBuiltinCatalogEntity(resource, &entity, item.Object)
		}
		enrichCatalogTableFields(&entity, item.Object, resource)
		out = append(out, entity)
	}
	if resource == "pods" && len(out) > 0 {
		enrichListedPodMetrics(reqCtx, client, out, namespace)
	}
	return model.CatalogEntityList{
		Entities:    out,
		AccessState: model.ResourceAccessAllowed,
	}, nil
}

func entityStatusHint(obj map[string]interface{}, resource string) string {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return ""
	}
	if phase, ok := status["phase"]; ok {
		return fmt.Sprintf("%v", phase)
	}
	switch resource {
	case "deployments", "statefulsets", "replicasets":
		ready := status["readyReplicas"]
		total := status["replicas"]
		if ready != nil && total != nil {
			return fmt.Sprintf("%v/%v ready", ready, total)
		}
	case "daemonsets":
		ready := status["numberReady"]
		total := status["desiredNumberScheduled"]
		if ready != nil && total != nil {
			return fmt.Sprintf("%v/%v ready", ready, total)
		}
	case "jobs":
		succeeded := int32FromObject(obj, "status", "succeeded")
		completions := jobCompletionsTarget(obj)
		if succeeded != nil && completions != nil {
			return fmt.Sprintf("%d/%d", *succeeded, *completions)
		}
		if succeeded, ok := status["succeeded"]; ok {
			return fmt.Sprintf("%v succeeded", succeeded)
		}
	case "cronjobs":
		if schedule := stringFromObject(obj, "spec", "schedule"); schedule != "" {
			return schedule
		}
	}
	return ""
}

func entityNodeName(obj map[string]interface{}, resource string) string {
	if resource != "pods" {
		return ""
	}
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return ""
	}
	if node, ok := spec["nodeName"].(string); ok {
		return node
	}
	return ""
}

type serviceEndpointCount struct {
	ready int32
	total int32
}

func enrichServiceCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.ServiceType = stringFromObject(obj, "spec", "type")
	if entity.ServiceType == "" {
		entity.ServiceType = "ClusterIP"
	}
	entity.ClusterIP = stringFromObject(obj, "spec", "clusterIP")
	entity.Selector = catalogSelectorString(obj)
	entity.Ports = catalogServicePorts(obj)
	entity.ExternalIPs = catalogServiceExternalIPs(obj)
	if entity.ServiceType == "ExternalName" {
		entity.StatusHint = "External"
	}
}

func applyServiceEndpointCounts(entity *model.CatalogEntity, counts serviceEndpointCount) {
	if entity == nil {
		return
	}
	entity.ReadyEndpoints = &counts.ready
	entity.TotalEndpoints = &counts.total
	entity.StatusHint = serviceEndpointStatusHint(entity.ServiceType, counts.ready, counts.total)
}

func serviceEndpointStatusHint(svcType string, ready, total int32) string {
	if svcType == "ExternalName" {
		return "External"
	}
	if total == 0 {
		return "No endpoints"
	}
	return fmt.Sprintf("%d/%d ready", ready, total)
}

func catalogServiceEndpointCountsByName(ctx context.Context, client *Client, namespace string) map[string]serviceEndpointCount {
	out := make(map[string]serviceEndpointCount)
	if client == nil || client.Clientset == nil || namespace == "" {
		return out
	}
	slices, err := client.Clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return out
	}
	for _, sl := range slices.Items {
		svcName := sl.Labels[discoveryv1.LabelServiceName]
		if svcName == "" {
			continue
		}
		ready, total := countEndpointSliceAddresses([]discoveryv1.EndpointSlice{sl})
		cur := out[svcName]
		cur.ready += ready
		cur.total += total
		out[svcName] = cur
	}
	return out
}

func countEndpointSliceAddresses(slices []discoveryv1.EndpointSlice) (ready, total int32) {
	for _, sl := range slices {
		for _, ep := range sl.Endpoints {
			n := int32(len(ep.Addresses))
			if n == 0 {
				continue
			}
			total += n
			isReady := ep.Conditions.Ready == nil || *ep.Conditions.Ready
			if isReady {
				ready += n
			}
		}
	}
	return ready, total
}

func enrichEndpointsCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	subsets, _ := obj["subsets"].([]interface{})
	ready, total, summary := catalogLegacyEndpointSubsets(subsets)
	entity.ReadyEndpoints = &ready
	entity.TotalEndpoints = &total
	entity.EndpointSummary = summary
	if summary != "" {
		entity.StatusHint = summary
	} else if total == 0 {
		entity.StatusHint = "<none>"
	}
}

func enrichEndpointSliceCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		if labels, ok := meta["labels"].(map[string]interface{}); ok {
			if svc, ok := labels["kubernetes.io/service-name"].(string); ok {
				entity.ServiceName = svc
			}
		}
	}
	entity.AddressType = stringFromObject(obj, "addressType")
	endpoints, _ := obj["endpoints"].([]interface{})
	ports, _ := obj["ports"].([]interface{})
	ready, total, summary := catalogEndpointSliceSummary(endpoints, ports)
	entity.ReadyEndpoints = &ready
	entity.TotalEndpoints = &total
	entity.EndpointSummary = summary
	if summary != "" {
		entity.StatusHint = summary
	} else if total == 0 {
		entity.StatusHint = "<none>"
	}
}

func catalogLegacyEndpointSubsets(subsets []interface{}) (ready, total int32, summary string) {
	var parts []string
	for _, item := range subsets {
		sub, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ports := catalogLegacyEndpointPorts(sub)
		countReady := func(addrs []interface{}, isReady bool) {
			for _, item := range addrs {
				am, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				ip, _ := am["ip"].(string)
				if ip == "" {
					continue
				}
				total++
				if isReady {
					ready++
				}
				parts = append(parts, catalogEndpointIPPorts(ip, ports)...)
			}
		}
		if addrs, ok := sub["addresses"].([]interface{}); ok {
			countReady(addrs, true)
		}
		if addrs, ok := sub["notReadyAddresses"].([]interface{}); ok {
			countReady(addrs, false)
		}
	}
	return ready, total, strings.Join(parts, ",")
}

func catalogLegacyEndpointPorts(sub map[string]interface{}) []int32 {
	portsRaw, ok := sub["ports"].([]interface{})
	if !ok {
		return nil
	}
	var ports []int32
	for _, item := range portsRaw {
		pm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if p := int32FromMap(pm, "port"); p != nil {
			ports = append(ports, *p)
		}
	}
	return ports
}

func catalogEndpointSliceSummary(endpoints, portsRaw []interface{}) (ready, total int32, summary string) {
	ports := catalogEndpointSlicePorts(portsRaw)
	var parts []string
	for _, item := range endpoints {
		ep, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		isReady := true
		if cond, ok := ep["conditions"].(map[string]interface{}); ok {
			if readyVal, ok := cond["ready"].(bool); ok {
				isReady = readyVal
			}
		}
		addrs, _ := ep["addresses"].([]interface{})
		for _, addrItem := range addrs {
			ip, _ := addrItem.(string)
			if ip == "" {
				continue
			}
			total++
			if isReady {
				ready++
			}
			parts = append(parts, catalogEndpointIPPorts(ip, ports)...)
		}
	}
	return ready, total, strings.Join(parts, ",")
}

func catalogEndpointSlicePorts(portsRaw []interface{}) []int32 {
	var ports []int32
	for _, item := range portsRaw {
		pm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if p := int32FromMap(pm, "port"); p != nil {
			ports = append(ports, *p)
		}
	}
	return ports
}

func catalogEndpointIPPorts(ip string, ports []int32) []string {
	if len(ports) == 0 {
		return []string{ip}
	}
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		out = append(out, fmt.Sprintf("%s:%d", ip, p))
	}
	return out
}

func enrichIngressCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.LoadBalancers = catalogIngressLoadBalancers(obj)
	entity.IngressRulesSummary = catalogIngressRulesSummary(obj)
	if len(entity.LoadBalancers) > 0 {
		entity.StatusHint = strings.Join(entity.LoadBalancers, ", ")
	} else if entity.IngressRulesSummary != "" {
		entity.StatusHint = entity.IngressRulesSummary
	}
}

func catalogIngressLoadBalancers(obj map[string]interface{}) []string {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return nil
	}
	lb, ok := status["loadBalancer"].(map[string]interface{})
	if !ok {
		return nil
	}
	ingress, ok := lb["ingress"].([]interface{})
	if !ok {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	for _, item := range ingress {
		im, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		add(stringFromMap(im, "ip"))
		add(stringFromMap(im, "hostname"))
	}
	return out
}

func catalogIngressRulesSummary(obj map[string]interface{}) string {
	rules, ok := obj["spec"].(map[string]interface{})["rules"].([]interface{})
	if !ok || len(rules) == 0 {
		return ""
	}
	var parts []string
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		host := stringFromMap(rule, "host")
		http, ok := rule["http"].(map[string]interface{})
		if !ok {
			if host != "" {
				parts = append(parts, host)
			}
			continue
		}
		paths, ok := http["paths"].([]interface{})
		if !ok || len(paths) == 0 {
			if host != "" {
				parts = append(parts, host)
			}
			continue
		}
		for _, pathItem := range paths {
			pm, ok := pathItem.(map[string]interface{})
			if !ok {
				continue
			}
			path := stringFromMap(pm, "path")
			label := host
			if label == "" {
				label = "*"
			}
			if path != "" {
				label += path
			} else {
				label += "/*"
			}
			parts = append(parts, label)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return fmt.Sprintf("%s +%d", parts[0], len(parts)-1)
}

func enrichStorageClassCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	// StorageClass stores provisioner/reclaimPolicy/volumeBindingMode at the root — not under spec.
	entity.Provisioner = catalogStorageClassField(obj, "provisioner")
	entity.ReclaimPolicy = catalogStorageClassField(obj, "reclaimPolicy")
	entity.VolumeBindingMode = catalogStorageClassField(obj, "volumeBindingMode")
	entity.IsDefault = catalogStorageClassDefaultValue(obj)
	if entity.Provisioner != "" {
		entity.StatusHint = entity.Provisioner
	} else if entity.ReclaimPolicy != "" {
		entity.StatusHint = entity.ReclaimPolicy
	}
}

func catalogStorageClassField(obj map[string]interface{}, key string) string {
	if v := stringFromObject(obj, key); v != "" {
		return v
	}
	return stringFromObject(obj, "spec", key)
}

func catalogKindForResource(resource string) string {
	switch resource {
	case "pods":
		return "Pod"
	case "deployments":
		return "Deployment"
	case "replicasets":
		return "ReplicaSet"
	case "statefulsets":
		return "StatefulSet"
	case "daemonsets":
		return "DaemonSet"
	case "jobs":
		return "Job"
	case "cronjobs":
		return "CronJob"
	case "services":
		return "Service"
	case "endpoints":
		return "Endpoints"
	case "endpointslices":
		return "EndpointSlice"
	case "ingresses":
		return "Ingress"
	case "ingressclasses":
		return "IngressClass"
	case "networkpolicies":
		return "NetworkPolicy"
	case "storageclasses":
		return "StorageClass"
	case "persistentvolumes":
		return "PersistentVolume"
	case "persistentvolumeclaims":
		return "PersistentVolumeClaim"
	case "replicationcontrollers":
		return "ReplicationController"
	case "configmaps":
		return "ConfigMap"
	case "secrets":
		return "Secret"
	case "horizontalpodautoscalers":
		return "HorizontalPodAutoscaler"
	case "poddisruptionbudgets":
		return "PodDisruptionBudget"
	case "leases":
		return "Lease"
	case "nodes":
		return "Node"
	case "namespaces":
		return "Namespace"
	case "events":
		return "Event"
	case "serviceaccounts":
		return "ServiceAccount"
	case "roles":
		return "Role"
	case "clusterroles":
		return "ClusterRole"
	case "rolebindings":
		return "RoleBinding"
	case "clusterrolebindings":
		return "ClusterRoleBinding"
	case "resourcequotas":
		return "ResourceQuota"
	case "limitranges":
		return "LimitRange"
	case "priorityclasses":
		return "PriorityClass"
	case "runtimeclasses":
		return "RuntimeClass"
	case "volumeattachments":
		return "VolumeAttachment"
	case "csidrivers":
		return "CSIDriver"
	case "csinodes":
		return "CSINode"
	case "csistoragecapacities":
		return "CSIStorageCapacity"
	case "volumeattributesclasses":
		return "VolumeAttributesClass"
	case "customresourcedefinitions":
		return "CustomResourceDefinition"
	case "apiservices":
		return "APIService"
	case "mutatingwebhookconfigurations":
		return "MutatingWebhookConfiguration"
	case "validatingwebhookconfigurations":
		return "ValidatingWebhookConfiguration"
	case "validatingadmissionpolicies":
		return "ValidatingAdmissionPolicy"
	case "validatingadmissionpolicybindings":
		return "ValidatingAdmissionPolicyBinding"
	default:
		return ""
	}
}

func catalogStorageClassDefaultValue(obj map[string]interface{}) *bool {
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	ann, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		return nil
	}
	for _, key := range []string{
		"storageclass.kubernetes.io/is-default-class",
		"storageclass.beta.kubernetes.io/is-default-class",
	} {
		v, ok := ann[key].(string)
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true":
			t := true
			return &t
		case "false":
			f := false
			return &f
		}
	}
	return nil
}

func enrichNetworkPolicyCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return
	}
	entity.PolicyTypes = catalogNetworkPolicyTypes(spec)
	if len(entity.PolicyTypes) > 0 {
		entity.StatusHint = strings.Join(entity.PolicyTypes, ", ")
	}
}

func catalogNetworkPolicyTypes(spec map[string]interface{}) []string {
	if types, ok := spec["policyTypes"].([]interface{}); ok && len(types) > 0 {
		out := make([]string, 0, len(types))
		for _, item := range types {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	var out []string
	if ingress, ok := spec["ingress"].([]interface{}); ok && len(ingress) > 0 {
		out = append(out, "Ingress")
	}
	if egress, ok := spec["egress"].([]interface{}); ok && len(egress) > 0 {
		out = append(out, "Egress")
	}
	return out
}

func enrichIngressClassCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return
	}
	entity.IngressController = stringFromMap(spec, "controller")
	params, ok := spec["parameters"].(map[string]interface{})
	if !ok {
		if entity.IngressController != "" {
			entity.StatusHint = entity.IngressController
		}
		return
	}
	entity.ParameterAPIGroup = stringFromMap(params, "apiGroup")
	entity.ParameterScope = stringFromMap(params, "scope")
	entity.ParameterKind = stringFromMap(params, "kind")
	entity.ParameterNamespace = stringFromMap(params, "namespace")
	if entity.IngressController != "" {
		entity.StatusHint = entity.IngressController
	}
}

func catalogSelectorString(obj map[string]interface{}) string {
	sel, ok := obj["spec"].(map[string]interface{})["selector"].(map[string]interface{})
	if !ok || len(sel) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sel))
	for k, v := range sel {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func catalogServicePorts(obj map[string]interface{}) []string {
	portsRaw, ok := obj["spec"].(map[string]interface{})["ports"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(portsRaw))
	for _, item := range portsRaw {
		pm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		port := int32FromMap(pm, "port")
		proto := stringFromMap(pm, "protocol")
		if proto == "" {
			proto = "TCP"
		}
		if port == nil {
			continue
		}
		label := fmt.Sprintf("%d/%s", *port, proto)
		if name := stringFromMap(pm, "name"); name != "" {
			label = name + ":" + label
		}
		out = append(out, label)
	}
	return out
}

func catalogServiceExternalIPs(obj map[string]interface{}) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			return
		}
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if ips, ok := spec["externalIPs"].([]interface{}); ok {
			for _, item := range ips {
				add(fmt.Sprintf("%v", item))
			}
		}
		if ext := stringFromMap(spec, "externalName"); ext != "" {
			add(ext)
		}
		if lb := stringFromMap(spec, "loadBalancerIP"); lb != "" {
			add(lb)
		}
	}
	if status, ok := obj["status"].(map[string]interface{}); ok {
		if lb, ok := status["loadBalancer"].(map[string]interface{}); ok {
			if ingress, ok := lb["ingress"].([]interface{}); ok {
				for _, item := range ingress {
					im, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					add(stringFromMap(im, "ip"))
					add(stringFromMap(im, "hostname"))
				}
			}
		}
	}
	return out
}

func enrichDaemonSetCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.DesiredReplicas = int32FromObject(obj, "status", "desiredNumberScheduled")
	entity.CurrentReplicas = int32FromObject(obj, "status", "currentNumberScheduled")
	entity.ReadyReplicas = int32FromObject(obj, "status", "numberReady")
	entity.UpdatedReplicas = int32FromObject(obj, "status", "updatedNumberScheduled")
	entity.AvailableReplicas = int32FromObject(obj, "status", "numberAvailable")
	entity.Misscheduled = int32FromObject(obj, "status", "numberMisscheduled")
	if entity.Misscheduled == nil && entity.DesiredReplicas != nil {
		zero := int32(0)
		entity.Misscheduled = &zero
	}
}

func enrichDeploymentCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.AvailableReplicas = int32FromObject(obj, "status", "availableReplicas")
	entity.UpdatedReplicas = int32FromObject(obj, "status", "updatedReplicas")
	entity.UnavailableReplicas = int32FromObject(obj, "status", "unavailableReplicas")
	enrichCatalogConditions(entity, obj)
}

func enrichJobCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.Succeeded = int32FromObject(obj, "status", "succeeded")
	entity.Completions = jobCompletionsTarget(obj)
	entity.JobDuration = jobDurationHint(obj)
	enrichCatalogConditions(entity, obj)
}

func jobDurationHint(obj map[string]interface{}) string {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return ""
	}
	startRaw := stringFromMap(status, "startTime")
	if startRaw == "" {
		return ""
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return ""
	}
	end := time.Now()
	if completionRaw := stringFromMap(status, "completionTime"); completionRaw != "" {
		if completion, err := time.Parse(time.RFC3339, completionRaw); err == nil {
			end = completion
		}
	}
	return formatCatalogDuration(end.Sub(start))
}

func formatCatalogDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int(d.Seconds())
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	min := sec / 60
	if min < 60 {
		return fmt.Sprintf("%dm", min)
	}
	hr := min / 60
	if hr < 48 {
		return fmt.Sprintf("%dh", hr)
	}
	return fmt.Sprintf("%dd", hr/24)
}

func jobCompletionsTarget(obj map[string]interface{}) *int32 {
	if completions := int32FromObject(obj, "spec", "completions"); completions != nil {
		return completions
	}
	one := int32(1)
	return &one
}

func enrichCronJobCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.Schedule = stringFromObject(obj, "spec", "schedule")
	if suspend := boolFromObject(obj, "spec", "suspend"); suspend != nil {
		entity.Suspend = suspend
	} else {
		f := false
		entity.Suspend = &f
	}
	active := int32(0)
	if status, ok := obj["status"].(map[string]interface{}); ok {
		if raw, ok := status["active"].([]interface{}); ok {
			active = int32(len(raw))
		}
		entity.LastScheduleTime = stringFromMap(status, "lastScheduleTime")
	}
	entity.ActiveJobs = &active
}

func enrichCatalogConditions(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return
	}
	raw, ok := status["conditions"].([]interface{})
	if !ok {
		return
	}
	for _, item := range raw {
		cm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cond := model.CatalogCondition{
			Type:    stringFromMap(cm, "type"),
			Status:  stringFromMap(cm, "status"),
			Reason:  stringFromMap(cm, "reason"),
			Message: stringFromMap(cm, "message"),
		}
		if cond.Type != "" {
			entity.Conditions = append(entity.Conditions, cond)
		}
	}
}

func stringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func stringFromObject(obj map[string]interface{}, fields ...string) string {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

func boolFromObject(obj map[string]interface{}, fields ...string) *bool {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil || val == nil {
		return nil
	}
	if b, ok := val.(bool); ok {
		return &b
	}
	return nil
}

func enrichWorkloadCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	entity.DesiredReplicas = int32FromObject(obj, "spec", "replicas")
	entity.CurrentReplicas = int32FromObject(obj, "status", "replicas")
	entity.ReadyReplicas = int32FromObject(obj, "status", "readyReplicas")
	if entity.CurrentReplicas != nil && *entity.CurrentReplicas == 0 && entity.ReadyReplicas == nil {
		zero := int32(0)
		entity.ReadyReplicas = &zero
	}
}

func int32FromObject(obj map[string]interface{}, fields ...string) *int32 {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil || val == nil {
		return nil
	}
	return coerceInt32(val)
}

func int32FromMap(m map[string]interface{}, key string) *int32 {
	if m == nil {
		return nil
	}
	return coerceInt32(m[key])
}

func coerceInt32(v interface{}) *int32 {
	switch n := v.(type) {
	case int32:
		return &n
	case int:
		i := int32(n)
		return &i
	case int64:
		i := int32(n)
		return &i
	case float64:
		i := int32(n)
		return &i
	case float32:
		i := int32(n)
		return &i
	case uint64:
		i := int32(n)
		return &i
	case json.Number:
		if i64, err := n.Int64(); err == nil {
			i := int32(i64)
			return &i
		}
	}
	return nil
}

func enrichPodCatalogEntity(entity *model.CatalogEntity, obj map[string]interface{}) {
	if entity == nil || obj == nil {
		return
	}
	reqCPU, reqMem, limCPU, limMem := podResourceTotalsFromObject(obj)
	if reqCPU > 0 {
		v := reqCPU
		entity.CPURequestMilli = &v
	}
	if limCPU > 0 {
		v := limCPU
		entity.CPULimitMilli = &v
	}
	if reqMem > 0 {
		v := reqMem
		entity.MemRequestMi = &v
	}
	if limMem > 0 {
		v := limMem
		entity.MemLimitMi = &v
	}
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if containers, ok := spec["containers"].([]interface{}); ok {
			for _, raw := range containers {
				cm, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if name, ok := cm["name"].(string); ok && name != "" {
					entity.ContainerNames = append(entity.ContainerNames, name)
				}
			}
		}
	}
	if status, ok := obj["status"].(map[string]interface{}); ok {
		if qos, ok := status["qosClass"].(string); ok {
			entity.QOSClass = qos
		}
		var restarts int32
		if initStatuses, ok := status["initContainerStatuses"].([]interface{}); ok {
			for _, raw := range initStatuses {
				cs, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if parsed, ok := parseCatalogContainerStatus(cs, true); ok {
					entity.Containers = append(entity.Containers, parsed)
				}
				restarts += containerRestartCount(cs)
			}
		}
		if containerStatuses, ok := status["containerStatuses"].([]interface{}); ok {
			for _, raw := range containerStatuses {
				cs, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if parsed, ok := parseCatalogContainerStatus(cs, false); ok {
					entity.Containers = append(entity.Containers, parsed)
				}
				restarts += containerRestartCount(cs)
			}
		}
		entity.RestartCount = &restarts
	}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		if owners, ok := meta["ownerReferences"].([]interface{}); ok {
			var picked map[string]interface{}
			for _, raw := range owners {
				owner, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if controller, ok := owner["controller"].(bool); ok && controller {
					picked = owner
					break
				}
				if picked == nil {
					picked = owner
				}
			}
			if picked != nil {
				if kind, ok := picked["kind"].(string); ok {
					entity.OwnerKind = kind
				}
				if name, ok := picked["name"].(string); ok {
					entity.OwnerName = name
				}
			}
		}
	}
}

func containerRestartCount(cs map[string]interface{}) int32 {
	switch v := cs["restartCount"].(type) {
	case float64:
		return int32(v)
	case int64:
		return int32(v)
	case int32:
		return v
	default:
		return 0
	}
}

func parseCatalogContainerStatus(cs map[string]interface{}, init bool) (model.CatalogContainerStatus, bool) {
	name, _ := cs["name"].(string)
	if name == "" {
		return model.CatalogContainerStatus{}, false
	}
	ready, _ := cs["ready"].(bool)
	state, reason, message := catalogContainerState(cs["state"])
	return model.CatalogContainerStatus{
		Name:    name,
		State:   state,
		Ready:   ready,
		Init:    init,
		Reason:  reason,
		Message: message,
	}, true
}

func catalogContainerState(raw interface{}) (state, reason, message string) {
	st, ok := raw.(map[string]interface{})
	if !ok {
		return "unknown", "", ""
	}
	if waiting, ok := st["waiting"].(map[string]interface{}); ok {
		reason, _ = waiting["reason"].(string)
		message, _ = waiting["message"].(string)
		return "waiting", reason, message
	}
	if running, ok := st["running"].(map[string]interface{}); ok && running != nil {
		return "running", "", ""
	}
	if terminated, ok := st["terminated"].(map[string]interface{}); ok {
		reason, _ = terminated["reason"].(string)
		message, _ = terminated["message"].(string)
		return "terminated", reason, message
	}
	return "unknown", "", ""
}
