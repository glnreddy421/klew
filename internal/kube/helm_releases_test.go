package kube

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestDecodeHelmReleasePayload(t *testing.T) {
	payload := helmReleasePayload{
		Name:      "kyverno",
		Namespace: "kyverno",
		Version:   2,
	}
	payload.Info.Status = "deployed"
	payload.Info.LastDeployed = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	payload.Chart.Metadata.Name = "kyverno"
	payload.Chart.Metadata.Version = "3.2.1"
	payload.Chart.Metadata.AppVersion = "v1.12.0"
	payload.Config = map[string]interface{}{"replicas": float64(3)}
	payload.Chart.Values = map[string]interface{}{"replicas": float64(1)}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	encoded := []byte(base64.StdEncoding.EncodeToString(gz.Bytes()))

	got, err := decodeHelmReleasePayload(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "kyverno" || got.Version != 2 {
		t.Fatalf("name/version = %q %d", got.Name, got.Version)
	}
	if got.Chart.Metadata.Name != "kyverno" || got.Chart.Metadata.Version != "3.2.1" {
		t.Fatalf("chart = %q %q", got.Chart.Metadata.Name, got.Chart.Metadata.Version)
	}
}

func TestHelmRecordFromStorage(t *testing.T) {
	payload := helmReleasePayload{
		Name:      "payment-api",
		Namespace: "klew-lab",
		Version:   1,
	}
	payload.Info.Status = "deployed"
	payload.Chart.Metadata.Name = "payment-api"
	payload.Chart.Metadata.Version = "0.1.0"
	raw, _ := json.Marshal(payload)

	rec, err := helmRecordFromStorage(helmStorageObject{
		kind:      "Secret",
		namespace: "klew-lab",
		name:      "sh.helm.release.v1.payment-api.v1",
		uid:       "abc",
		labels: map[string]string{
			"owner":   "helm",
			"name":    "payment-api",
			"version": "1",
		},
		data: raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Name != "payment-api" || rec.Revision != 1 {
		t.Fatalf("release = %q rev %d", rec.Name, rec.Revision)
	}
	if rec.Chart != "payment-api" || rec.Status != "deployed" {
		t.Fatalf("chart/status = %q %q", rec.Chart, rec.Status)
	}
	if rec.ConfigYAML == "" {
		t.Fatal("expected config yaml")
	}
	if rec.StorageKind != "Secret" || rec.StorageName == "" {
		t.Fatalf("storage = %q %q", rec.StorageKind, rec.StorageName)
	}
}

func TestHelmReleaseNameFromLabels(t *testing.T) {
	name := helmReleaseNameFromLabels(map[string]string{"name": "kyverno"}, "sh.helm.release.v1.kyverno.v3")
	if name != "kyverno" {
		t.Fatalf("name = %q", name)
	}
	name = helmReleaseNameFromLabels(nil, "sh.helm.release.v1.demo.v5")
	if name != "demo" {
		t.Fatalf("parsed name = %q", name)
	}
}

func TestIsVirtualCatalogResource(t *testing.T) {
	if !IsVirtualCatalogResource(VirtualHelmReleasesResourceID) {
		t.Fatal("expected helm releases to be virtual")
	}
	if IsVirtualCatalogResource("v1/secrets") {
		t.Fatal("secrets should not be virtual")
	}
}
