package kube

import "testing"

func TestSummarizeKubeletVersionsSingle(t *testing.T) {
	g := summarizeKubeletVersions([]string{"v1.36.1", "v1.36.1"})
	if g.Count != 2 || g.Label != "v1.36.1" || g.Skewed {
		t.Fatalf("unexpected group: %+v", g)
	}
}

func TestSummarizeKubeletVersionsRange(t *testing.T) {
	g := summarizeKubeletVersions([]string{"v1.35.2", "v1.36.1", "v1.36.1"})
	if g.Count != 3 || !g.Skewed {
		t.Fatalf("expected skewed group: %+v", g)
	}
	if g.Label != "v1.35.2–v1.36.1" {
		t.Fatalf("label = %q", g.Label)
	}
}

func TestSummarizeClusterVersionsSkewedRoles(t *testing.T) {
	summary := summarizeClusterVersions("v1.36.1", []ClusterNodeItem{
		{Role: "control-plane", KubeletVersion: "v1.36.1"},
		{Role: "worker", KubeletVersion: "v1.35.2"},
	})
	if !summary.Skewed {
		t.Fatal("expected skew between control plane and workers")
	}
	if summary.ControlPlane.Label != "v1.36.1" {
		t.Fatalf("control plane = %q", summary.ControlPlane.Label)
	}
	if summary.Workers.Label != "v1.35.2" {
		t.Fatalf("workers = %q", summary.Workers.Label)
	}
}

func TestSummarizeClusterVersionsUnified(t *testing.T) {
	summary := summarizeClusterVersions("v1.36.1", []ClusterNodeItem{
		{Role: "control-plane", KubeletVersion: "v1.36.1"},
		{Role: "worker", KubeletVersion: "v1.36.1"},
	})
	if summary.Skewed {
		t.Fatal("expected unified versions")
	}
}
