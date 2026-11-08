package engine

import (
	"fmt"
	"sort"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// Rules over objects the generic collectors list (EvidenceBundle.Resources).
// A rule whose collector did not run (API not installed, no permission) sees
// no objects and reports nothing.

func resourceRules() []Rule {
	return []Rule{
		findingRule("pvc-unbound", claimFindings),
		findingRule("pvc-missing", missingClaimFindings),
		findingRule("pv-failed", volumeFindings),
		findingRule("pdb", disruptionBudgetFindings),
		findingRule("network-policy", networkPolicyFindings),
		findingRule("service-account-missing", serviceAccountFindings),
		findingRule("role-binding", roleBindingFindings),
		findingRule("gateway", gatewayFindings),
		findingRule("http-route", httpRouteFindings),
		findingRule("admission-policy", admissionPolicyFindings),
		findingRule("resource-claim", resourceClaimFindings),
		findingRule("olm-csv", csvFindings),
		findingRule("olm-subscription", subscriptionFindings),
		findingRule("olm-installplan", installPlanFindings),
		findingRule("olm-catalogsource", catalogSourceFindings),
		findingRule("olm-operatorgroup", operatorGroupFindings),
		findingRule("olm-clusterextension", clusterExtensionFindings),
		findingRule("olm-clustercatalog", clusterCatalogFindings),
	}
}

// ---- helpers for unstructured objects

func nested(obj map[string]any, path ...string) any {
	var cur any = obj
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func nestedStr(obj map[string]any, path ...string) string {
	s, _ := nested(obj, path...).(string)
	return s
}

func objName(obj map[string]any) string { return nestedStr(obj, "metadata", "name") }

func objRef(obj map[string]any, kind string) model.ObjectRef {
	if k := nestedStr(obj, "kind"); k != "" {
		kind = k
	}
	return model.ObjectRef{Kind: kind, Name: objName(obj), Namespace: nestedStr(obj, "metadata", "namespace")}
}

// condition returns a status condition by type.
func condition(obj map[string]any, path []string, condType string) (status, reason, message string, ok bool) {
	conds, _ := nested(obj, path...).([]any)
	for _, c := range conds {
		m, _ := c.(map[string]any)
		if m == nil || m["type"] != condType {
			continue
		}
		return nestedStr(m, "status"), nestedStr(m, "reason"), nestedStr(m, "message"), true
	}
	return "", "", "", false
}

func statusCondition(obj map[string]any, condType string) (string, string, string, bool) {
	return condition(obj, []string{"status", "conditions"}, condType)
}

func conditionFact(condType, reason, message string) string {
	out := condType + "=False"
	if reason != "" {
		out += ": " + reason
	}
	if message != "" {
		out += ": " + message
	}
	return out
}

func names(objs []map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, o := range objs {
		out[objName(o)] = true
	}
	return out
}

func typed[T any](obj map[string]any) (T, bool) {
	var out T
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, &out)
	return out, err == nil
}

// newestEventMessage is the newest event message about kind/name.
func newestEventMessage(b model.EvidenceBundle, kind, name string) string {
	var msg string
	var newest model.Timestamp
	for _, e := range b.Events {
		if !currentEvent(b, e) || e.InvolvedObject.Kind != kind || e.InvolvedObject.Name != name {
			continue
		}
		if t := eventTime(e); msg == "" || t.After(newest) {
			msg, newest = e.Reason+": "+e.Message, t
		}
	}
	return msg
}

func podsSelectedBy(b model.EvidenceBundle, sel labels.Selector) []string {
	var out []string
	for _, p := range b.Pods {
		if sel.Matches(labels.Set(p.Labels)) {
			out = append(out, p.Name)
		}
	}
	return out
}

func listNames(ns []string, max int) string {
	if len(ns) <= max {
		return strings.Join(ns, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(ns[:max], ", "), len(ns)-max)
}

// ---- storage

func claimFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	classes, haveClasses := b.Resources["storageclasses"]
	classNames := names(classes)
	for _, pvc := range b.Resources["persistentvolumeclaims"] {
		phase := nestedStr(pvc, "status", "phase")
		if phase == "" || phase == "Bound" {
			continue
		}
		f := model.Finding{Object: objRef(pvc, "PersistentVolumeClaim"), Severity: model.SeverityHigh, State: "ClaimPending"}
		if phase == "Lost" {
			f.Severity, f.State = model.SeverityCritical, "ClaimLost"
		}
		f.Facts = append(f.Facts, fmt.Sprintf("claim %s is %s", objName(pvc), phase))
		class, hasClass := nested(pvc, "spec", "storageClassName").(string)
		switch {
		case hasClass && class != "" && haveClasses && !classNames[class]:
			sorted := make([]string, 0, len(classNames))
			for n := range classNames {
				sorted = append(sorted, n)
			}
			sort.Strings(sorted)
			f.Facts = append(f.Facts, fmt.Sprintf("StorageClass %s does not exist (cluster has %s)", class, listNames(sorted, 5)))
		case hasClass && class != "":
			f.Facts = append(f.Facts, "storageClassName "+class)
		case haveClasses && defaultStorageClass(classes) == "":
			f.Facts = append(f.Facts, "no storageClassName and the cluster has no default StorageClass")
		}
		if msg := newestEventMessage(b, "PersistentVolumeClaim", objName(pvc)); msg != "" {
			f.Facts = append(f.Facts, msg)
		}
		out = append(out, f)
	}
	return out
}

func defaultStorageClass(classes []map[string]any) string {
	for _, c := range classes {
		if nestedStr(c, "metadata", "annotations", "storageclass.kubernetes.io/is-default-class") == "true" {
			return objName(c)
		}
	}
	return ""
}

// missingClaimFindings reports pods that mount a claim that does not exist.
func missingClaimFindings(b model.EvidenceBundle) []model.Finding {
	claims, ok := b.Resources["persistentvolumeclaims"]
	if !ok {
		return nil
	}
	have := names(claims)
	var out []model.Finding
	for _, p := range b.Pods {
		for _, c := range p.PVCRefs {
			if !have[c] {
				out = append(out, model.Finding{
					Object:   model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
					Severity: model.SeverityCritical, State: "ClaimMissing",
					Facts: []string{fmt.Sprintf("pod %s mounts PersistentVolumeClaim %s, which does not exist", p.Name, c)},
				})
			}
		}
	}
	return out
}

func volumeFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, pv := range b.Resources["persistentvolumes"] {
		phase := nestedStr(pv, "status", "phase")
		claim := nestedStr(pv, "spec", "claimRef", "name")
		switch phase {
		case "Failed":
			out = append(out, model.Finding{
				Object: objRef(pv, "PersistentVolume"), Severity: model.SeverityHigh, State: "VolumeFailed",
				Facts: []string{fmt.Sprintf("volume %s for claim %s failed: %s", objName(pv), claim, nestedStr(pv, "status", "message"))},
			})
		case "Released":
			out = append(out, model.Finding{
				Object: objRef(pv, "PersistentVolume"), Severity: model.SeverityWarning, State: "VolumeReleased",
				Facts: []string{fmt.Sprintf("volume %s was released by claim %s and is not reusable until reclaimed", objName(pv), claim)},
			})
		}
	}
	return out
}

