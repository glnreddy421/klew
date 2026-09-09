package kube

import "strings"

// Virtual catalog resource IDs — not discovered via the Kubernetes API.
const (
	VirtualHelmReleasesResourceID = "klew/virtual/v1/helmreleases"
)

// IsVirtualCatalogResource reports whether resourceID is a Klew virtual browse kind.
func IsVirtualCatalogResource(resourceID string) bool {
	return strings.HasPrefix(strings.TrimSpace(resourceID), "klew/virtual/")
}
