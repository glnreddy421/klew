package kube

import (
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCountReadyEndpointSliceAddresses(t *testing.T) {
	t.Parallel()
	ready := true
	notReady := false
	gotReady, gotTotal := countReadyEndpointSliceAddresses([]discoveryv1.EndpointSlice{{
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1", "10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
			{Addresses: []string{"10.0.0.3"}, Conditions: discoveryv1.EndpointConditions{Ready: &notReady}},
			{Addresses: []string{"10.0.0.4"}, Conditions: discoveryv1.EndpointConditions{}},
		},
	}})
	if gotReady != 3 || gotTotal != 3 {
		t.Fatalf("got ready=%d total=%d, want 3/3", gotReady, gotTotal)
	}
}

func TestCountReadyEndpointSliceAddressesEmpty(t *testing.T) {
	t.Parallel()
	gotReady, gotTotal := countReadyEndpointSliceAddresses(nil)
	if gotReady != 0 || gotTotal != 0 {
		t.Fatalf("got ready=%d total=%d, want 0/0", gotReady, gotTotal)
	}
}

func ptrBool(v bool) *bool { return &v }

func TestCountReadyEndpointSliceAddressesMultipleSlices(t *testing.T) {
	t.Parallel()
	slices := []discoveryv1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Name: "svc-a"}, Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptrBool(true)}},
		}},
		{ObjectMeta: metav1.ObjectMeta{Name: "svc-b"}, Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: ptrBool(true)}},
		}},
	}
	gotReady, gotTotal := countReadyEndpointSliceAddresses(slices)
	if gotReady != 2 || gotTotal != 2 {
		t.Fatalf("got ready=%d total=%d, want 2/2", gotReady, gotTotal)
	}
}
