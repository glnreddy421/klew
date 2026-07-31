package details

import (
	"context"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
)

type ingressProvider struct{}

func (ingressProvider) Kind() string { return "Ingress" }

func (ingressProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	ing, err := getIngress(ctx, req)
	if err != nil {
		return nil, err
	}
	class := ""
	if ing.Spec.IngressClassName != nil {
		class = *ing.Spec.IngressClassName
	}
	detail := &ObjectDetail{
		Title:    "Ingress/" + ing.Name,
		Category: "network",
		Status:   StatusBadge{Tone: "healthy", Label: "Configured"},
		Summary: fields(
			"IngressClass", class,
			"Hosts", strings.Join(ingressHosts(ing), ", "),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("ingressClass", "IngressClass", GroupSpec, fields("Class", class)))

	if rows := ingressRuleRows(ing.Spec.Rules); len(rows) > 0 {
		sections = append(sections, sectionTable("rules", "Rules", GroupSpec,
			[]string{"Host", "Path", "Path Type", "Service", "Port"}, rows))
	}
	if hosts := ingressHosts(ing); len(hosts) > 0 {
		var rows [][]string
		for _, h := range hosts {
			rows = append(rows, []string{h})
		}
		sections = append(sections, sectionTable("hosts", "Hosts", GroupSpec, []string{"Host"}, rows))
	}
	if rows := ingressBackendRows(ing); len(rows) > 0 {
		sections = append(sections, sectionTable("backendServices", "Backend Services", GroupRelationships,
			[]string{"Service", "Port"}, rows))
	}
	if rows := ingressTLSRows(ing.Spec.TLS); len(rows) > 0 {
		sections = append(sections, sectionTable("tls", "TLS", GroupSpec,
			[]string{"Secret", "Hosts"}, rows))
	}
	if addrs := ingressAddresses(ing); len(addrs) > 0 {
		sections = append(sections, sectionFields("address", "Address", GroupStatus, fields(
			"LoadBalancer", strings.Join(addrs, ", "),
		)))
	}
	sections = append(sections, metaSections(ing.Labels, ing.Annotations, ownerRefsFromMeta(ing.OwnerReferences, ing.Namespace))...)
	if mf := managedFieldsSection(ing.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func ingressHosts(ing *networkingv1.Ingress) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range ing.Spec.Rules {
		if r.Host != "" && !seen[r.Host] {
			seen[r.Host] = true
			out = append(out, r.Host)
		}
	}
	for _, t := range ing.Spec.TLS {
		for _, h := range t.Hosts {
			if !seen[h] {
				seen[h] = true
				out = append(out, h)
			}
		}
	}
	return out
}

func ingressRuleRows(rules []networkingv1.IngressRule) [][]string {
	var rows [][]string
	for _, r := range rules {
		if r.HTTP == nil {
			continue
		}
		for _, p := range r.HTTP.Paths {
			svc, port := "", ""
			if p.Backend.Service != nil {
				svc = p.Backend.Service.Name
				if p.Backend.Service.Port.Number != 0 {
					port = fmtInt32(p.Backend.Service.Port.Number)
				} else {
					port = p.Backend.Service.Port.Name
				}
			}
			pt := ""
			if p.PathType != nil {
				pt = string(*p.PathType)
			}
			rows = append(rows, []string{r.Host, p.Path, pt, svc, port})
		}
	}
	return rows
}

func ingressBackendRows(ing *networkingv1.Ingress) [][]string {
	seen := map[string]bool{}
	var rows [][]string
	add := func(svc, port string) {
		key := svc + ":" + port
		if svc == "" || seen[key] {
			return
		}
		seen[key] = true
		rows = append(rows, []string{svc, port})
	}
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		s := ing.Spec.DefaultBackend.Service
		port := s.Port.Name
		if s.Port.Number != 0 {
			port = fmtInt32(s.Port.Number)
		}
		add(s.Name, port)
	}
	for _, r := range ing.Spec.Rules {
		if r.HTTP == nil {
			continue
		}
		for _, p := range r.HTTP.Paths {
			if p.Backend.Service == nil {
				continue
			}
			s := p.Backend.Service
			port := s.Port.Name
			if s.Port.Number != 0 {
				port = fmtInt32(s.Port.Number)
			}
			add(s.Name, port)
		}
	}
	return rows
}

func ingressTLSRows(tls []networkingv1.IngressTLS) [][]string {
	var rows [][]string
	for _, t := range tls {
		rows = append(rows, []string{t.SecretName, strings.Join(t.Hosts, ", ")})
	}
	return rows
}

func ingressAddresses(ing *networkingv1.Ingress) []string {
	var out []string
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			out = append(out, lb.IP)
		}
		if lb.Hostname != "" {
			out = append(out, lb.Hostname)
		}
	}
	return out
}
