package kube

import (
	"context"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/glnreddy421/klew/internal/model"
)

// CollectMetrics aggregates resource usage for the given pods.
//
// It prefers metrics.k8s.io (metrics-server — the same source as `kubectl top`),
// summing live CPU/memory usage across exactly the supplied pods. Requests and
// limits are always derived natively from the pod specs (no metrics-server
// required). If metrics-server is unavailable, usage is omitted and only the
// native requests/limits are returned.
func CollectMetrics(ctx context.Context, client *Client, pods []model.PodSummary) model.MetricsSummary {
	var m model.MetricsSummary

	// native path: requests/limits straight from the collected pod specs
	for _, p := range pods {
		for _, c := range p.Containers {
			m.CPURequestM += milliCPU(c.RequestsCPU)
			m.CPULimitM += milliCPU(c.LimitsCPU)
			m.MemRequestMi += mebiMem(c.RequestsMem)
			m.MemLimitMi += mebiMem(c.LimitsMem)
		}
	}

	if client == nil || client.Config == nil || len(pods) == 0 {
		m.Note = "metrics-server unavailable — showing pod requests/limits"
		return m
	}

	mc, err := metricsclient.NewForConfig(client.Config)
	if err != nil {
		m.Note = "metrics-server unavailable — showing pod requests/limits"
		return m
	}

	// which pods (by namespace) do we care about?
	want := map[string]bool{}
	namespaces := map[string]bool{}
	for _, p := range pods {
		want[p.Namespace+"/"+p.Name] = true
		namespaces[p.Namespace] = true
	}

	found := false
	for ns := range namespaces {
		list, err := mc.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		found = true
		for _, pm := range list.Items {
			if !want[pm.Namespace+"/"+pm.Name] {
				continue
			}
			for _, c := range pm.Containers {
				if cpu := c.Usage.Cpu(); cpu != nil {
					m.CPUUsageM += cpu.MilliValue()
				}
				if mem := c.Usage.Memory(); mem != nil {
					m.MemUsageMi += mem.Value() / (1024 * 1024)
				}
			}
		}
	}

	if found {
		m.Available = true
		m.Note = "metrics-server"
	} else {
		m.Note = "metrics-server unavailable — showing pod requests/limits"
	}
	return m
}

func milliCPU(s string) int64 {
	if s == "" {
		return 0
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return 0
	}
	return q.MilliValue()
}

func mebiMem(s string) int64 {
	if s == "" {
		return 0
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return 0
	}
	return q.Value() / (1024 * 1024)
}
