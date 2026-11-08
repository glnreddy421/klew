package kube

import (
	"context"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/internal/model"
)

func ownedBy(name, kind, owner string, lbl map[string]string) corev1.Pod {
	p := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "shop", Labels: lbl}}
	if kind != "" {
		p.OwnerReferences = []metav1.OwnerReference{{Kind: kind, Name: owner}}
	}
	return p
}

// lists is a namespace where names overlap on purpose: "api" is a prefix of
// "api-gateway" and a suffix of "payment-api".
func lists() *scopeLists {
	api := ownedBy("api-7c9-x", "ReplicaSet", "api-7c9", map[string]string{"app": "api"})
	api.Spec.Volumes = []corev1.Volume{
		{Name: "cfg", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "api-config"}}}},
		{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "api-data"}}},
	}
	api.Spec.ServiceAccountName = "api-sa"
	api.Spec.Containers = []corev1.Container{{Name: "api", Env: []corev1.EnvVar{{Name: "TOKEN", ValueFrom: &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "api-token"}, Key: "token"}}}}}}
	return &scopeLists{
		pods: []corev1.Pod{
			api,
			ownedBy("payment-api-5d4-y", "ReplicaSet", "payment-api-5d4", map[string]string{"app": "payment-api"}),
			ownedBy("api-gateway-0", "StatefulSet", "api-gateway", map[string]string{"app": "api-gateway"}),
			ownedBy("nightly-281-z", "Job", "nightly-281", map[string]string{"job": "nightly"}),
			ownedBy("debug", "", "", map[string]string{"app": "api"}),
		},
		rsOwner:  map[string]string{"api-7c9": "api", "api-6b1": "api", "payment-api-5d4": "payment-api"},
		jobOwner: map[string]string{"nightly-281": "nightly"},
		rsNames:  map[string]bool{"api-7c9": true, "api-6b1": true, "payment-api-5d4": true},
		jobNames: map[string]bool{"nightly-281": true},
		services: []corev1.Service{
			{ObjectMeta: metav1.ObjectMeta{Name: "api"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "api"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "payment-api"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "payment-api"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "external"}},
		},
		ingresses: []networkingv1.Ingress{{
			ObjectMeta: metav1.ObjectMeta{Name: "public"},
			Spec: networkingv1.IngressSpec{DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{Name: "api"}}},
		}},
	}
}

func TestResolveTargetByKind(t *testing.T) {
	cases := []struct {
		ref       model.ObjectRef
		pods      []string
		mustHave  []string
		mustNot   []string
		wantFound bool
	}{
		{ref: model.ObjectRef{Kind: "Deployment", Name: "api"}, pods: []string{"api-7c9-x"}, wantFound: true,
			mustHave: []string{"Deployment/api", "ReplicaSet/api-7c9", "ReplicaSet/api-6b1", "Service/api", "Ingress/public",
				"ConfigMap/api-config", "PersistentVolumeClaim/api-data", "Secret/api-token", "ServiceAccount/api-sa"},
			mustNot: []string{"Deployment/payment-api", "ReplicaSet/payment-api-5d4", "Service/payment-api", "StatefulSet/api-gateway"}},
		{ref: model.ObjectRef{Kind: "ReplicaSet", Name: "api-7c9"}, pods: []string{"api-7c9-x"}, wantFound: true,
			mustHave: []string{"Deployment/api"}},
		{ref: model.ObjectRef{Kind: "StatefulSet", Name: "api-gateway"}, pods: []string{"api-gateway-0"}, wantFound: true},
		{ref: model.ObjectRef{Kind: "CronJob", Name: "nightly"}, pods: []string{"nightly-281-z"}, wantFound: true,
			mustHave: []string{"Job/nightly-281"}},
		{ref: model.ObjectRef{Kind: "Pod", Name: "debug"}, pods: []string{"debug"}, wantFound: true,
			mustHave: []string{"Service/api"}},
		{ref: model.ObjectRef{Kind: "Service", Name: "api"}, pods: []string{"api-7c9-x", "debug"}, wantFound: true,
			mustHave: []string{"Deployment/api", "Ingress/public"}},
		{ref: model.ObjectRef{Kind: "Service", Name: "external"}, wantFound: true},
		{ref: model.ObjectRef{Kind: "Ingress", Name: "public"}, pods: []string{"api-7c9-x", "debug"}, wantFound: true,
			mustHave: []string{"Service/api"}},
		{ref: model.ObjectRef{Kind: "ConfigMap", Name: "api-config"}, pods: []string{"api-7c9-x"}, wantFound: true,
			mustHave: []string{"Deployment/api"}},
		{ref: model.ObjectRef{Kind: "Secret", Name: "api-token"}, pods: []string{"api-7c9-x"}, wantFound: true},
		{ref: model.ObjectRef{Kind: "PersistentVolumeClaim", Name: "api-data"}, pods: []string{"api-7c9-x"}, wantFound: true},
		{ref: model.ObjectRef{Kind: "Deployment", Name: "absent"}},
		{ref: model.ObjectRef{Kind: "NetworkPolicy", Name: "lockdown"}, mustHave: []string{"NetworkPolicy/lockdown"}},
	}
	for _, tc := range cases {
		t.Run(tc.ref.Kind+"/"+tc.ref.Name, func(t *testing.T) {
			got := lists().resolveTarget(tc.ref, false)
			if !slices.Equal(got.Pods, tc.pods) {
				t.Errorf("pods = %v, want %v", got.Pods, tc.pods)
			}
			if got.Found != tc.wantFound {
				t.Errorf("found = %v, want %v", got.Found, tc.wantFound)
			}
			for _, o := range tc.mustHave {
				if !slices.Contains(got.Objects, o) {
					t.Errorf("objects %v lack %s", got.Objects, o)
				}
			}
			for _, o := range tc.mustNot {
				if slices.Contains(got.Objects, o) {
					t.Errorf("objects %v include unrelated %s", got.Objects, o)
				}
			}
		})
	}
}

