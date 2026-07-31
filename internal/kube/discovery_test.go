package kube

import "testing"

func TestParseQuery(t *testing.T) {
	tests := []struct {
		in               string
		kind, name, free string
	}{
		{"deploy/payment", "Deployment", "payment", ""},
		{"deployment/payment-api", "Deployment", "payment-api", ""},
		{"pod/my-pod", "Pod", "my-pod", ""},
		{"cronjob/nightly", "CronJob", "nightly", ""},
		{"cj/nightly", "CronJob", "nightly", ""},
		{"ing/payment-ingress", "Ingress", "payment-ingress", ""},
		{"hpa/payment", "HorizontalPodAutoscaler", "payment", ""},
		{"payment", "", "", "payment"},
	}
	for _, tc := range tests {
		k, n, f := ParseQuery(tc.in)
		if k != tc.kind || n != tc.name || f != tc.free {
			t.Fatalf("ParseQuery(%q) = %q,%q,%q want %q,%q,%q", tc.in, k, n, f, tc.kind, tc.name, tc.free)
		}
	}
}

func TestResourceMatchesKind(t *testing.T) {
	if !resourceMatchesKind("CronJob", "CronJob", "cronjobs", []string{"cj"}) {
		t.Fatal("expected CronJob match")
	}
	if !resourceMatchesKind("cj", "CronJob", "cronjobs", []string{"cj"}) {
		t.Fatal("expected short-name match")
	}
	if resourceMatchesKind("Deployment", "CronJob", "cronjobs", nil) {
		t.Fatal("expected non-match")
	}
}

func TestScoreNameMatch(t *testing.T) {
	if scoreNameMatch("payment-api", "payment") < 0.7 {
		t.Fatal("expected fuzzy match score")
	}
	if scoreNameMatch("payment", "payment") != 1.0 {
		t.Fatal("expected exact match")
	}
}
