package kube

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"

	"github.com/glnreddy421/klew/internal/model"
)

func TestCatalogContainerState(t *testing.T) {
	state, reason, message := catalogContainerState(map[string]interface{}{
		"waiting": map[string]interface{}{
			"reason":  "CrashLoopBackOff",
			"message": "back-off 5m0s restarting failed container=app",
		},
	})
	if state != "waiting" || reason != "CrashLoopBackOff" || message == "" {
		t.Fatalf("waiting state = %q %q %q", state, reason, message)
	}

	state, reason, _ = catalogContainerState(map[string]interface{}{
		"running": map[string]interface{}{"startedAt": "2026-01-01T00:00:00Z"},
	})
	if state != "running" || reason != "" {
		t.Fatalf("running state = %q %q", state, reason)
	}
}

func TestEnrichWorkloadCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "api-75495887df"}
	enrichWorkloadCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": float64(2),
		},
		"status": map[string]interface{}{
			"replicas":      float64(2),
			"readyReplicas": float64(2),
		},
	})
	if entity.DesiredReplicas == nil || *entity.DesiredReplicas != 2 {
		t.Fatalf("desired = %v", entity.DesiredReplicas)
	}
	if entity.CurrentReplicas == nil || *entity.CurrentReplicas != 2 {
		t.Fatalf("current = %v", entity.CurrentReplicas)
	}
	if entity.ReadyReplicas == nil || *entity.ReadyReplicas != 2 {
		t.Fatalf("ready = %v", entity.ReadyReplicas)
	}

	entity = model.CatalogEntity{Name: "scaled-down"}
	enrichWorkloadCatalogEntity(&entity, map[string]interface{}{
		"status": map[string]interface{}{
			"replicas": float64(0),
		},
	})
	if entity.ReadyReplicas == nil || *entity.ReadyReplicas != 0 {
		t.Fatalf("ready for scaled down = %v", entity.ReadyReplicas)
	}
}

func TestEnrichDaemonSetCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "kube-proxy"}
	enrichDaemonSetCatalogEntity(&entity, map[string]interface{}{
		"status": map[string]interface{}{
			"desiredNumberScheduled": float64(2),
			"currentNumberScheduled": float64(2),
			"numberReady":            float64(2),
			"updatedNumberScheduled": float64(2),
			"numberAvailable":        float64(2),
			"numberMisscheduled":     float64(0),
		},
	})
	for _, check := range []struct {
		name string
		got  *int32
		want int32
	}{
		{"desired", entity.DesiredReplicas, 2},
		{"current", entity.CurrentReplicas, 2},
		{"ready", entity.ReadyReplicas, 2},
		{"updated", entity.UpdatedReplicas, 2},
		{"available", entity.AvailableReplicas, 2},
		{"misscheduled", entity.Misscheduled, 0},
	} {
		if check.got == nil || *check.got != check.want {
			t.Fatalf("%s = %v want %d", check.name, check.got, check.want)
		}
	}

	entity = model.CatalogEntity{Name: "missing-misscheduled-key"}
	enrichDaemonSetCatalogEntity(&entity, map[string]interface{}{
		"status": map[string]interface{}{
			"desiredNumberScheduled": float64(1),
		},
	})
	if entity.Misscheduled == nil || *entity.Misscheduled != 0 {
		t.Fatalf("misscheduled default = %v", entity.Misscheduled)
	}
}

func TestEnrichJobCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "hello"}
	enrichJobCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"completions": float64(3),
		},
		"status": map[string]interface{}{
			"succeeded": float64(2),
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Complete",
					"status": "False",
					"reason": "JobIncomplete",
				},
			},
		},
	})
	if entity.Succeeded == nil || *entity.Succeeded != 2 {
		t.Fatalf("succeeded = %v", entity.Succeeded)
	}
	if entity.Completions == nil || *entity.Completions != 3 {
		t.Fatalf("completions = %v", entity.Completions)
	}
	if len(entity.Conditions) != 1 || entity.Conditions[0].Type != "Complete" {
		t.Fatalf("conditions = %+v", entity.Conditions)
	}

	entity = model.CatalogEntity{Name: "default-completions"}
	enrichJobCatalogEntity(&entity, map[string]interface{}{
		"status": map[string]interface{}{
			"succeeded": float64(1),
		},
	})
	if entity.Completions == nil || *entity.Completions != 1 {
		t.Fatalf("default completions = %v", entity.Completions)
	}
}

