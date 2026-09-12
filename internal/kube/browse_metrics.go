package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/glnreddy421/klew/internal/model"
)

// ContainerMetricUsage is live usage for one container (metrics-server).
type ContainerMetricUsage struct {
	CPUMilli int64 `json:"cpuMilli"`
	MemMi    int64 `json:"memMi"`
}

// PodMetricUsage is live usage for one pod and its containers.
type PodMetricUsage struct {
	CPUMilli   int64                           `json:"cpuMilli"`
	MemMi      int64                           `json:"memMi"`
	Containers map[string]ContainerMetricUsage `json:"containers,omitempty"`
}

// BrowseMetricsResult is scope-level resource usage for Resources browse.
type BrowseMetricsResult struct {
	Available bool                 `json:"available"`
	Note      string                 `json:"note,omitempty"`
	Summary   model.MetricsSummary   `json:"summary"`
}

func podMetricsKey(namespace, name string) string {
	return namespace + "/" + name
}

func newMetricsClient(client *Client) (*metricsclient.Clientset, error) {
	if client == nil || client.Config == nil {
		return nil, fmt.Errorf("kubernetes client config is required")
	}
	return metricsclient.NewForConfig(client.Config)
}

func listPodMetricsMap(ctx context.Context, client *Client, namespaces map[string]struct{}) (map[string]PodMetricUsage, bool) {
	out := map[string]PodMetricUsage{}
	mc, err := newMetricsClient(client)
	if err != nil {
		return out, false
	}
	found := false
	for ns := range namespaces {
		list, err := mc.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		found = true
		for _, pm := range list.Items {
			usage := PodMetricUsage{Containers: map[string]ContainerMetricUsage{}}
			for _, c := range pm.Containers {
				var cpuMilli, memMi int64
				if q := c.Usage.Cpu(); q != nil {
					cpuMilli = q.MilliValue()
				}
				if q := c.Usage.Memory(); q != nil {
					memMi = q.Value() / (1024 * 1024)
				}
				usage.CPUMilli += cpuMilli
				usage.MemMi += memMi
				usage.Containers[c.Name] = ContainerMetricUsage{CPUMilli: cpuMilli, MemMi: memMi}
			}
			out[podMetricsKey(pm.Namespace, pm.Name)] = usage
		}
	}
	return out, found
}

func namespacesFromEntities(entities []model.CatalogEntity) map[string]struct{} {
	ns := map[string]struct{}{}
	for _, entity := range entities {
		if entity.Namespace != "" {
			ns[entity.Namespace] = struct{}{}
		}
	}
	return ns
}

func namespacesFromScope(scope CatalogEntityScope) map[string]struct{} {
	ns := map[string]struct{}{}
	switch {
	case scope.AllNamespaces:
		return ns
	case len(scope.Namespaces) > 0:
		for _, n := range scope.Namespaces {
			if n != "" {
				ns[n] = struct{}{}
			}
		}
	case scope.Namespace != "":
		ns[scope.Namespace] = struct{}{}
	}
	return ns
}

func applyPodMetricsToEntities(entities []model.CatalogEntity, metrics map[string]PodMetricUsage) {
	for i := range entities {
		usage, ok := metrics[podMetricsKey(entities[i].Namespace, entities[i].Name)]
		if !ok {
			continue
		}
		if entities[i].TableFields == nil {
			entities[i].TableFields = map[string]string{}
		}
		if usage.CPUMilli > 0 {
			cpu := usage.CPUMilli
			entities[i].CPUUsageMilli = &cpu
			entities[i].TableFields["cpu"] = FormatCPUMilli(usage.CPUMilli)
		}
		if usage.MemMi > 0 {
			mem := usage.MemMi
			entities[i].MemUsageMi = &mem
			entities[i].TableFields["memory"] = FormatMemMi(usage.MemMi)
		}
	}
}

func SummarizeCatalogPodMetrics(entities []model.CatalogEntity, objects []map[string]interface{}) model.MetricsSummary {
	var summary model.MetricsSummary
	for i, entity := range entities {
		if entity.CPUUsageMilli != nil {
			summary.CPUUsageM += *entity.CPUUsageMilli
			summary.Available = true
		}
		if entity.MemUsageMi != nil {
			summary.MemUsageMi += *entity.MemUsageMi
			summary.Available = true
		}
		if i < len(objects) {
			reqCPU, reqMem, limCPU, limMem := podResourceTotalsFromObject(objects[i])
			summary.CPURequestM += reqCPU
			summary.MemRequestMi += reqMem
			summary.CPULimitM += limCPU
			summary.MemLimitMi += limMem
		}
	}
	if summary.Available {
		summary.Note = "metrics-server"
	} else {
		summary.Note = "metrics-server unavailable — showing pod requests/limits where listed"
	}
	return summary
}

