package history

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Watch APIs the recorder can use. Both expose the same Event objects (same UID).
const (
	APIEventsV1 = "events.k8s.io/v1"
	APICoreV1   = "v1"
)

// ClusterKey identifies a recorded cluster: the kube context name plus a hash
// of the API server URL, so a renamed or re-pointed context never merges with
// a different cluster's history.
func ClusterKey(contextName, serverURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimRight(strings.TrimSpace(serverURL), "/")))
	return contextName + "|" + hex.EncodeToString(sum[:6])
}

// ContextFromKey returns the context-name part of a cluster key.
func ContextFromKey(key string) string {
	if i := strings.LastIndex(key, "|"); i >= 0 {
		return key[:i]
	}
	return key
}

// lockFileName is a filesystem-safe name for a cluster key (context names may
// contain "/" and ":", e.g. EKS ARNs).
func lockFileName(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:12]) + ".lock"
}
