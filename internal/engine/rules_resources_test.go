package engine

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

type obj = map[string]any

func meta(kind, name string) obj {
	return obj{"kind": kind, "metadata": obj{"name": name, "namespace": "ns"}}
}

func with(o obj, path string, v any) obj {
	keys := strings.Split(path, ".")
	cur := o
	for _, k := range keys[:len(keys)-1] {
		next, ok := cur[k].(obj)
		if !ok {
			next = obj{}
			cur[k] = next
		}
		cur = next
	}
	cur[keys[len(keys)-1]] = v
	return o
}

func cond(t, status, reason, msg string) obj {
	return obj{"type": t, "status": status, "reason": reason, "message": msg}
}

// findingsOf runs every default rule and returns the findings from rule.
func findingsOf(b model.EvidenceBundle, rule string) []model.Finding {
	var out []model.Finding
	for _, f := range DefaultRegistry().Evaluate(b).Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestResourceRules(t *testing.T) {
	pending := with(with(meta("PersistentVolumeClaim", "data"), "status.phase", "Pending"), "spec.storageClassName", "fast")
	lost := with(meta("PersistentVolumeClaim", "old"), "status.phase", "Lost")
	bound := with(meta("PersistentVolumeClaim", "ok"), "status.phase", "Bound")
	pdbUnhealthy := with(meta("PodDisruptionBudget", "api"), "status", obj{"expectedPods": int64(3), "currentHealthy": int64(1), "desiredHealthy": int64(2)})
	pdbBlocking := with(meta("PodDisruptionBudget", "db"), "status", obj{"expectedPods": int64(1), "currentHealthy": int64(1), "desiredHealthy": int64(1), "disruptionsAllowed": int64(0)})
	pdbEmpty := with(meta("PodDisruptionBudget", "gone"), "status", obj{"expectedPods": int64(0)})
	denyEgress := with(with(meta("NetworkPolicy", "lockdown"), "spec.podSelector", obj{"matchLabels": obj{"app": "api"}}), "spec.policyTypes", []any{"Egress"})
	orphanPolicy := with(meta("NetworkPolicy", "stale"), "spec.podSelector", obj{"matchLabels": obj{"app": "removed"}})
	binding := with(with(meta("RoleBinding", "reader"), "roleRef", obj{"kind": "Role", "name": "missing-role", "apiGroup": "rbac.authorization.k8s.io"}),
		"subjects", []any{obj{"kind": "ServiceAccount", "name": "ghost"}})
	gw := with(with(meta("Gateway", "public"), "spec.gatewayClassName", "nginx"), "status.conditions", []any{cond("Programmed", "False", "AddressNotAssigned", "no address")})
	gwBadClass := with(meta("Gateway", "internal"), "spec.gatewayClassName", "absent")
	route := with(with(meta("HTTPRoute", "shop"), "status.parents", []any{obj{"parentRef": obj{"name": "public"},
		"conditions": []any{cond("ResolvedRefs", "False", "BackendNotFound", "service checkout not found")}}}),
		"spec.rules", []any{obj{"backendRefs": []any{obj{"name": "checkout", "port": int64(80)}, obj{"name": "web"}}}})
	vap := with(meta("ValidatingAdmissionPolicy", "limits"), "status.typeChecking.expressionWarnings", []any{obj{"fieldRef": "spec.validations[0]", "warning": "no such key: replicas"}})
	vapBinding := with(meta("ValidatingAdmissionPolicyBinding", "limits-binding"), "spec.policyName", "deleted-policy")
	claim := meta("ResourceClaim", "gpu")
	csvFailed := with(with(with(meta("ClusterServiceVersion", "etcd.v1"), "status.phase", "Failed"), "status.reason", "InstallCheckFailed"), "status.message", "deployment not ready")
	csvOK := with(meta("ClusterServiceVersion", "redis.v2"), "status.phase", "Succeeded")
	sub := with(meta("Subscription", "etcd"), "status.conditions", []any{cond("ResolutionFailed", "True", "ConstraintsNotSatisfiable", "no operators found")})
	plan := with(meta("InstallPlan", "install-abc"), "status.phase", "RequiresApproval")
	catalog := with(meta("CatalogSource", "community"), "status.connectionState", obj{"lastObservedState": "TRANSIENT_FAILURE", "address": "community.olm:50051"})
	ext := with(meta("ClusterExtension", "argocd"), "status.conditions", []any{cond("Installed", "False", "Failed", "bundle unpack failed")})
	cat := with(meta("ClusterCatalog", "operatorhub"), "status.conditions", []any{cond("Serving", "False", "Unavailable", "catalog content not ready")})

	b := model.EvidenceBundle{
		Namespace: "ns",
		Services:  []model.ServiceSummary{{Name: "web"}},
		Pods: []model.PodSummary{
			{Name: "api-1", Labels: map[string]string{"app": "api"}, ServiceAccount: "deleted-sa", PVCRefs: []string{"data", "nonexistent"}, Ready: true, Phase: "Running"},
		},
		Events: []model.EventRecord{{Reason: "ProvisioningFailed", Message: "storageclass.storage.k8s.io \"fast\" not found",
			InvolvedObject: model.ObjectRef{Kind: "PersistentVolumeClaim", Name: "data"}}},
		Resources: map[string][]map[string]any{
			"persistentvolumeclaims":            {pending, lost, bound},
			"storageclasses":                    {meta("StorageClass", "standard")},
			"persistentvolumes":                 {with(with(meta("PersistentVolume", "pv-1"), "status.phase", "Released"), "spec.claimRef.name", "old")},
			"poddisruptionbudgets":              {pdbUnhealthy, pdbBlocking, pdbEmpty},
			"networkpolicies":                   {denyEgress, orphanPolicy},
			"serviceaccounts":                   {meta("ServiceAccount", "default")},
			"roles":                             {},
			"rolebindings":                      {binding},
			"gateways":                          {gw, gwBadClass},
			"gatewayclasses":                    {meta("GatewayClass", "nginx")},
			"httproutes":                        {route},
			"validatingadmissionpolicies":       {vap},
			"validatingadmissionpolicybindings": {vapBinding},
			"resourceclaims":                    {claim},
			"clusterserviceversions":            {csvFailed, csvOK},
			"subscriptions":                     {sub},
			"installplans":                      {plan},
			"catalogsources":                    {catalog},
			"operatorgroups":                    {meta("OperatorGroup", "a"), meta("OperatorGroup", "b")},
			"clusterextensions":                 {ext},
			"clustercatalogs":                   {cat},
		},
	}

	cases := []struct {
		rule  string
		state string
		facts []string
	}{
		{"pvc-unbound", "ClaimPending", []string{"claim data is Pending", "StorageClass fast does not exist (cluster has standard)", `ProvisioningFailed: storageclass.storage.k8s.io "fast" not found`}},
		{"pvc-unbound", "ClaimLost", []string{"claim old is Lost"}},
		{"pvc-missing", "ClaimMissing", []string{"mounts PersistentVolumeClaim nonexistent, which does not exist"}},
		{"pv-failed", "VolumeReleased", []string{"released by claim old"}},
		{"pdb", "BudgetUnhealthy", []string{"1 of 3 pods healthy; the budget needs 2"}},
		{"pdb", "BudgetBlocksEviction", []string{"allows 0 disruptions"}},
		{"pdb", "BudgetSelectsNoPods", []string{"selects no pods"}},
		{"network-policy", "DeniesEgress", []string{"denies all egress for api-1, including DNS"}},
		{"network-policy", "PolicySelectsNoPods", []string{"stale selects no pods"}},
		{"service-account-missing", "ServiceAccountMissing", []string{"ServiceAccount deleted-sa, which no longer exists"}},
		{"role-binding", "BindingDangling", []string{"Role missing-role, which does not exist", "ServiceAccount ghost, which does not exist"}},
		{"gateway", "GatewayNotProgrammed", []string{"Programmed=False: AddressNotAssigned: no address"}},
		{"gateway", "GatewayClassMissing", []string{"GatewayClass absent does not exist"}},
		{"http-route", "RouteRefsUnresolved", []string{"parent public: ResolvedRefs=False: BackendNotFound", "backend Service checkout does not exist"}},
		{"admission-policy", "PolicyTypeWarnings", []string{"spec.validations[0]: no such key: replicas"}},
		{"admission-policy", "BindingPolicyMissing", []string{"deleted-policy, which does not exist"}},
		{"resource-claim", "ClaimNotAllocated", []string{"ResourceClaim gpu is not allocated"}},
		{"olm-csv", "OperatorFailed", []string{"operator etcd.v1 is Failed: InstallCheckFailed: deployment not ready"}},
		{"olm-subscription", "ResolutionFailed", []string{"ConstraintsNotSatisfiable: no operators found"}},
		{"olm-installplan", "InstallPlanNeedsApproval", []string{"waiting for manual approval"}},
		{"olm-catalogsource", "CatalogSourceUnhealthy", []string{"TRANSIENT_FAILURE"}},
		{"olm-operatorgroup", "TooManyOperatorGroups", []string{"2 OperatorGroups (a, b)"}},
		{"olm-clusterextension", "ExtensionNotInstalled", []string{"bundle unpack failed"}},
		{"olm-clustercatalog", "CatalogNotServing", []string{"catalog content not ready"}},
	}
	for _, tc := range cases {
		t.Run(tc.rule+"/"+tc.state, func(t *testing.T) {
			var hit *model.Finding
			for _, f := range findingsOf(b, tc.rule) {
				if f.State == tc.state {
					hit = &f
					break
				}
			}
			if hit == nil {
				t.Fatalf("no %s finding from %s; got %+v", tc.state, tc.rule, findingsOf(b, tc.rule))
			}
			facts := strings.Join(hit.Facts, "; ")
			for _, w := range tc.facts {
				if !strings.Contains(facts, w) {
					t.Errorf("facts = %q, want %q", facts, w)
				}
			}
		})
	}

	for _, rule := range []string{"olm-csv", "pvc-unbound"} {
		for _, f := range findingsOf(b, rule) {
			if f.Object.Name == "redis.v2" || f.Object.Name == "ok" {
				t.Errorf("%s reported a healthy object: %+v", rule, f)
			}
		}
	}
}

func TestResourceRulesNeedTheirCollector(t *testing.T) {
	b := model.EvidenceBundle{Pods: []model.PodSummary{{Name: "p", ServiceAccount: "sa", PVCRefs: []string{"data"}}}}
	for _, rule := range []string{"pvc-missing", "service-account-missing", "olm-operatorgroup"} {
		if got := findingsOf(b, rule); len(got) != 0 {
			t.Errorf("%s without its collector's objects = %+v, want none", rule, got)
		}
	}
}

func TestClaimFindingsGroupWithTheWorkloadThatMountsThem(t *testing.T) {
	pod := ownedPod("store-0", "StatefulSet", "store", false, model.ContainerStatus{Name: "store", State: "waiting", Reason: "ContainerCreating"})
	pod.Phase, pod.PVCRefs = "Pending", []string{"data-store-0"}
	b := model.EvidenceBundle{
		Pods:      []model.PodSummary{pod},
		Resources: map[string][]map[string]any{"persistentvolumeclaims": {with(meta("PersistentVolumeClaim", "data-store-0"), "status.phase", "Pending")}},
	}
	got := problemsFor(b)
	if len(got) != 1 {
		t.Fatalf("problems = %+v, want the claim under StatefulSet/store", got)
	}
	p := got["StatefulSet/store"]
	if p.State != "ClaimPending" {
		t.Fatalf("store problem = %+v", p)
	}
	wantFacts(t, p, "claim data-store-0 is Pending")
}

// TestRulesSurviveMalformedObjects feeds every rule objects whose fields have
// the wrong types. Rules must cope on their own, not via panic recovery.
func TestRulesSurviveMalformedObjects(t *testing.T) {
	garbage := []map[string]any{
		{},
		{"metadata": "x", "spec": 5, "status": []any{1, "a"}},
		{"metadata": obj{"name": 7}, "spec": obj{"rules": []any{"r", 3, nil}, "podSelector": "app=x", "roleRef": 1, "subjects": "s"},
			"status": obj{"phase": 9, "conditions": []any{"c", obj{"type": 1}}, "parents": []any{1}, "connectionState": "x",
				"typeChecking": obj{"expressionWarnings": []any{"w"}}, "expectedPods": "3"}},
		{"kind": []any{}, "status": obj{"phase": "Failed", "conditions": obj{}}},
	}
	resources := map[string][]map[string]any{}
	for _, name := range []string{"persistentvolumeclaims", "storageclasses", "persistentvolumes", "poddisruptionbudgets",
		"networkpolicies", "serviceaccounts", "roles", "rolebindings", "gateways", "gatewayclasses", "httproutes",
		"validatingadmissionpolicies", "validatingadmissionpolicybindings", "resourceclaims", "clusterserviceversions",
		"subscriptions", "installplans", "catalogsources", "operatorgroups", "clusterextensions", "clustercatalogs",
		"cronjobs", "statefulsets", "daemonsets", "configmaps", "ingressclasses",
		"mutatingwebhookconfigurations", "validatingwebhookconfigurations"} {
		resources[name] = garbage
	}
	b := model.EvidenceBundle{
		Resources: resources,
		Pods:      []model.PodSummary{{Name: "p", PVCRefs: []string{"x"}, ServiceAccount: "sa", Labels: map[string]string{"app": "x"}}},
		Services:  []model.ServiceSummary{{Name: "s", Selector: "not a valid selector ((("}},
		Ingresses: []model.IngressSummary{{Name: "i", Backends: []string{"", "s"}}},
		Events:    []model.EventRecord{{Reason: "Unhealthy", Message: "Liveness probe failed"}},
	}
	res := DefaultRegistry().Evaluate(b)
	for _, w := range res.Warnings {
		if strings.Contains(w, "failed and was skipped") {
			t.Errorf("rule panicked: %s", w)
		}
	}
	BuildProblems(b, res.Findings)
}

func TestPanickingRuleBecomesAWarning(t *testing.T) {
	reg := &Registry{Rules: []Rule{
		NewRule("boom", func(model.EvidenceBundle) RuleResult { panic("bad index") }),
		findingRule("after", func(model.EvidenceBundle) []model.Finding {
			return []model.Finding{{Object: model.ObjectRef{Kind: "Service", Name: "s"}, State: "X", Severity: model.SeverityHigh}}
		}),
	}}
	res := reg.Evaluate(model.EvidenceBundle{})
	if len(res.Findings) != 1 || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "rule boom failed and was skipped: bad index") {
		t.Fatalf("result = %+v", res)
	}
}
