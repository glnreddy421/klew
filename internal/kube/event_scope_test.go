package kube

import "testing"

func TestEventInInvestigationScope(t *testing.T) {
	scope := newInvestigationPods([]string{"payment-api-abc", "redis-0"})

	cases := []struct {
		kind, name, query string
		want            bool
	}{
		{"Pod", "redis-0", "payment", true},
		{"Pod", "other-pod", "payment", false},
		{"Pod", "other-pod", "", true},
		{"Pod", "payment-api-abc", "redis", true},
		{"Node", "node-1", "redis", true},
		{"PersistentVolumeClaim", "data", "redis", true},
		{"Deployment", "payment-api", "payment", true},
		{"Deployment", "other", "payment", false},
	}
	for _, tc := range cases {
		got := EventInInvestigationScope(tc.kind, tc.name, tc.query, scope)
		if got != tc.want {
			t.Fatalf("EventInInvestigationScope(%q,%q,%q)=%v want %v", tc.kind, tc.name, tc.query, got, tc.want)
		}
	}
}