func TestTargetRef(t *testing.T) {
	matches := []model.MatchedObject{
		{Ref: model.ObjectRef{Kind: "Service", Name: "api"}},
		{Ref: model.ObjectRef{Kind: "Deployment", Name: "api"}},
		{Ref: model.ObjectRef{Kind: "Deployment", Name: "payment-api"}},
	}
	cases := map[string]model.ObjectRef{
		"deploy/api":         {Kind: "Deployment", Name: "api"},
		"svc/api":            {Kind: "Service", Name: "api"},
		"cm/settings":        {Kind: "ConfigMap", Name: "settings"},
		"api":                {Kind: "Deployment", Name: "api"}, // exact name; Deployment outranks Service
		"payment-api":        {Kind: "Deployment", Name: "payment-api"},
		"pay":                {}, // free text naming no object: keep name matching
		"":                   {},
		"widgets.acme/thing": {}, // not a kind
		"Certificate/web":    {Kind: "Certificate", Name: "web"},
	}
	for q, want := range cases {
		got, ok := targetRef(q, matches)
		if ok != (want.Kind != "") || got != want {
			t.Errorf("targetRef(%q) = %+v, %v; want %+v", q, got, ok, want)
		}
	}
}

func TestPodInTargetFollowsRollouts(t *testing.T) {
	sets := newTargetSets(&model.ScopeTarget{Pods: []string{"api-7c9-x"}, Objects: []string{"Deployment/api", "ReplicaSet/api-7c9"}})
	newRS := ownedBy("api-9f2-q", "ReplicaSet", "api-9f2", map[string]string{"pod-template-hash": "9f2"})
	other := ownedBy("payment-api-9f2-q", "ReplicaSet", "payment-api-9f2", map[string]string{"pod-template-hash": "9f2"})
	if !sets.podInTarget(&newRS) {
		t.Error("pod of a new ReplicaSet of the target Deployment was dropped")
	}
	if sets.podInTarget(&other) {
		t.Error("pod of payment-api leaked into an api investigation")
	}
	if !sets.eventInTarget("Node", "node-1") || sets.eventInTarget("Pod", "payment-api-5d4-y") || !sets.eventInTarget("ReplicaSet", "api-7c9") {
		t.Error("event filter wrong")
	}
}