func TestJobDurationHint(t *testing.T) {
	start := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
	completion := time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	got := jobDurationHint(map[string]interface{}{
		"status": map[string]interface{}{
			"startTime":      start,
			"completionTime": completion,
		},
	})
	if got == "" {
		t.Fatal("expected duration")
	}
	if !strings.Contains(got, "m") {
		t.Fatalf("expected minute duration, got %q", got)
	}
}

func TestEnrichCronJobCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "nightly"}
	enrichCronJobCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"schedule": "0 2 * * *",
			"suspend":  true,
		},
		"status": map[string]interface{}{
			"lastScheduleTime": "2026-09-09T10:00:00Z",
			"active": []interface{}{
				map[string]interface{}{"kind": "Job", "name": "nightly-123", "namespace": "default"},
			},
		},
	})
	if entity.Schedule != "0 2 * * *" {
		t.Fatalf("schedule = %q", entity.Schedule)
	}
	if entity.Suspend == nil || !*entity.Suspend {
		t.Fatalf("suspend = %v", entity.Suspend)
	}
	if entity.ActiveJobs == nil || *entity.ActiveJobs != 1 {
		t.Fatalf("active = %v", entity.ActiveJobs)
	}
	if entity.LastScheduleTime != "2026-09-09T10:00:00Z" {
		t.Fatalf("lastScheduleTime = %q", entity.LastScheduleTime)
	}

	entity = model.CatalogEntity{Name: "defaults"}
	enrichCronJobCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"schedule": "*/5 * * * *",
		},
	})
	if entity.Suspend == nil || *entity.Suspend {
		t.Fatalf("default suspend = %v", entity.Suspend)
	}
	if entity.ActiveJobs == nil || *entity.ActiveJobs != 0 {
		t.Fatalf("default active = %v", entity.ActiveJobs)
	}
}

func TestCoerceInt32FromJSONNumber(t *testing.T) {
	n := coerceInt32(json.Number("3"))
	if n == nil || *n != 3 {
		t.Fatalf("json.Number = %v", n)
	}
}

func TestEnrichDaemonSetFromListJSON(t *testing.T) {
	raw := `{
		"apiVersion":"apps/v1",
		"kind":"DaemonSet",
		"metadata":{"name":"kube-proxy","namespace":"kube-system"},
		"status":{
			"desiredNumberScheduled":2,
			"currentNumberScheduled":2,
			"numberReady":2,
			"updatedNumberScheduled":2,
			"numberAvailable":2,
			"numberMisscheduled":0
		}
	}`
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatal(err)
	}
	entity := model.CatalogEntity{Name: "kube-proxy"}
	enrichDaemonSetCatalogEntity(&entity, obj)
	if entity.DesiredReplicas == nil || *entity.DesiredReplicas != 2 {
		t.Fatalf("desired from list json = %v", entity.DesiredReplicas)
	}
	if entity.Misscheduled == nil || *entity.Misscheduled != 0 {
		t.Fatalf("misscheduled from list json = %v", entity.Misscheduled)
	}
}

func TestEnrichDeploymentCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "api"}
	enrichDeploymentCatalogEntity(&entity, map[string]interface{}{
		"status": map[string]interface{}{
			"availableReplicas": float64(2),
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Available",
					"status": "True",
					"reason": "MinimumReplicasAvailable",
				},
				map[string]interface{}{
					"type":   "Progressing",
					"status": "True",
					"reason": "NewReplicaSetAvailable",
				},
			},
		},
	})
	if entity.AvailableReplicas == nil || *entity.AvailableReplicas != 2 {
		t.Fatalf("available = %v", entity.AvailableReplicas)
	}
	if len(entity.Conditions) != 2 || entity.Conditions[0].Type != "Available" {
		t.Fatalf("conditions = %+v", entity.Conditions)
	}
}

func TestParseCatalogContainerStatus(t *testing.T) {
	got, ok := parseCatalogContainerStatus(map[string]interface{}{
		"name":  "app",
		"ready": false,
		"state": map[string]interface{}{
			"waiting": map[string]interface{}{
				"reason":  "CrashLoopBackOff",
				"message": "back-off restarting",
			},
		},
	}, false)
	if !ok {
		t.Fatal("expected parsed container")
	}
	if got.Name != "app" || got.State != "waiting" || got.Reason != "CrashLoopBackOff" {
		t.Fatalf("unexpected container: %+v", got)
	}
}

