package kube

import (
	"errors"
	"fmt"

	"github.com/glnreddy421/klew/internal/model"
)

// ErrNamespaceRequired means the caller must select a namespace interactively
// or pass -n explicitly. Context namespace is a hint only, not a boundary.
var ErrNamespaceRequired = errors.New("namespace boundary required")

// ResolveNamespace returns the explicit -n flag value, or empty.
func ResolveNamespace(flagNS, contextNS string) string {
	ns, _, err := RequireNamespaceScope(flagNS, contextNS, false)
	if err != nil {
		return ""
	}
	return ns
}

// RequireNamespaceScope ensures live investigation targets an explicit scope.
// Accepts -n or -A only. Context namespace does not substitute for selection.
func RequireNamespaceScope(flagNS, contextNS string, allNS bool) (string, bool, error) {
	_ = contextNS
	if allNS {
		return "", true, nil
	}
	if flagNS != "" {
		return flagNS, false, nil
	}
	return "", false, ErrNamespaceRequired
}

// RequireNamespaceOrError is for non-interactive commands that cannot show the picker.
func RequireNamespaceOrError(flagNS string, allNS bool) (string, bool, error) {
	if allNS {
		return "", true, nil
	}
	if flagNS != "" {
		return flagNS, false, nil
	}
	return "", false, fmt.Errorf(
		"namespace required: pass -n <namespace> or omit --no-tui to select a namespace interactively",
	)
}

// ScopeFromFlags builds namespace scope for investigation.
func ScopeFromFlags(flagNS string, allNamespaces bool, clientNS string) model.NamespaceScope {
	if allNamespaces {
		return model.NamespaceScope{AllNamespaces: true, Primary: "*"}
	}
	ns := ResolveNamespace(flagNS, clientNS)
	return model.NamespaceScope{AllNamespaces: false, Primary: ns, Namespaces: []string{ns}}
}

// CollectNamespace returns the namespace passed to namespaced API calls.
func CollectNamespace(scope model.NamespaceScope) (string, error) {
	if scope.AllNamespaces {
		return "", fmt.Errorf("use per-namespace collection when AllNamespaces is set")
	}
	if scope.Primary == "" {
		return "default", nil
	}
	return scope.Primary, nil
}
