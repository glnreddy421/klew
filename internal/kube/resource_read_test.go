package kube

import (
	"strings"
	"testing"
)

func TestResolveResourceType(t *testing.T) {
	resources := []DiscoveredResource{
		{Group: "", Version: "v1", Resource: "pods", Kind: "Pod", Namespaced: true, ShortNames: []string{"po"}},
		{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment", Namespaced: true, ShortNames: []string{"deploy"}},
		{Group: "", Version: "v1", Resource: "events", Kind: "Event", Namespaced: true, ShortNames: []string{"ev"}},
		{Group: "events.k8s.io", Version: "v1", Resource: "events", Kind: "Event", Namespaced: true, ShortNames: []string{"ev"}},
		{Group: "cert-manager.io", Version: "v1", Resource: "certificates", Kind: "Certificate", Namespaced: true},
		{Group: "certs.example.com", Version: "v1", Resource: "certificates", Kind: "Certificate", Namespaced: true},
	}
	cases := map[string]string{
		"Deployment":                   "apps/deployments",
		"deployments":                  "apps/deployments",
		"deploy":                       "apps/deployments",
		"DEPLOYMENT":                   "apps/deployments",
		"po":                           "/pods",
		"events":                       "/events", // core wins over events.k8s.io
		"events.events.k8s.io":         "events.k8s.io/events",
		"certificates.cert-manager.io": "cert-manager.io/certificates",
		"certificate.cert-manager.io":  "cert-manager.io/certificates",
	}
	for in, want := range cases {
		r, err := ResolveResourceType(resources, in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got := r.Group + "/" + r.Resource; got != want {
			t.Errorf("%q: got %s, want %s", in, got, want)
		}
	}

	if _, err := ResolveResourceType(resources, "Certificate"); err == nil || !strings.Contains(err.Error(), "certificates.cert-manager.io") {
		t.Errorf("ambiguous custom kind: want error naming the groups, got %v", err)
	}
	if _, err := ResolveResourceType(resources, "widgets"); err == nil || !strings.Contains(err.Error(), "no resource type") {
		t.Errorf("unknown type: got %v", err)
	}
	if _, err := ResolveResourceType(resources, " "); err == nil {
		t.Error("empty type: want error")
	}
}
