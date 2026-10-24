// Package changes builds workload revision history (ReplicaSets,
// ControllerRevisions, Helm releases) and field-level diffs between revisions.
// It only reads from the cluster.
package changes

import (
	"regexp"
	"strings"
)

// Redacted replaces a value that looks secret.
const Redacted = "‹redacted›"

// secretKeyPattern matches names whose values must never be shown.
//
// Deliberately broad: it also masks harmless names such as CACHE_KEY_PREFIX or
// MONKEY_MODE. Over-masking is intended — a hidden harmless value costs a
// click to the manifest, a leaked credential cannot be taken back. Do not
// narrow this without weighing that trade-off.
var secretKeyPattern = regexp.MustCompile(`(?i)(pass|pwd|secret|token|key|credential|auth|cert|private|dsn|session|cookie|signature|salt)`)

var (
	// scheme://user:password@host — credentials embedded in a URL / DSN.
	urlCredentials = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.\-]*://[^/\s:@]+:[^/\s@]+@`)
	// JWT: three base64url segments.
	jwtPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}$`)
	// Long opaque tokens (base64 / hex), e.g. API keys.
	opaqueToken = regexp.MustCompile(`^[A-Za-z0-9+/=_\-]{32,}$`)
	// Well-known credential prefixes.
	knownPrefixes = []string{"-----BEGIN", "ghp_", "gho_", "github_pat_", "xox", "sk_live_", "sk-", "AKIA", "ASIA", "AIza", "glpat-"}
)

// RedactValue masks value when its key or the value itself looks secret.
// Empty values are returned as-is.
func RedactValue(key, value string) string {
	if value == "" {
		return value
	}
	if secretKeyPattern.MatchString(key) || looksSecret(value) {
		return Redacted
	}
	return value
}

func looksSecret(v string) bool {
	s := strings.TrimSpace(v)
	if urlCredentials.MatchString(s) || jwtPattern.MatchString(s) {
		return true
	}
	for _, p := range knownPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	// Long opaque strings with no spaces or path separators look like tokens.
	if opaqueToken.MatchString(s) && !strings.Contains(s, "/") && hasMixedClasses(s) {
		return true
	}
	return false
}

// hasMixedClasses avoids masking long plain words or numbers.
func hasMixedClasses(s string) bool {
	var lower, upper, digit bool
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		}
	}
	n := 0
	for _, b := range []bool{lower, upper, digit} {
		if b {
			n++
		}
	}
	return n >= 2
}