// TestCollectDeploymentTargetIsExact runs a collection where names overlap:
// deployment/api must not pull in payment-api.
func TestCollectDeploymentTargetIsExact(t *testing.T) {
	t.Parallel()
	objs := []runtime.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "shop"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "payment-api", Namespace: "shop"}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "api-7c9", Namespace: "shop", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "api"}}}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "payment-api-5d4", Namespace: "shop", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "payment-api"}}}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "api-migrate", Namespace: "shop"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "shop"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "api"}}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "payment-api", Namespace: "shop"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "payment-api"}}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "e1", Namespace: "shop"}, Reason: "BackOff", InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "payment-api-5d4-y"}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "e2", Namespace: "shop"}, Reason: "ScalingReplicaSet", InvolvedObject: corev1.ObjectReference{Kind: "Deployment", Name: "api"}},
	}
	for _, p := range []corev1.Pod{
		ownedBy("api-7c9-x", "ReplicaSet", "api-7c9", map[string]string{"app": "api"}),
		ownedBy("payment-api-5d4-y", "ReplicaSet", "payment-api-5d4", map[string]string{"app": "payment-api"}),
	} {
		objs = append(objs, &p)
	}
	cs := fake.NewSimpleClientset(objs...)
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		review := a.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		review.Status.Allowed = true
		return true, review, nil
	})
	col := &Collector{Client: &Client{Clientset: cs, Config: &rest.Config{Host: "http://127.0.0.1:1"}, Context: "lab", Namespace: "shop"}}
	b, err := col.Collect(context.Background(), CollectOptions{Namespace: "shop", Query: "deployment/api",
		Collectors: []EvidenceCollector{
			NewCollector("workloads", collectWorkloads), NewCollector("replicasets", collectReplicaSets),
			NewCollector("pods", collectPods), NewCollector("jobs", collectJobs),
			NewCollector("services", collectServicesStep), NewCollector("events", collectEventsStep),
		}})
	if err != nil {
		t.Fatal(err)
	}
	if b.Target == nil || !b.Target.Found {
		t.Fatalf("target = %+v", b.Target)
	}
	if got := podNames(b.Pods); !slices.Equal(got, []string{"api-7c9-x"}) {
		t.Errorf("pods = %v", got)
	}
	if len(b.Workloads) != 1 || b.Workloads[0].Name != "api" {
		t.Errorf("workloads = %+v", b.Workloads)
	}
	if len(b.ReplicaSets) != 1 || b.ReplicaSets[0].Name != "api-7c9" {
		t.Errorf("replicasets = %+v", b.ReplicaSets)
	}
	if len(b.Services) != 1 || b.Services[0].Name != "api" {
		t.Errorf("services = %+v", b.Services)
	}
	if len(b.Jobs) != 0 {
		t.Errorf("job api-migrate shares a name prefix, not an owner: %+v", b.Jobs)
	}
	for _, e := range b.Events {
		if e.InvolvedObject.Name == "payment-api-5d4-y" {
			t.Errorf("payment-api event leaked: %+v", e)
		}
	}
	if len(b.Events) != 1 {
		t.Errorf("events = %+v, want the api Deployment's", b.Events)
	}
}

func TestNarrowResourcesToTarget(t *testing.T) {
	env := &CollectEnv{target: newTargetSets(&model.ScopeTarget{
		Pods:    []string{"api-7c9-x"},
		Objects: []string{"Deployment/api", "PersistentVolumeClaim/api-data", "Service/api", "ServiceAccount/api-sa"},
	})}
	b := &model.EvidenceBundle{
		Pods: []model.PodSummary{{Name: "api-7c9-x", Labels: map[string]string{"app": "api"}}},
		Resources: map[string][]map[string]any{
			"persistentvolumeclaims": {{"metadata": map[string]any{"name": "api-data"}}, {"metadata": map[string]any{"name": "other"}}},
			"poddisruptionbudgets": {
				{"metadata": map[string]any{"name": "api"}, "spec": map[string]any{"selector": map[string]any{"matchLabels": map[string]any{"app": "api"}}}},
				{"metadata": map[string]any{"name": "db"}, "spec": map[string]any{"selector": map[string]any{"matchLabels": map[string]any{"app": "db"}}}},
			},
			"networkpolicies": {{"metadata": map[string]any{"name": "all"}, "spec": map[string]any{"podSelector": map[string]any{}}}},
			"rolebindings": {
				{"metadata": map[string]any{"name": "api-read"}, "roleRef": map[string]any{"kind": "Role", "name": "reader"},
					"subjects": []any{map[string]any{"kind": "ServiceAccount", "name": "api-sa"}}},
				{"metadata": map[string]any{"name": "db-read"}, "subjects": []any{map[string]any{"kind": "ServiceAccount", "name": "db-sa"}}},
			},
			"roles":                  {{"metadata": map[string]any{"name": "reader"}}, {"metadata": map[string]any{"name": "admin"}}},
			"clusterserviceversions": {{"metadata": map[string]any{"name": "etcd.v1"}}},
			"storageclasses":         {{"metadata": map[string]any{"name": "standard"}}},
			"my-certificates":        {{"metadata": map[string]any{"name": "web"}}},
		},
	}
	narrowResourcesToTarget(context.Background(), env, b)
	names := func(kind string) []string {
		var out []string
		for _, o := range b.Resources[kind] {
			out = append(out, nestedString(o, "metadata", "name"))
		}
		return out
	}
	checks := map[string][]string{
		"persistentvolumeclaims": {"api-data"},
		"poddisruptionbudgets":   {"api"},
		"networkpolicies":        {"all"},
		"rolebindings":           {"api-read"},
		"roles":                  {"reader"},
		"clusterserviceversions": nil,
		"storageclasses":         {"standard"},
		"my-certificates":        {"web"},
	}
	for kind, want := range checks {
		if got := names(kind); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", kind, got, want)
		}
	}
}