func podResourceTotalsFromObject(obj map[string]interface{}) (reqCPU, reqMem, limCPU, limMem int64) {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return 0, 0, 0, 0
	}
	for _, key := range []string{"containers", "initContainers"} {
		raw, ok := spec[key].([]interface{})
		if !ok {
			continue
		}
		for _, item := range raw {
			cm, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			res, _ := cm["resources"].(map[string]interface{})
			reqCPU += quantityMapMilli(res, "requests", string(corev1.ResourceCPU))
			reqMem += quantityMapMi(res, "requests", string(corev1.ResourceMemory))
			limCPU += quantityMapMilli(res, "limits", string(corev1.ResourceCPU))
			limMem += quantityMapMi(res, "limits", string(corev1.ResourceMemory))
		}
	}
	return reqCPU, reqMem, limCPU, limMem
}

func quantityMapMilli(res map[string]interface{}, bucket, key string) int64 {
	if res == nil {
		return 0
	}
	raw, ok := res[bucket].(map[string]interface{})
	if !ok {
		return 0
	}
	val, ok := raw[key].(string)
	if !ok || val == "" {
		return 0
	}
	q, err := resource.ParseQuantity(val)
	if err != nil {
		return 0
	}
	return q.MilliValue()
}

func quantityMapMi(res map[string]interface{}, bucket, key string) int64 {
	if res == nil {
		return 0
	}
	raw, ok := res[bucket].(map[string]interface{})
	if !ok {
		return 0
	}
	val, ok := raw[key].(string)
	if !ok || val == "" {
		return 0
	}
	q, err := resource.ParseQuantity(val)
	if err != nil {
		return 0
	}
	return q.Value() / (1024 * 1024)
}

func enrichListedPodMetrics(ctx context.Context, client *Client, entities []model.CatalogEntity, listNamespace string) {
	if client == nil || len(entities) == 0 {
		return
	}
	nsSet := namespacesFromEntities(entities)
	if listNamespace != "" {
		nsSet[listNamespace] = struct{}{}
	}
	metrics, _ := listPodMetricsMap(ctx, client, nsSet)
	if len(metrics) > 0 {
		applyPodMetricsToEntities(entities, metrics)
	}
}

// GetPodMetricUsage returns live metrics for one pod when metrics-server is available.
func GetPodMetricUsage(ctx context.Context, client *Client, namespace, name string) (PodMetricUsage, bool) {
	if client == nil || namespace == "" || name == "" {
		return PodMetricUsage{}, false
	}
	mc, err := newMetricsClient(client)
	if err != nil {
		return PodMetricUsage{}, false
	}
	pm, err := mc.MetricsV1beta1().PodMetricses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return PodMetricUsage{}, false
	}
	usage := PodMetricUsage{Containers: map[string]ContainerMetricUsage{}}
	for _, c := range pm.Containers {
		var cpuMilli, memMi int64
		if q := c.Usage.Cpu(); q != nil {
			cpuMilli = q.MilliValue()
		}
		if q := c.Usage.Memory(); q != nil {
			memMi = q.Value() / (1024 * 1024)
		}
		usage.CPUMilli += cpuMilli
		usage.MemMi += memMi
		usage.Containers[c.Name] = ContainerMetricUsage{CPUMilli: cpuMilli, MemMi: memMi}
	}
	return usage, true
}

// CollectBrowseMetrics aggregates pod usage for a browse scope.
func CollectBrowseMetrics(ctx context.Context, client *Client, scope CatalogEntityScope) BrowseMetricsResult {
	result := BrowseMetricsResult{
		Summary: model.MetricsSummary{
			Note: "metrics-server unavailable — install metrics-server for live CPU/memory usage",
		},
	}
	if client == nil {
		return result
	}
	nsSet := namespacesFromScope(scope)
	if len(nsSet) == 0 {
		return result
	}
	metrics, available := listPodMetricsMap(ctx, client, nsSet)
	result.Available = available
	if available {
		result.Note = "metrics-server"
		result.Summary.Available = true
		result.Summary.Note = "metrics-server"
	}
	for _, usage := range metrics {
		result.Summary.CPUUsageM += usage.CPUMilli
		result.Summary.MemUsageMi += usage.MemMi
	}
	return result
}

func FormatCPUMilli(milli int64) string {
	if milli <= 0 {
		return ""
	}
	if milli >= 1000 {
		if milli%1000 == 0 {
			return fmt.Sprintf("%d", milli/1000)
		}
		return fmt.Sprintf("%.2f", float64(milli)/1000)
	}
	return fmt.Sprintf("%dm", milli)
}

func FormatMemMi(mi int64) string {
	if mi <= 0 {
		return ""
	}
	if mi >= 1024 {
		if mi%1024 == 0 {
			return fmt.Sprintf("%dGi", mi/1024)
		}
		return fmt.Sprintf("%.1fGi", float64(mi)/1024)
	}
	return fmt.Sprintf("%dMi", mi)
}
