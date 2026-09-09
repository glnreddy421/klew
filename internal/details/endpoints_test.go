package details

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestLegacyEndpointAddressRows(t *testing.T) {
	rows := legacyEndpointAddressRows([]corev1.EndpointSubset{{
		Addresses: []corev1.EndpointAddress{{
			IP:       "172.20.0.2",
			Hostname: "kubernetes",
		}},
		Ports: []corev1.EndpointPort{{
			Name:     "https",
			Port:     6443,
			Protocol: corev1.ProtocolTCP,
		}},
	}})
	if len(rows) != 1 || rows[0][0] != "172.20.0.2" {
		t.Fatalf("addresses = %#v", rows)
	}
	ports := legacyEndpointPortRows([]corev1.EndpointSubset{{
		Ports: []corev1.EndpointPort{{Name: "https", Port: 6443, Protocol: corev1.ProtocolTCP}},
	}})
	if len(ports) != 1 || ports[0][1] != "6443" {
		t.Fatalf("ports = %#v", ports)
	}
}
