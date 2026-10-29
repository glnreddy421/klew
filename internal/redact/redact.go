// Package redact masks credentials and personal data in free text (log lines,
// event messages) before it leaves Klew.
//
// It is best effort. Patterns catch common secret shapes — authorization
// headers, JWTs, key=value credentials, URL passwords, well-known token
// prefixes, private keys, email addresses, long mixed-case tokens — but no
// pattern list can recognise every secret or every piece of personal data an
// application might log. Callers must not present redacted output as free of
// sensitive data.
//
// It errs towards over-masking: a hidden harmless value costs some context, a
// leaked credential cannot be taken back. IP addresses, hostnames, UUIDs and
// hex digests are left alone because they are what incident diagnosis runs on.
package redact

import (
	"regexp"
	"strings"
)

// Marker replaces a masked value. It matches changes.Redacted.
const Marker = "‹redacted›"

// EmailMarker replaces an email address.
const EmailMarker = "‹email›"

type rule struct {
	re   *regexp.Regexp
	repl string
	// keep, when set, leaves a match unchanged if its last submatch passes.
	keep func(value string) bool
}

// isPlaceholder spots log-template wildcards (Drain's <*>), which stand for a
// value and are not one.
func isPlaceholder(v string) bool { return strings.Trim(v, `"'`) == "<*>" }

// Order matters: whole blocks and structured forms first, so later, looser
// rules do not split them.
var rules = []rule{
	// PEM private keys, possibly spanning lines.
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)`), Marker, nil},
	// scheme://user:password@host — keep the scheme and host.
	{regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.\-]*://)[^/\s:@]+:[^/\s@]+@`), "${1}" + Marker + "@", nil},
	// Authorization: Bearer|Basic|Token <credential>.
	{regexp.MustCompile(`(?i)\b(bearer|basic|token)(\s+)[A-Za-z0-9\-._~+/]{8,}=*`), "${1}${2}" + Marker, nil},
	// JWTs anywhere in the text.
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}`), Marker, nil},
	// Well-known token prefixes: GitHub, GitLab, Slack, Stripe, OpenAI-style, AWS, Google.
	{regexp.MustCompile(`\b(?:gh[pousr]_|github_pat_|glpat-|xox[abprs]-|sk_live_|rk_live_|sk-|AKIA|ASIA|AIza)[A-Za-z0-9_\-]{10,}`), Marker, nil},
	// key=value, key: value, "key":"value" where the key names a credential.
	{regexp.MustCompile(`(?i)(["']?\b[\w.\-]*(?:password|passwd|pwd|secret|token|api[_\-]?key|apikey|access[_\-]?key|private[_\-]?key|client[_\-]?secret|credential|session[_\-]?id|cookie|signature)[\w.\-]*["']?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;&}\])]+)`), "${1}" + Marker, isPlaceholder},
	// Email addresses.
	{regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`), EmailMarker, nil},
}

// opaqueToken matches long base64/base64url-looking runs; see looksLikeToken.
var opaqueToken = regexp.MustCompile(`[A-Za-z0-9+/=_\-]{32,}`)

// String returns s with credentials and email addresses masked.
func String(s string) string {
	if s == "" {
		return s
	}
	for _, r := range rules {
		if r.keep == nil {
			s = r.re.ReplaceAllString(s, r.repl)
			continue
		}
		s = r.re.ReplaceAllStringFunc(s, func(m string) string {
			sub := r.re.FindStringSubmatch(m)
			if r.keep(sub[len(sub)-1]) {
				return m
			}
			return r.re.ReplaceAllString(m, r.repl)
		})
	}
	return opaqueToken.ReplaceAllStringFunc(s, func(m string) string {
		if looksLikeToken(m) {
			return Marker
		}
		return m
	})
}

// Strings applies String to every element, returning a new slice.
func Strings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = String(s)
	}
	return out
}

// looksLikeToken flags long runs mixing upper case, lower case and digits —
// the shape of base64 API keys. Hex digests, UUIDs, lowercase resource names
// and paths do not qualify.
func looksLikeToken(s string) bool {
	if strings.Contains(s, "/") && !strings.HasSuffix(s, "=") {
		return false
	}
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
	return lower && upper && digit
}
