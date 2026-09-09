package details

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
)

func countEndpointSliceAddressTotals(slices []discoveryv1.EndpointSlice) (ready, total int) {
	for _, sl := range slices {
		r, t := countEndpointSliceAddresses(sl)
		ready += r
		total += t
	}
	return ready, total
}

func countEndpointSliceAddresses(sl discoveryv1.EndpointSlice) (ready, total int) {
	for _, ep := range sl.Endpoints {
		n := len(ep.Addresses)
		if n == 0 {
			continue
		}
		total += n
		isReady := ep.Conditions.Ready == nil || *ep.Conditions.Ready
		if isReady {
			ready += n
		}
	}
	return ready, total
}

func endpointStatusBadge(ready, total int) StatusBadge {
	if total == 0 {
		return StatusBadge{Tone: "warning", Label: "No endpoints"}
	}
	if ready < total {
		return StatusBadge{Tone: "warning", Label: fmt.Sprintf("%d/%d ready", ready, total)}
	}
	return StatusBadge{Tone: "healthy", Label: fmt.Sprintf("%d/%d ready", ready, total)}
}

func legacyEndpointAddressRows(subsets []corev1.EndpointSubset) [][]string {
	var rows [][]string
	for _, sub := range subsets {
		for _, addr := range sub.Addresses {
			rows = append(rows, legacyEndpointAddressRow(addr, true))
		}
		for _, addr := range sub.NotReadyAddresses {
			rows = append(rows, legacyEndpointAddressRow(addr, false))
		}
	}
	return rows
}

func legacyEndpointAddressRow(addr corev1.EndpointAddress, ready bool) []string {
	target := ""
	if addr.TargetRef != nil {
		target = addr.TargetRef.Kind + "/" + addr.TargetRef.Name
	}
	node := ""
	if addr.NodeName != nil {
		node = *addr.NodeName
	}
	return []string{addr.IP, addr.Hostname, node, target, boolStr(ready)}
}

func legacyEndpointPortRows(subsets []corev1.EndpointSubset) [][]string {
	var rows [][]string
	for _, sub := range subsets {
		for _, p := range sub.Ports {
			rows = append(rows, []string{p.Name, fmtInt32(p.Port), string(p.Protocol), ""})
		}
	}
	return rows
}

func serviceLoadBalancerIngressRows(svc *corev1.Service) [][]string {
	if svc == nil || len(svc.Status.LoadBalancer.Ingress) == 0 {
		return nil
	}
	var rows [][]string
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		ipMode := ""
		if ing.IPMode != nil {
			ipMode = string(*ing.IPMode)
		}
		rows = append(rows, []string{ing.IP, ing.Hostname, ipMode, formatLoadBalancerPortStatus(ing.Ports)})
	}
	return rows
}

func formatLoadBalancerPortStatus(ports []corev1.PortStatus) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		part := fmtInt32(p.Port)
		if p.Protocol != "" {
			part += "/" + string(p.Protocol)
		}
		if p.Error != nil {
			part += " (" + *p.Error + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func joinStringSlice(items []string) string {
	return strings.Join(items, ", ")
}

func serviceEndpointSummaryParts(slices []discoveryv1.EndpointSlice, ep *corev1.Endpoints, svc *corev1.Service) []string {
	if len(slices) > 0 {
		return endpointSliceSummaryParts(slices, svc)
	}
	if ep != nil && len(ep.Subsets) > 0 {
		return legacyEndpointSummaryParts(ep)
	}
	return nil
}

func endpointSliceSummaryParts(slices []discoveryv1.EndpointSlice, svc *corev1.Service) []string {
	fallbackPorts := servicePortNumbers(svc)
	var parts []string
	seen := make(map[string]struct{})
	for _, sl := range slices {
		ports := endpointSlicePortNumbers(sl.Ports)
		if len(ports) == 0 {
			ports = fallbackPorts
		}
		for _, ep := range sl.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			for _, addr := range ep.Addresses {
				if len(ports) == 0 {
					addUniqueEndpoint(&parts, seen, addr)
					continue
				}
				for _, port := range ports {
					addUniqueEndpoint(&parts, seen, fmt.Sprintf("%s:%d", addr, port))
				}
			}
		}
	}
	return parts
}

func legacyEndpointSummaryParts(ep *corev1.Endpoints) []string {
	var parts []string
	seen := make(map[string]struct{})
	for _, sub := range ep.Subsets {
		ports := subsetPortNumbers(sub.Ports)
		for _, addr := range sub.Addresses {
			if len(ports) == 0 {
				addUniqueEndpoint(&parts, seen, addr.IP)
				continue
			}
			for _, port := range ports {
				addUniqueEndpoint(&parts, seen, fmt.Sprintf("%s:%s", addr.IP, port))
			}
		}
		for _, addr := range sub.NotReadyAddresses {
			if len(ports) == 0 {
				addUniqueEndpoint(&parts, seen, addr.IP)
				continue
			}
			for _, port := range ports {
				addUniqueEndpoint(&parts, seen, fmt.Sprintf("%s:%s", addr.IP, port))
			}
		}
	}
	return parts
}

func addUniqueEndpoint(parts *[]string, seen map[string]struct{}, value string) {
	if value == "" {
		return
	}
	if _, ok := seen[value]; ok {
		return
	}
	seen[value] = struct{}{}
	*parts = append(*parts, value)
}

func servicePortNumbers(svc *corev1.Service) []int32 {
	if svc == nil {
		return nil
	}
	out := make([]int32, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		if p.Port != 0 {
			out = append(out, p.Port)
		}
	}
	return out
}

func endpointSlicePortNumbers(ports []discoveryv1.EndpointPort) []int32 {
	out := make([]int32, 0, len(ports))
	for _, p := range ports {
		if p.Port != nil && *p.Port != 0 {
			out = append(out, *p.Port)
		}
	}
	return out
}

func subsetPortNumbers(ports []corev1.EndpointPort) []string {
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		if p.Port != 0 {
			out = append(out, fmtInt32(p.Port))
		}
	}
	return out
}
