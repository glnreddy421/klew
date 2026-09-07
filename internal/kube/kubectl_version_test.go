package kube

import "testing"

func TestVersionSkewRequiresDownload(t *testing.T) {
	bundled := "v1.31.4"
	cases := []struct {
		cluster string
		want    bool
	}{
		{"v1.31.0", false},
		{"v1.30.8", false},
		{"v1.29.10", true},
		{"v1.32.0", false},
		{"v1.33.0", true},
		{"v2.0.0", true},
		{"v1.31.4-gke.20240101", false},
	}
	for _, tc := range cases {
		got := versionSkewRequiresDownload(bundled, tc.cluster)
		if got != tc.want {
			t.Fatalf("versionSkewRequiresDownload(%q, %q) = %v, want %v", bundled, tc.cluster, got, tc.want)
		}
	}
}

func TestNormalizeKubectlReleaseVersion(t *testing.T) {
	if got := normalizeKubectlReleaseVersion("v1.29.4-gke.123"); got != "v1.29.4" {
		t.Fatalf("got %q", got)
	}
}