// ---- availability and traffic

func disruptionBudgetFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, obj := range b.Resources["poddisruptionbudgets"] {
		pdb, ok := typed[policyv1.PodDisruptionBudget](obj)
		if !ok {
			continue
		}
		st := pdb.Status
		f := model.Finding{Object: objRef(obj, "PodDisruptionBudget")}
		switch {
		case st.ExpectedPods == 0:
			f.Severity, f.State = model.SeverityWarning, "BudgetSelectsNoPods"
			f.Facts = []string{fmt.Sprintf("PodDisruptionBudget %s selects no pods", pdb.Name)}
		case st.CurrentHealthy < st.DesiredHealthy:
			f.Severity, f.State = model.SeverityHigh, "BudgetUnhealthy"
			f.Facts = []string{fmt.Sprintf("%d of %d pods healthy; the budget needs %d", st.CurrentHealthy, st.ExpectedPods, st.DesiredHealthy)}
		case st.DisruptionsAllowed == 0:
			f.Severity, f.State = model.SeverityWarning, "BudgetBlocksEviction"
			f.Facts = []string{fmt.Sprintf("allows 0 disruptions with %d of %d pods healthy, so node drains and evictions will wait", st.CurrentHealthy, st.ExpectedPods)}
		default:
			continue
		}
		out = append(out, f)
	}
	return out
}