func TestCustomKindTargetNarrowsItsCollector(t *testing.T) {
	env := &CollectEnv{target: newTargetSets(&model.ScopeTarget{Objects: []string{"Certificate/web"}})}
	b := &model.EvidenceBundle{
		Target: &model.ScopeTarget{Ref: model.ObjectRef{Kind: "Certificate", Name: "web"}},
		Resources: map[string][]map[string]any{"certificates": {
			{"kind": "Certificate", "metadata": map[string]any{"name": "web"}},
			{"kind": "Certificate", "metadata": map[string]any{"name": "api"}},
		}},
	}
	narrowResourcesToTarget(context.Background(), env, b)
	if !b.Target.Found || len(b.Resources["certificates"]) != 1 {
		t.Fatalf("found = %v, certificates = %v", b.Target.Found, b.Resources["certificates"])
	}
}

func TestMergeTargetsUnionsScopes(t *testing.T) {
	a := &model.ScopeTarget{Ref: model.ObjectRef{Kind: "Deployment", Name: "gateway"}, Found: true, Pods: []string{"gateway-1"}, Objects: []string{"Deployment/gateway"}}
	b := &model.ScopeTarget{Ref: model.ObjectRef{Kind: "Service", Name: "gateway"}, Objects: []string{"Service/gateway"}}
	m := mergeTargets(a, b)
	if m.Ref != a.Ref || !m.Found || !slices.Equal(m.Objects, []string{"Deployment/gateway", "Service/gateway"}) || !slices.Equal(m.Pods, []string{"gateway-1"}) {
		t.Fatalf("merged = %+v", m)
	}
}

func TestSplitTargets(t *testing.T) {
	cases := map[string][]string{
		"Deployment/api, Service/web": {"Deployment/api", "Service/web"},
		"Deployment/api":              {"Deployment/api"},
		"api,web":                     {"api,web"},            // not kind/name: one free-text query
		"Deployment/api,web":          {"Deployment/api,web"}, // mixed: not split
	}
	for q, want := range cases {
		if got := splitTargets(q); !slices.Equal(got, want) {
			t.Errorf("splitTargets(%q) = %v, want %v", q, got, want)
		}
	}
}

func TestCollectSeveralTargetsIsTheUnion(t *testing.T) {
	t.Parallel()
	objs := []runtime.Object{
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "api-7c9", Namespace: "shop", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "api"}}}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "web-1a2", Namespace: "shop", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "web"}}}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "db-3b4", Namespace: "shop", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "db"}}}},
	}
	for _, p := range []corev1.Pod{
		ownedBy("api-7c9-x", "ReplicaSet", "api-7c9", nil),
		ownedBy("web-1a2-y", "ReplicaSet", "web-1a2", nil),
		ownedBy("db-3b4-z", "ReplicaSet", "db-3b4", nil),
	} {
		objs = append(objs, &p)
	}
	cs := fake.NewSimpleClientset(objs...)
	allowAll(cs)
	col := &Collector{Client: &Client{Clientset: cs, Config: &rest.Config{Host: "http://127.0.0.1:1"}, Context: t.Name(), Namespace: "shop"}}
	b, err := col.Collect(context.Background(), CollectOptions{Namespace: "shop", Query: "Deployment/api,Deployment/web,Deployment/gone",
		Collectors: []EvidenceCollector{NewCollector("pods", collectPods)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := podNames(b.Pods); !slices.Equal(got, []string{"api-7c9-x", "web-1a2-y"}) {
		t.Errorf("pods = %v", got)
	}
	if !slices.Equal(b.Target.Missing, []string{"Deployment/gone"}) {
		t.Errorf("missing = %v", b.Target.Missing)
	}
	if !slices.ContainsFunc(b.Warnings, func(w string) bool { return w == "Not found in shop: Deployment/gone" }) {
		t.Errorf("warnings = %v", b.Warnings)
	}
}
