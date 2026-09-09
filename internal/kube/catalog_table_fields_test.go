package kube

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestEnrichCatalogTableFieldsStorageClass(t *testing.T) {
	entity := model.CatalogEntity{Name: "fast"}
	def := true
	entity.Provisioner = "kubernetes.io/aws-ebs"
	entity.ReclaimPolicy = "Delete"
	entity.VolumeBindingMode = "WaitForFirstConsumer"
	entity.IsDefault = &def

	enrichCatalogTableFields(&entity, map[string]interface{}{
		"provisioner":       "kubernetes.io/aws-ebs",
		"reclaimPolicy":     "Delete",
		"volumeBindingMode": "WaitForFirstConsumer",
	}, "storageclasses")

	if entity.TableFields["provisioner"] != "kubernetes.io/aws-ebs" {
		t.Fatalf("provisioner = %q", entity.TableFields["provisioner"])
	}
	if entity.TableFields["defaultClass"] != "true" {
		t.Fatalf("defaultClass = %q", entity.TableFields["defaultClass"])
	}
}

func TestEnrichCatalogTableFieldsLease(t *testing.T) {
	entity := model.CatalogEntity{Name: "kube-controller-manager", LeaseHolder: "controller-1"}
	enrichCatalogTableFields(&entity, map[string]interface{}{
		"spec": map[string]interface{}{"holderIdentity": "controller-1"},
	}, "leases")

	if entity.TableFields["holder"] != "controller-1" {
		t.Fatalf("holder = %q", entity.TableFields["holder"])
	}
}

func TestEnrichCatalogTableFieldsConfigMap(t *testing.T) {
	entity := model.CatalogEntity{
		Name: "app-config",
		ConfigMapData: []model.CatalogDataEntry{
			{Key: "config.yaml", SizeBytes: 42},
		},
	}
	enrichCatalogTableFields(&entity, map[string]interface{}{
		"data": map[string]interface{}{"config.yaml": "foo"},
	}, "configmaps")

	if entity.TableFields["dataKeys"] != "config.yaml (42 bytes)" {
		t.Fatalf("dataKeys = %q", entity.TableFields["dataKeys"])
	}
}
