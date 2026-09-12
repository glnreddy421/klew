package kube

import "testing"

func TestFormatCPUMilli(t *testing.T) {
	if got := FormatCPUMilli(250); got != "250m" {
		t.Fatalf("FormatCPUMilli(250) = %q", got)
	}
	if got := FormatCPUMilli(2000); got != "2" {
		t.Fatalf("FormatCPUMilli(2000) = %q", got)
	}
}

func TestFormatMemMi(t *testing.T) {
	if got := FormatMemMi(512); got != "512Mi" {
		t.Fatalf("FormatMemMi(512) = %q", got)
	}
	if got := FormatMemMi(2048); got != "2Gi" {
		t.Fatalf("FormatMemMi(2048) = %q", got)
	}
}

func TestPodResourceTotalsFromObject(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "app",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "100m",
							"memory": "128Mi",
						},
						"limits": map[string]interface{}{
							"cpu":    "500m",
							"memory": "512Mi",
						},
					},
				},
			},
		},
	}
	reqCPU, reqMem, limCPU, limMem := podResourceTotalsFromObject(obj)
	if reqCPU != 100 || reqMem != 128 || limCPU != 500 || limMem != 512 {
		t.Fatalf("totals = %d/%d/%d/%d", reqCPU, reqMem, limCPU, limMem)
	}
}
