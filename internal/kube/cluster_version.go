package kube

import (
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// ClusterVersionGroup summarizes kubelet versions for one node role group.
type ClusterVersionGroup struct {
	Count    int            `json:"count"`
	Label    string         `json:"label,omitempty"`
	Skewed   bool           `json:"skewed"`
	Versions map[string]int `json:"versions,omitempty"`
}

// ClusterVersionSummary captures API server and kubelet versions by role.
type ClusterVersionSummary struct {
	APIServer    string              `json:"apiServer,omitempty"`
	ControlPlane ClusterVersionGroup `json:"controlPlane"`
	Workers      ClusterVersionGroup `json:"workers"`
	Skewed       bool                `json:"skewed"`
}

func summarizeClusterVersions(apiServer string, items []ClusterNodeItem) ClusterVersionSummary {
	var cpVersions, workerVersions []string
	for _, item := range items {
		v := strings.TrimSpace(item.KubeletVersion)
		if v == "" {
			continue
		}
		switch item.Role {
		case "control-plane":
			cpVersions = append(cpVersions, v)
		default:
			workerVersions = append(workerVersions, v)
		}
	}

	summary := ClusterVersionSummary{
		APIServer:    strings.TrimSpace(apiServer),
		ControlPlane: summarizeKubeletVersions(cpVersions),
		Workers:      summarizeKubeletVersions(workerVersions),
	}
	summary.Skewed = versionSummarySkewed(summary)
	return summary
}

func versionSummarySkewed(summary ClusterVersionSummary) bool {
	if summary.ControlPlane.Skewed || summary.Workers.Skewed {
		return true
	}
	labels := uniqueNonEmpty(
		normalizeVersionLabel(summary.APIServer),
		normalizeVersionLabel(summary.ControlPlane.Label),
		normalizeVersionLabel(summary.Workers.Label),
	)
	return len(labels) > 1
}

func summarizeKubeletVersions(versions []string) ClusterVersionGroup {
	counts := map[string]int{}
	for _, v := range versions {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		counts[v]++
	}
	if len(counts) == 0 {
		return ClusterVersionGroup{}
	}

	keys := make([]string, 0, len(counts))
	total := 0
	for k, n := range counts {
		keys = append(keys, k)
		total += n
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return compareKubeletVersion(keys[i], keys[j]) < 0
	})

	label := keys[0]
	skewed := len(keys) > 1
	if skewed {
		sorted := append([]string(nil), keys...)
		sort.Slice(sorted, func(i, j int) bool {
			return compareKubeletVersion(sorted[i], sorted[j]) < 0
		})
		label = sorted[0] + "–" + sorted[len(sorted)-1]
	}

	return ClusterVersionGroup{
		Count:    total,
		Label:    label,
		Skewed:   skewed,
		Versions: counts,
	}
}

func compareKubeletVersion(a, b string) int {
	av := strings.TrimPrefix(strings.TrimSpace(a), "v")
	bv := strings.TrimPrefix(strings.TrimSpace(b), "v")
	return strings.Compare(av, bv)
}

func normalizeVersionLabel(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// Collapse ranges to the upper bound for equality checks during upgrades.
	if i := strings.Index(v, "–"); i >= 0 {
		return strings.TrimSpace(v[i+len("–"):])
	}
	return v
}

func uniqueNonEmpty(values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = normalizeVersionLabel(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func isControlPlaneNode(n *corev1.Node) bool {
	if n == nil {
		return false
	}
	labels := n.Labels
	if labels == nil {
		return false
	}
	if _, ok := labels["node-role.kubernetes.io/control-plane"]; ok {
		return true
	}
	if _, ok := labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	return false
}

func nodeRole(n *corev1.Node) string {
	if isControlPlaneNode(n) {
		return "control-plane"
	}
	return "worker"
}