func TestLiveListDaemonSets(t *testing.T) {
	client, err := NewFromFlags("", "docker-desktop", "")
	if err != nil {
		t.Skip(err)
	}
	list, err := ListCatalogEntities(context.Background(), client, "apps/v1/daemonsets", CatalogEntityScope{AllNamespaces: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Entities) == 0 {
		t.Fatal("no daemonsets")
	}
	found := false
	for _, e := range list.Entities {
		if e.Name != "kube-proxy" {
			continue
		}
		found = true
		if e.DesiredReplicas == nil || *e.DesiredReplicas != 2 {
			t.Fatalf("kube-proxy desired = %v hint=%q", e.DesiredReplicas, e.StatusHint)
		}
		if e.Misscheduled == nil {
			t.Fatalf("kube-proxy misscheduled nil hint=%q", e.StatusHint)
		}
	}
	if !found {
		t.Skip("kube-proxy not found")
	}
	for _, e := range list.Entities {
		if e.Name != "kube-proxy" {
			continue
		}
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("kube-proxy json: %s", string(b))
	}
}

func TestServiceEndpointStatusHint(t *testing.T) {
	if got := serviceEndpointStatusHint("ExternalName", 0, 0); got != "External" {
		t.Fatalf("external = %q", got)
	}
	if got := serviceEndpointStatusHint("ClusterIP", 0, 0); got != "No endpoints" {
		t.Fatalf("empty = %q", got)
	}
	if got := serviceEndpointStatusHint("ClusterIP", 2, 3); got != "2/3 ready" {
		t.Fatalf("partial = %q", got)
	}
}

func TestCountEndpointSliceAddresses(t *testing.T) {
	ready := true
	notReady := false
	readyN, totalN := countEndpointSliceAddresses([]discoveryv1.EndpointSlice{{
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1", "10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
			{Addresses: []string{"10.0.0.3"}, Conditions: discoveryv1.EndpointConditions{Ready: &notReady}},
		},
	}})
	if readyN != 2 || totalN != 3 {
		t.Fatalf("ready=%d total=%d", readyN, totalN)
	}
}

func TestEnrichEndpointsCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "kubernetes"}
	enrichEndpointsCatalogEntity(&entity, map[string]interface{}{
		"subsets": []interface{}{
			map[string]interface{}{
				"addresses": []interface{}{
					map[string]interface{}{"ip": "172.20.0.2"},
				},
				"ports": []interface{}{
					map[string]interface{}{"name": "https", "port": float64(6443), "protocol": "TCP"},
				},
			},
		},
	})
	if entity.EndpointSummary != "172.20.0.2:6443" {
		t.Fatalf("summary = %q", entity.EndpointSummary)
	}
	if entity.ReadyEndpoints == nil || *entity.ReadyEndpoints != 1 {
		t.Fatalf("ready = %v", entity.ReadyEndpoints)
	}
}

func TestEnrichEndpointSliceCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "kubernetes-abcd"}
	enrichEndpointSliceCatalogEntity(&entity, map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"kubernetes.io/service-name": "kubernetes",
			},
		},
		"addressType": "IPv4",
		"endpoints": []interface{}{
			map[string]interface{}{
				"addresses": []interface{}{"172.20.0.2"},
				"conditions": map[string]interface{}{"ready": true},
			},
		},
		"ports": []interface{}{
			map[string]interface{}{"port": float64(6443), "protocol": "TCP"},
		},
	})
	if entity.ServiceName != "kubernetes" {
		t.Fatalf("service = %q", entity.ServiceName)
	}
	if entity.EndpointSummary != "172.20.0.2:6443" {
		t.Fatalf("summary = %q", entity.EndpointSummary)
	}
}

func TestEnrichIngressCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "my-ing"}
	enrichIngressCatalogEntity(&entity, map[string]interface{}{
		"status": map[string]interface{}{
			"loadBalancer": map[string]interface{}{
				"ingress": []interface{}{
					map[string]interface{}{"ip": "203.0.113.10"},
				},
			},
		},
		"spec": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{
					"host": "app.example.com",
					"http": map[string]interface{}{
						"paths": []interface{}{
							map[string]interface{}{"path": "/api", "pathType": "Prefix"},
							map[string]interface{}{"path": "/web", "pathType": "Prefix"},
						},
					},
				},
			},
		},
	})
	if len(entity.LoadBalancers) != 1 || entity.LoadBalancers[0] != "203.0.113.10" {
		t.Fatalf("load balancers = %v", entity.LoadBalancers)
	}
	if entity.IngressRulesSummary != "app.example.com/api +1" {
		t.Fatalf("rules = %q", entity.IngressRulesSummary)
	}
}

func TestEnrichIngressClassCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "nginx"}
	enrichIngressClassCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"controller": "k8s.io/ingress-nginx",
			"parameters": map[string]interface{}{
				"apiGroup":  "k8s.example.com",
				"kind":      "IngressParameter",
				"name":      "external-config",
				"namespace": "default",
				"scope":     "Namespace",
			},
		},
	})
	if entity.IngressController != "k8s.io/ingress-nginx" {
		t.Fatalf("controller = %q", entity.IngressController)
	}
	if entity.ParameterAPIGroup != "k8s.example.com" {
		t.Fatalf("apiGroup = %q", entity.ParameterAPIGroup)
	}
	if entity.ParameterKind != "IngressParameter" {
		t.Fatalf("kind = %q", entity.ParameterKind)
	}
	if entity.ParameterNamespace != "default" {
		t.Fatalf("namespace = %q", entity.ParameterNamespace)
	}
	if entity.ParameterScope != "Namespace" {
		t.Fatalf("scope = %q", entity.ParameterScope)
	}
}

func TestEnrichNetworkPolicyCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "deny-all"}
	enrichNetworkPolicyCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"policyTypes": []interface{}{"Ingress", "Egress"},
			"podSelector": map[string]interface{}{},
		},
	})
	if len(entity.PolicyTypes) != 2 || entity.PolicyTypes[0] != "Ingress" || entity.PolicyTypes[1] != "Egress" {
		t.Fatalf("policy types = %v", entity.PolicyTypes)
	}

	entity = model.CatalogEntity{Name: "web-ingress"}
	enrichNetworkPolicyCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"ingress": []interface{}{
				map[string]interface{}{},
			},
		},
	})
	if len(entity.PolicyTypes) != 1 || entity.PolicyTypes[0] != "Ingress" {
		t.Fatalf("inferred policy types = %v", entity.PolicyTypes)
	}
}

func TestEnrichStorageClassCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "hostpath"}
	enrichStorageClassCatalogEntity(&entity, map[string]interface{}{
		"kind": "StorageClass",
		"metadata": map[string]interface{}{
			"name": "hostpath",
			"annotations": map[string]interface{}{
				"storageclass.kubernetes.io/is-default-class": "false",
			},
		},
		"provisioner":       "docker.io/hostpath",
		"reclaimPolicy":     "Delete",
		"volumeBindingMode": "WaitForFirstConsumer",
	})
	if entity.Provisioner != "docker.io/hostpath" {
		t.Fatalf("provisioner = %q", entity.Provisioner)
	}
	if entity.ReclaimPolicy != "Delete" {
		t.Fatalf("reclaim = %q", entity.ReclaimPolicy)
	}
	if entity.VolumeBindingMode != "WaitForFirstConsumer" {
		t.Fatalf("volumeBindingMode = %q", entity.VolumeBindingMode)
	}
	if entity.IsDefault == nil || *entity.IsDefault {
		t.Fatalf("isDefault = %v, want false", entity.IsDefault)
	}

	entity = model.CatalogEntity{Name: "standard"}
	enrichStorageClassCatalogEntity(&entity, map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		"spec": map[string]interface{}{
			"provisioner":       "kubernetes.io/gce-pd",
			"reclaimPolicy":     "Delete",
			"volumeBindingMode": "Immediate",
		},
	})
	if entity.Provisioner != "kubernetes.io/gce-pd" {
		t.Fatalf("spec fallback provisioner = %q", entity.Provisioner)
	}
	if entity.IsDefault == nil || !*entity.IsDefault {
		t.Fatalf("isDefault = %v", entity.IsDefault)
	}
}

func TestEnrichPVCCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "data"}
	enrichPVCCatalogEntity(&entity, map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Bound",
			"capacity": map[string]interface{}{
				"storage": "8Gi",
			},
		},
		"spec": map[string]interface{}{
			"volumeName":       "pv-abc",
			"storageClassName": "hostpath",
			"accessModes":      []interface{}{"ReadWriteOnce"},
		},
	})
	if entity.StatusHint != "Bound" {
		t.Fatalf("phase = %q", entity.StatusHint)
	}
	if entity.VolumeName != "pv-abc" || entity.StorageClassName != "hostpath" {
		t.Fatalf("volume/sc = %q / %q", entity.VolumeName, entity.StorageClassName)
	}
	if entity.Capacity != "8Gi" {
		t.Fatalf("capacity = %q", entity.Capacity)
	}
}

func TestEnrichPVCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "pv-abc"}
	enrichPVCatalogEntity(&entity, map[string]interface{}{
		"status": map[string]interface{}{"phase": "Bound"},
		"spec": map[string]interface{}{
			"capacity": map[string]interface{}{"storage": "8Gi"},
			"accessModes": []interface{}{"ReadWriteOnce"},
			"persistentVolumeReclaimPolicy": "Delete",
			"storageClassName":              "hostpath",
			"claimRef": map[string]interface{}{
				"namespace": "default",
				"name":      "data",
			},
		},
	})
	if entity.ReclaimPolicy != "Delete" {
		t.Fatalf("reclaim = %q", entity.ReclaimPolicy)
	}
	if entity.ClaimRef != "default/data" {
		t.Fatalf("claim = %q", entity.ClaimRef)
	}
}

func TestEnrichConfigMapCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "kube-root-ca.crt"}
	enrichConfigMapCatalogEntity(&entity, map[string]interface{}{
		"data": map[string]interface{}{
			"client-ca-file": "-----BEGIN",
		},
		"binaryData": map[string]interface{}{
			"bundle": "YQ==",
		},
	})
	if len(entity.ConfigMapData) != 2 {
		t.Fatalf("entries = %v", entity.ConfigMapData)
	}
	byKey := map[string]model.CatalogDataEntry{}
	for _, e := range entity.ConfigMapData {
		byKey[e.Key] = e
	}
	if byKey["client-ca-file"].SizeBytes != 10 {
		t.Fatalf("client-ca-file size = %d", byKey["client-ca-file"].SizeBytes)
	}
	if byKey["client-ca-file"].Value != "-----BEGIN" {
		t.Fatalf("client-ca-file value = %q", byKey["client-ca-file"].Value)
	}
	if byKey["bundle (binary)"].SizeBytes != 1 {
		t.Fatalf("bundle size = %d", byKey["bundle (binary)"].SizeBytes)
	}
	if byKey["bundle (binary)"].Value != "YQ==" {
		t.Fatalf("bundle value = %q", byKey["bundle (binary)"].Value)
	}
}

func TestEnrichLeaseCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "kube-controller-manager"}
	enrichLeaseCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"holderIdentity": "kube-controller-manager_abc123",
		},
	})
	if entity.LeaseHolder != "kube-controller-manager_abc123" {
		t.Fatalf("holder = %q", entity.LeaseHolder)
	}
}

func TestEnrichServiceCatalogEntity(t *testing.T) {
	entity := model.CatalogEntity{Name: "payment-api"}
	enrichServiceCatalogEntity(&entity, map[string]interface{}{
		"spec": map[string]interface{}{
			"type":      "LoadBalancer",
			"clusterIP": "10.96.0.12",
			"selector": map[string]interface{}{
				"app": "payment-api",
			},
			"ports": []interface{}{
				map[string]interface{}{
					"name":     "http",
					"port":     float64(80),
					"protocol": "TCP",
				},
			},
			"externalIPs": []interface{}{"203.0.113.10"},
		},
		"status": map[string]interface{}{
			"loadBalancer": map[string]interface{}{
				"ingress": []interface{}{
					map[string]interface{}{"ip": "198.51.100.7"},
				},
			},
		},
	})
	if entity.ServiceType != "LoadBalancer" {
		t.Fatalf("type = %q", entity.ServiceType)
	}
	if entity.ClusterIP != "10.96.0.12" {
		t.Fatalf("clusterIP = %q", entity.ClusterIP)
	}
	if entity.Selector != "app=payment-api" {
		t.Fatalf("selector = %q", entity.Selector)
	}
	if len(entity.Ports) != 1 || entity.Ports[0] != "http:80/TCP" {
		t.Fatalf("ports = %v", entity.Ports)
	}
	if len(entity.ExternalIPs) != 2 {
		t.Fatalf("externalIPs = %v", entity.ExternalIPs)
	}
}