func networkPolicyFindings(b model.EvidenceBundle) []model.Finding {
	if b.Query != "" {
		// Pods are narrowed to the query, so "selects no pods" would be a guess.
		return nil
	}
	var out []model.Finding
	for _, obj := range b.Resources["networkpolicies"] {
		np, ok := typed[networkingv1.NetworkPolicy](obj)
		if !ok {
			continue
		}
		sel, err := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
		if err != nil {
			continue
		}
		pods := podsSelectedBy(b, sel)
		ref := objRef(obj, "NetworkPolicy")
		if len(pods) == 0 {
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityWarning, State: "PolicySelectsNoPods",
				Facts: []string{fmt.Sprintf("NetworkPolicy %s selects no pods (podSelector %s)", np.Name, sel)}})
			continue
		}
		for _, t := range np.Spec.PolicyTypes {
			denyAll := (t == networkingv1.PolicyTypeIngress && len(np.Spec.Ingress) == 0) ||
				(t == networkingv1.PolicyTypeEgress && len(np.Spec.Egress) == 0)
			if !denyAll {
				continue
			}
			fact := fmt.Sprintf("NetworkPolicy %s denies all %s for %s", np.Name, strings.ToLower(string(t)), listNames(pods, 5))
			if t == networkingv1.PolicyTypeEgress {
				fact += ", including DNS"
			}
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityWarning, State: "Denies" + string(t), Facts: []string{fact}})
		}
	}
	return out
}

// ---- security

func serviceAccountFindings(b model.EvidenceBundle) []model.Finding {
	accounts, ok := b.Resources["serviceaccounts"]
	if !ok {
		return nil
	}
	have := names(accounts)
	var out []model.Finding
	for _, p := range b.Pods {
		if p.ServiceAccount != "" && !have[p.ServiceAccount] {
			out = append(out, model.Finding{
				Object:   model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
				Severity: model.SeverityHigh, State: "ServiceAccountMissing",
				Facts: []string{fmt.Sprintf("pod %s runs as ServiceAccount %s, which no longer exists", p.Name, p.ServiceAccount)},
			})
		}
	}
	return out
}

func roleBindingFindings(b model.EvidenceBundle) []model.Finding {
	roles, haveRoles := b.Resources["roles"]
	accounts, haveAccounts := b.Resources["serviceaccounts"]
	roleNames, accountNames := names(roles), names(accounts)
	var out []model.Finding
	for _, obj := range b.Resources["rolebindings"] {
		rb, ok := typed[rbacv1.RoleBinding](obj)
		if !ok {
			continue
		}
		var facts []string
		if rb.RoleRef.Kind == "Role" && haveRoles && !roleNames[rb.RoleRef.Name] {
			facts = append(facts, fmt.Sprintf("RoleBinding %s grants Role %s, which does not exist", rb.Name, rb.RoleRef.Name))
		}
		for _, s := range rb.Subjects {
			if s.Kind == rbacv1.ServiceAccountKind && (s.Namespace == "" || s.Namespace == b.Namespace) && haveAccounts && !accountNames[s.Name] {
				facts = append(facts, fmt.Sprintf("RoleBinding %s names ServiceAccount %s, which does not exist", rb.Name, s.Name))
			}
		}
		if len(facts) > 0 {
			out = append(out, model.Finding{Object: objRef(obj, "RoleBinding"), Severity: model.SeverityWarning, State: "BindingDangling", Facts: facts})
		}
	}
	return out
}

// ---- Gateway API

func gatewayFindings(b model.EvidenceBundle) []model.Finding {
	classes, haveClasses := b.Resources["gatewayclasses"]
	classByName := map[string]map[string]any{}
	for _, c := range classes {
		classByName[objName(c)] = c
	}
	var out []model.Finding
	for _, gw := range b.Resources["gateways"] {
		f := model.Finding{Object: objRef(gw, "Gateway"), Severity: model.SeverityHigh}
		class := nestedStr(gw, "spec", "gatewayClassName")
		if c, ok := classByName[class]; ok {
			if st, reason, msg, found := statusCondition(c, "Accepted"); found && st == "False" {
				f.State = "GatewayClassNotAccepted"
				f.Facts = append(f.Facts, fmt.Sprintf("GatewayClass %s: %s", class, conditionFact("Accepted", reason, msg)))
			}
		} else if haveClasses && class != "" {
			f.State = "GatewayClassMissing"
			f.Facts = append(f.Facts, fmt.Sprintf("GatewayClass %s does not exist", class))
		}
		for _, t := range []string{"Accepted", "Programmed"} {
			if st, reason, msg, found := statusCondition(gw, t); found && st == "False" {
				if f.State == "" {
					f.State = "GatewayNot" + t
				}
				f.Facts = append(f.Facts, conditionFact(t, reason, msg))
			}
		}
		if f.State != "" {
			out = append(out, f)
		}
	}
	return out
}

