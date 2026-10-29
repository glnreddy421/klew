package redact

import (
	"strings"
	"testing"
)

func TestStringMasksSecrets(t *testing.T) {
	cases := map[string]struct {
		in     string
		leaked string // must not survive
		kept   string // must survive (context around the secret)
	}{
		"bearer header":   {"GET /v1/orders Authorization: Bearer abcDEF1234567890xyz", "abcDEF1234567890xyz", "GET /v1/orders"},
		"basic auth":      {"auth header Basic dXNlcjpwYXNzd29yZA==", "dXNlcjpwYXNzd29yZA==", "auth header"},
		"jwt":             {"token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c rejected", "eyJhbGciOiJIUzI1NiJ9", "rejected"},
		"password kv":     {"connecting user=svc_pay password=hunter2-prod host=db-0", "hunter2-prod", "host=db-0"},
		"json secret":     {`{"client_secret":"s3cr3t-value","region":"us-east-1"}`, "s3cr3t-value", "us-east-1"},
		"api key colon":   {"X-Api-Key: 9f8e7d6c5b4a", "9f8e7d6c5b4a", "X-Api-Key"},
		"url credentials": {"dial postgres://app:Sup3rS3cret@db-0.prod:5432/orders failed", "Sup3rS3cret", "db-0.prod:5432/orders"},
		"aws key":         {"using AKIAIOSFODNN7EXAMPLE for s3", "AKIAIOSFODNN7EXAMPLE", "for s3"},
		"github token":    {"clone with ghp_1234567890abcdefghijklmnopqrstuvwxyz", "ghp_1234567890abcdefghij", "clone with"},
		"slack token":     {"posting via xoxb-123456789012-abcdefghijkl", "xoxb-123456789012", "posting via"},
		"email":           {"payment failed for jane.doe+test@example.com order 88231", "jane.doe+test@example.com", "order 88231"},
		"opaque base64":   {"signed with Zm9vYmFyQmF6UXV4MTIzNDU2Nzg5MEFCQ0RFRkdISUpL now", "Zm9vYmFyQmF6UXV4MTIzNDU2Nzg5MEFCQ0RFRkdISUpL", "now"},
		"private key":     {"key: -----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA\n-----END RSA PRIVATE KEY----- loaded", "MIIEpAIBAAKCAQEA", "loaded"},
		"truncated pem":   {"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASC", "MIIEvQIBADANBgkqhkiG9w0BAQEFAASC", ""},
		"session id":      {"session_id=Ab12Cd34 expired", "Ab12Cd34", "expired"},
		"quoted single":   {"token='abc 123 def' refused", "abc 123 def", "refused"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := String(tc.in)
			if strings.Contains(got, tc.leaked) {
				t.Errorf("secret leaked\n in:  %q\n out: %q", tc.in, got)
			}
			if tc.kept != "" && !strings.Contains(got, tc.kept) {
				t.Errorf("context lost: %q missing\n out: %q", tc.kept, got)
			}
		})
	}
}

func TestStringKeepsDiagnosticText(t *testing.T) {
	// Values incident diagnosis depends on must pass through unchanged.
	for _, in := range []string{
		"Back-off restarting failed container api in pod payment-api-84bd756749-j4tsh",
		"dial tcp 10.96.12.4:5432: connect: connection refused",
		"OOMKilled: container exceeded memory limit 256Mi (exit 137)",
		"Pulling image ghcr.io/acme/payment-api@sha256:3f1b2c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"request 550e8400-e29b-41d4-a716-446655440000 timed out after 30s",
		"Readiness probe failed: HTTP probe failed with statuscode: 503",
		"Successfully assigned klew-lab/hello-world-29885530-d6kq7 to desktop-worker",
		"payment failed for order <*>: connection refused",
		"auth failed password=<*> for <*>",
		`login "token":"<*>" rejected`,
		"token bucket refilled",
		"reading /var/run/secrets/kubernetes.io/serviceaccount/namespace",
	} {
		if got := String(in); got != in {
			t.Errorf("over-redacted diagnostic text\n in:  %q\n out: %q", in, got)
		}
	}
}

func TestStringsCopies(t *testing.T) {
	in := []string{"password=x1y2z3", "ok"}
	out := Strings(in)
	if in[0] != "password=x1y2z3" {
		t.Fatal("Strings modified its input")
	}
	if out[0] != "password="+Marker || out[1] != "ok" {
		t.Fatalf("Strings = %q", out)
	}
	if Strings(nil) != nil {
		t.Fatal("Strings(nil) should be nil")
	}
}
