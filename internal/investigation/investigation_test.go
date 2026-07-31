package investigation

import "testing"

func TestMatchKinds(t *testing.T) {
	cases := []struct {
		q, name string
		want    MatchKind
	}{
		{"payment-api", "payment-api", MatchExact},
		{"payment", "payment-api", MatchPrefix},
		{"api", "payment-api", MatchContains},
		{"nope", "payment-api", MatchNone},
	}
	for _, c := range cases {
		if got, _ := Match(c.q, c.name, nil); got != c.want {
			t.Errorf("Match(%q,%q)=%q want %q", c.q, c.name, got, c.want)
		}
	}
	if mk, ok := Match("payment", "svc", map[string]string{"app": "payment-api"}); !ok || mk != MatchLabel {
		t.Errorf("expected label match, got %q %v", mk, ok)
	}
}

func sampleDataset() Dataset {
	ns := "prod"
	r := func(k, n string) Ref { return Ref{Kind: k, Name: n, Namespace: ns} }
	rp := func(k, n string) *Ref { x := r(k, n); return &x }
	lbl := map[string]string{"app": "payment-api"}
	return Dataset{Namespace: ns, Objects: []Object{
		{Ref: r("Deployment", "payment-api"), Labels: lbl, Selector: lbl,
			ConfigMaps: []string{"payment-api-config"}, Secrets: []string{"payment-api-secret"}, ServiceAccount: "payment-api"},
		{Ref: r("ReplicaSet", "payment-api-r1"), Labels: lbl, Owner: rp("Deployment", "payment-api")},
		{Ref: r("Pod", "payment-api-a1"), Labels: lbl, Owner: rp("ReplicaSet", "payment-api-r1"), Node: "n1", ServiceAccount: "payment-api"},
		{Ref: r("Pod", "payment-api-a2"), Labels: lbl, Owner: rp("ReplicaSet", "payment-api-r1"), Node: "n2"},
		{Ref: r("Service", "payment-api"), Selector: lbl},
		{Ref: r("EndpointSlice", "payment-api-x"), Owner: rp("Service", "payment-api")},
		{Ref: r("Ingress", "payment"), Target: rp("Service", "payment-api")},
		{Ref: r("HorizontalPodAutoscaler", "payment-api"), Target: rp("Deployment", "payment-api")},
		{Ref: r("ConfigMap", "payment-api-config")},
		{Ref: r("Secret", "payment-api-secret")},
		{Ref: r("ServiceAccount", "payment-api")},
		{Ref: r("RoleBinding", "payment-api"), Target: rp("ServiceAccount", "payment-api"), Role: rp("Role", "payment-api")},
		{Ref: r("Role", "payment-api")},
		{Ref: r("Event", "e1"), Involved: rp("Pod", "payment-api-a1")},
		// unrelated workload
		{Ref: r("Deployment", "billing-api"), Labels: map[string]string{"app": "billing-api"}},
		{Ref: r("Service", "billing-api"), Selector: map[string]string{"app": "billing-api"}},
	}, Extensions: []ExtensionData{
		{Name: "Istio", CRDs: []Ref{r("VirtualService", "payment-api")}},
	}}
}

func TestDiscoverGroupsByWorkload(t *testing.T) {
	groups := Discover(sampleDataset(), "payment")
	if len(groups) != 1 {
		t.Fatalf("expected 1 payment group, got %d", len(groups))
	}
	g := groups[0]
	if g.Root.Name != "payment-api" || g.Pods != 2 || g.Service != "payment-api" {
		t.Fatalf("unexpected group: %+v", g)
	}
}

func TestBuildScopeRelationships(t *testing.T) {
	ds := sampleDataset()
	scope := BuildScope(ds, Ref{Kind: "Deployment", Name: "payment-api", Namespace: "prod"})

	if len(scope.Pods) != 2 {
		t.Errorf("expected 2 pods, got %d", len(scope.Pods))
	}
	if len(scope.Services) != 1 || len(scope.Ingresses) != 1 || len(scope.HPAs) != 1 {
		t.Errorf("networking/autoscaling not expanded: %+v", scope.Stats())
	}
	if len(scope.ConfigMaps) != 1 || len(scope.Secrets) != 1 {
		t.Errorf("config refs not expanded")
	}
	if len(scope.RoleBindings) != 1 || len(scope.ServiceAccounts) != 1 {
		t.Errorf("rbac not expanded")
	}
	// unrelated billing-api must never enter scope
	for _, d := range scope.Deployments {
		if d.Name == "billing-api" {
			t.Fatalf("unrelated workload leaked into scope")
		}
	}
	if len(scope.Extensions) != 1 || scope.Extensions[0] != "Istio" {
		t.Errorf("expected Istio extension, got %+v", scope.Extensions)
	}
	// must have a healthy relationship graph
	if len(scope.Relationships) < 10 {
		t.Errorf("expected a rich relationship graph, got %d edges", len(scope.Relationships))
	}
	// deterministic ownership edge present
	if !hasEdge(scope, "Deployment", "payment-api", "ReplicaSet", "payment-api-r1", RelOwns) {
		t.Errorf("missing Deployment→ReplicaSet owns edge")
	}
	if !hasEdge(scope, "Service", "payment-api", "Pod", "payment-api-a1", RelSelects) {
		t.Errorf("missing Service→Pod selects edge")
	}
}

func hasEdge(s InvestigationScope, fk, fn, tk, tn string, kind RelationKind) bool {
	for _, e := range s.Relationships {
		if e.From.Kind == fk && e.From.Name == fn && e.To.Kind == tk && e.To.Name == tn && e.Kind == kind {
			return true
		}
	}
	return false
}