func httpRouteFindings(b model.EvidenceBundle) []model.Finding {
	services := map[string]bool{}
	for _, s := range b.Services {
		services[s.Name] = true
	}
	var out []model.Finding
	for _, route := range b.Resources["httproutes"] {
		f := model.Finding{Object: objRef(route, "HTTPRoute"), Severity: model.SeverityHigh}
		parents, _ := nested(route, "status", "parents").([]any)
		for _, p := range parents {
			pm, _ := p.(map[string]any)
			parent := nestedStr(pm, "parentRef", "name")
			for _, t := range []string{"Accepted", "ResolvedRefs"} {
				if st, reason, msg, found := condition(pm, []string{"conditions"}, t); found && st == "False" {
					if f.State == "" {
						f.State = "Route" + map[string]string{"Accepted": "NotAccepted", "ResolvedRefs": "RefsUnresolved"}[t]
					}
					f.Facts = append(f.Facts, fmt.Sprintf("parent %s: %s", parent, conditionFact(t, reason, msg)))
				}
			}
		}
		if b.Query == "" {
			for _, svc := range routeBackendServices(route, b.Namespace) {
				if !services[svc] {
					if f.State == "" {
						f.State = "RouteBackendMissing"
					}
					f.Facts = append(f.Facts, fmt.Sprintf("backend Service %s does not exist", svc))
				}
			}
		}
		if f.State != "" {
			out = append(out, f)
		}
	}
	return out
}

// routeBackendServices lists the Service backends a route sends to in ns.
func routeBackendServices(route map[string]any, ns string) []string {
	var out []string
	rules, _ := nested(route, "spec", "rules").([]any)
	for _, r := range rules {
		rm, _ := r.(map[string]any)
		refs, _ := nested(rm, "backendRefs").([]any)
		for _, ref := range refs {
			m, _ := ref.(map[string]any)
			kind, group := nestedStr(m, "kind"), nestedStr(m, "group")
			refNS := nestedStr(m, "namespace")
			if (kind == "" || kind == "Service") && group == "" && (refNS == "" || refNS == ns) {
				out = appendUniqueString(out, nestedStr(m, "name"))
			}
		}
	}
	return out
}

// ---- admission policies

func admissionPolicyFindings(b model.EvidenceBundle) []model.Finding {
	policies := b.Resources["validatingadmissionpolicies"]
	have := names(policies)
	var out []model.Finding
	for _, p := range policies {
		warnings, _ := nested(p, "status", "typeChecking", "expressionWarnings").([]any)
		if len(warnings) == 0 {
			continue
		}
		f := model.Finding{Object: objRef(p, "ValidatingAdmissionPolicy"), Severity: model.SeverityWarning, State: "PolicyTypeWarnings"}
		for _, w := range warnings {
			m, _ := w.(map[string]any)
			f.Facts = append(f.Facts, fmt.Sprintf("%s: %s", nestedStr(m, "fieldRef"), nestedStr(m, "warning")))
		}
		out = append(out, f)
	}
	for _, bind := range b.Resources["validatingadmissionpolicybindings"] {
		if name := nestedStr(bind, "spec", "policyName"); name != "" && !have[name] {
			out = append(out, model.Finding{
				Object: objRef(bind, "ValidatingAdmissionPolicyBinding"), Severity: model.SeverityWarning, State: "BindingPolicyMissing",
				Facts: []string{fmt.Sprintf("binding %s names ValidatingAdmissionPolicy %s, which does not exist", objName(bind), name)},
			})
		}
	}
	return out
}

// ---- dynamic resource allocation

func resourceClaimFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, c := range b.Resources["resourceclaims"] {
		if nested(c, "status", "allocation") != nil {
			continue
		}
		fact := fmt.Sprintf("ResourceClaim %s is not allocated", objName(c))
		if msg := newestEventMessage(b, "ResourceClaim", objName(c)); msg != "" {
			fact += "; " + msg
		}
		out = append(out, model.Finding{Object: objRef(c, "ResourceClaim"), Severity: model.SeverityWarning, State: "ClaimNotAllocated", Facts: []string{fact}})
	}
	return out
}

// ---- Operator Lifecycle Manager

func csvFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, csv := range b.Resources["clusterserviceversions"] {
		phase := nestedStr(csv, "status", "phase")
		if phase == "" || phase == "Succeeded" {
			continue
		}
		sev := model.SeverityWarning
		if phase == "Failed" {
			sev = model.SeverityCritical
		}
		fact := fmt.Sprintf("operator %s is %s", objName(csv), phase)
		if reason, msg := nestedStr(csv, "status", "reason"), nestedStr(csv, "status", "message"); reason != "" || msg != "" {
			fact += ": " + strings.TrimPrefix(reason+": "+msg, ": ")
		}
		out = append(out, model.Finding{Object: objRef(csv, "ClusterServiceVersion"), Severity: sev, State: "Operator" + phase, Facts: []string{fact}})
	}
	return out
}

func subscriptionFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, sub := range b.Resources["subscriptions"] {
		f := model.Finding{Object: objRef(sub, "Subscription"), Severity: model.SeverityHigh}
		for _, t := range []string{"ResolutionFailed", "CatalogSourcesUnhealthy", "InstallPlanFailed"} {
			if st, reason, msg, found := statusCondition(sub, t); found && st == "True" {
				if f.State == "" {
					f.State = t
				}
				f.Facts = append(f.Facts, strings.TrimSuffix(fmt.Sprintf("%s: %s: %s", t, reason, msg), ": "))
			}
		}
		if f.State == "" && nestedStr(sub, "status", "state") == "UpgradePending" && nestedStr(sub, "spec", "installPlanApproval") == "Manual" {
			f.Severity, f.State = model.SeverityWarning, "UpgradeAwaitingApproval"
			f.Facts = []string{fmt.Sprintf("subscription %s has an upgrade waiting for manual InstallPlan approval", objName(sub))}
		}
		if f.State != "" {
			out = append(out, f)
		}
	}
	return out
}

func installPlanFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, ip := range b.Resources["installplans"] {
		ref := objRef(ip, "InstallPlan")
		switch nestedStr(ip, "status", "phase") {
		case "Failed":
			f := model.Finding{Object: ref, Severity: model.SeverityHigh, State: "InstallPlanFailed"}
			if st, reason, msg, found := statusCondition(ip, "Installed"); found && st == "False" {
				f.Facts = append(f.Facts, conditionFact("Installed", reason, msg))
			}
			out = append(out, f)
		case "RequiresApproval":
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityWarning, State: "InstallPlanNeedsApproval",
				Facts: []string{fmt.Sprintf("InstallPlan %s is waiting for manual approval", objName(ip))}})
		}
	}
	return out
}

func catalogSourceFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, cs := range b.Resources["catalogsources"] {
		state := nestedStr(cs, "status", "connectionState", "lastObservedState")
		if state == "" || state == "READY" {
			continue
		}
		out = append(out, model.Finding{Object: objRef(cs, "CatalogSource"), Severity: model.SeverityHigh, State: "CatalogSourceUnhealthy",
			Facts: []string{fmt.Sprintf("catalog %s connection is %s (address %s)", objName(cs), state, nestedStr(cs, "status", "connectionState", "address"))}})
	}
	return out
}

func operatorGroupFindings(b model.EvidenceBundle) []model.Finding {
	groups, ok := b.Resources["operatorgroups"]
	if !ok {
		return nil
	}
	switch {
	case len(groups) > 1:
		var ns []string
		for _, g := range groups {
			ns = append(ns, objName(g))
		}
		return []model.Finding{{
			Object: model.ObjectRef{Kind: "Namespace", Name: b.Namespace}, Severity: model.SeverityHigh, State: "TooManyOperatorGroups",
			Facts: []string{fmt.Sprintf("%d OperatorGroups (%s); OLM installs nothing in a namespace with more than one", len(groups), listNames(ns, 5))},
		}}
	case len(groups) == 0 && len(b.Resources["subscriptions"]) > 0:
		return []model.Finding{{
			Object: model.ObjectRef{Kind: "Namespace", Name: b.Namespace}, Severity: model.SeverityHigh, State: "NoOperatorGroup",
			Facts: []string{"the namespace has Subscriptions but no OperatorGroup, so OLM will not install them"},
		}}
	}
	return nil
}

func clusterExtensionFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, ce := range b.Resources["clusterextensions"] {
		f := model.Finding{Object: objRef(ce, "ClusterExtension"), Severity: model.SeverityHigh}
		if st, reason, msg, found := statusCondition(ce, "Installed"); found && st == "False" {
			f.State = "ExtensionNotInstalled"
			f.Facts = append(f.Facts, conditionFact("Installed", reason, msg))
		}
		if st, reason, msg, found := statusCondition(ce, "Progressing"); found && st == "True" && reason == "Retrying" {
			if f.State == "" {
				f.State = "ExtensionRetrying"
			}
			f.Facts = append(f.Facts, "Progressing: Retrying: "+msg)
		}
		if f.State != "" {
			out = append(out, f)
		}
	}
	return out
}

func clusterCatalogFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, cc := range b.Resources["clustercatalogs"] {
		if st, reason, msg, found := statusCondition(cc, "Serving"); found && st == "False" {
			out = append(out, model.Finding{Object: objRef(cc, "ClusterCatalog"), Severity: model.SeverityWarning, State: "CatalogNotServing",
				Facts: []string{conditionFact("Serving", reason, msg)}})
		}
	}
	return out
}
