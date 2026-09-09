package kube

import (
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// IsAuthConnectionError reports likely credential or exec-plugin auth failures (EKS/AWS SSO, etc.).
func IsAuthConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) {
		return true
	}
	lower := strings.ToLower(err.Error())
	for _, k := range []string{
		"expired",
		"token",
		"credentials",
		"authentication",
		"not authorized",
		"exec plugin",
		"get-token",
		"aws-iam-authenticator",
		"aws eks",
		"sso",
		"invalidclienttokenid",
		"no valid credential",
		"accessdenied",
	} {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

// FormatConnectionError turns low-level client errors into clearer operator messages.
func FormatConnectionError(scope string, err error) string {
	if err == nil {
		return ""
	}
	if IsAuthConnectionError(err) {
		return fmt.Sprintf("%s: authentication failed — cluster credentials may have expired (%v)", scope, err)
	}
	return fmt.Sprintf("%s: %v", scope, err)
}
