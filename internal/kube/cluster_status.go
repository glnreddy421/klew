package kube

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
)

const clusterStatusTTL = 30 * time.Second

// ClusterNodeSummary is cluster-wide node inventory (not investigation scope).
type ClusterNodeSummary struct {
	Total     int `json:"total"`
	Ready     int `json:"ready"`
	NotReady  int `json:"notReady"`
	Pressured int `json:"pressured"`
}

// ClusterNodeItem is one node in cluster-wide inventory.
type ClusterNodeItem struct {
	Name           string `json:"name"`
	Ready          bool   `json:"ready"`
	MemoryPressure bool   `json:"memoryPressure"`
	DiskPressure   bool   `json:"diskPressure"`
	PIDPressure    bool   `json:"pidPressure"`
	Unschedulable  bool   `json:"unschedulable"`
	KubeletVersion string `json:"kubeletVersion,omitempty"`
	Role           string `json:"role,omitempty"`
}

// ClusterStatus is cluster-level reachability and inventory for Overview context.
type ClusterStatus struct {
	Available         bool               `json:"available"`
	CollectedAt       model.Timestamp    `json:"collectedAt"`
	KubernetesVersion string             `json:"kubernetesVersion,omitempty"`
	Platform          string             `json:"platform,omitempty"`
	APIReachable      bool               `json:"apiReachable"`
	Error             string             `json:"error,omitempty"`
	Nodes             ClusterNodeSummary `json:"nodes"`
	NodeItems         []ClusterNodeItem        `json:"nodeItems,omitempty"`
	Versions          ClusterVersionSummary    `json:"versions"`
	NamespaceCount    int                      `json:"namespaceCount,omitempty"`
}

type cachedClusterStatus struct {
	status ClusterStatus
	at     time.Time
}

type clusterStatusCache struct {
	mu   sync.Mutex
	byID map[string]cachedClusterStatus
}

var globalClusterStatusCache clusterStatusCache

func (c *clusterStatusCache) key(kubeconfigPath string, client *Client) string {
	return kubeconfigPath + "|" + client.Context + "|" + client.Cluster
}

func (c *clusterStatusCache) get(kubeconfigPath string, client *Client) (ClusterStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byID == nil {
		return ClusterStatus{}, false
	}
	entry, ok := c.byID[c.key(kubeconfigPath, client)]
	if !ok || time.Since(entry.at) > clusterStatusTTL {
		return ClusterStatus{}, false
	}
	return entry.status, true
}

func (c *clusterStatusCache) set(kubeconfigPath string, client *Client, status ClusterStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byID == nil {
		c.byID = make(map[string]cachedClusterStatus)
	}
	c.byID[c.key(kubeconfigPath, client)] = cachedClusterStatus{status: status, at: time.Now()}
}

// InvalidateClusterStatus clears cached cluster status for a client identity.
func InvalidateClusterStatus(kubeconfigPath string, client *Client) {
	if client == nil {
		return
	}
	globalClusterStatusCache.mu.Lock()
	defer globalClusterStatusCache.mu.Unlock()
	if globalClusterStatusCache.byID == nil {
		return
	}
	delete(globalClusterStatusCache.byID, globalClusterStatusCache.key(kubeconfigPath, client))
}

// CollectClusterStatus queries the API server for version and node inventory.
func CollectClusterStatus(ctx context.Context, kubeconfigPath, selectedContext string) ClusterStatus {
	now := model.TimestampFrom(time.Now().UTC())
	if selectedContext == "" {
		return ClusterStatus{
			CollectedAt: now,
			Error:       "no context selected",
		}
	}

	client, err := NewFromFlags(kubeconfigPath, selectedContext, "")
	if err != nil {
		return ClusterStatus{
			CollectedAt: now,
			Error:       err.Error(),
		}
	}

	if cached, ok := globalClusterStatusCache.get(kubeconfigPath, client); ok {
		return cached
	}

	st := ClusterStatus{
		CollectedAt: now,
	}

	ver, err := client.Clientset.Discovery().ServerVersion()
	if err != nil {
		st.Error = fmt.Sprintf("api server: %v", err)
		return st
	}

	st.Available = true
	st.APIReachable = true
	st.KubernetesVersion = ver.GitVersion
	st.Platform = ver.Platform

	nodeList, err := client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		st.Error = fmt.Sprintf("list nodes: %v", err)
		globalClusterStatusCache.set(kubeconfigPath, client, st)
		return st
	}

	for _, n := range nodeList.Items {
		item := summarizeClusterNode(&n)
		st.NodeItems = append(st.NodeItems, item)
		st.Nodes.Total++
		if item.Ready {
			st.Nodes.Ready++
		} else {
			st.Nodes.NotReady++
		}
		if nodeItemPressured(item) {
			st.Nodes.Pressured++
		}
	}

	if nss, err := ListNamespaces(ctx, client); err == nil {
		st.NamespaceCount = len(nss)
	}

	st.Versions = summarizeClusterVersions(st.KubernetesVersion, st.NodeItems)

	globalClusterStatusCache.set(kubeconfigPath, client, st)
	return st
}

func summarizeClusterNode(n *corev1.Node) ClusterNodeItem {
	item := ClusterNodeItem{
		Name:           n.Name,
		KubeletVersion: n.Status.NodeInfo.KubeletVersion,
		Unschedulable:  n.Spec.Unschedulable,
		Role:           nodeRole(n),
	}
	for _, cond := range n.Status.Conditions {
		switch cond.Type {
		case corev1.NodeReady:
			item.Ready = cond.Status == corev1.ConditionTrue
		case corev1.NodeMemoryPressure:
			item.MemoryPressure = cond.Status == corev1.ConditionTrue
		case corev1.NodeDiskPressure:
			item.DiskPressure = cond.Status == corev1.ConditionTrue
		case corev1.NodePIDPressure:
			item.PIDPressure = cond.Status == corev1.ConditionTrue
		}
	}
	return item
}

func nodeItemPressured(item ClusterNodeItem) bool {
	return !item.Ready || item.MemoryPressure || item.DiskPressure ||
		item.PIDPressure || item.Unschedulable
}
