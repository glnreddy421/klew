package kube

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

// bigNamespaceClient is namespace "big" with n Deployments, each with a
// ReplicaSet, a Service with one ready endpoint, and two pods spread over
// four nodes. Deployment app0 has a second revision with a new image. It
// counts every API call by verb and resource.
func bigNamespaceClient(t *testing.T, n int) (*Client, func() map[string]int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/apis/apps/v1/namespaces/big/deployments" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","code":404}`)
			return
		}
		items := make([]string, n)
		for i := range items {
			items[i] = fmt.Sprintf(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"app%d","namespace":"big"}}`, i)
		}
		fmt.Fprintf(w, `{"kind":"DeploymentList","apiVersion":"apps/v1","items":[%s]}`, strings.Join(items, ","))
	}))
	t.Cleanup(srv.Close)

	template := func(image string, lbl map[string]string) corev1.PodTemplateSpec {
		return corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: lbl},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: image}}},
		}
	}
	var objs []runtime.Object
	for i := 0; i < 4; i++ {
		objs = append(objs, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("node%d", i)}})
	}
	ready := true
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("app%d", i)
		uid := types.UID("uid-" + name)
		lbl := map[string]string{"app": name}
		two := int32(2)
		owner := []metav1.OwnerReference{{Kind: "Deployment", Name: name, UID: uid}}
		objs = append(objs,
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "big", UID: uid, Generation: 1},
				Spec:       appsv1.DeploymentSpec{Replicas: &two, Selector: &metav1.LabelSelector{MatchLabels: lbl}, Template: template("img:2", lbl)},
			},
			&appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-v2", Namespace: "big", Labels: lbl, OwnerReferences: owner,
					Annotations: map[string]string{"deployment.kubernetes.io/revision": "2"}},
				Spec: appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: lbl}, Template: template("img:2", lbl)},
			},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "big"}, Spec: corev1.ServiceSpec{Selector: lbl}},
			&discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-abc", Namespace: "big", Labels: map[string]string{discoveryv1.LabelServiceName: name}},
				Endpoints:  []discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}}},
			},
		)
		if i == 0 {
			objs = append(objs, &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-v1", Namespace: "big", Labels: lbl, OwnerReferences: owner,
					Annotations: map[string]string{"deployment.kubernetes.io/revision": "1"}},
				Spec: appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: lbl}, Template: template("img:1", lbl)},
			})
		}
		for p := 0; p < 2; p++ {
			objs = append(objs, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-v2-%d", name, p), Namespace: "big", Labels: lbl,
					OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: name + "-v2"}}},
				Spec:   corev1.PodSpec{NodeName: fmt.Sprintf("node%d", (i+p)%4), Containers: []corev1.Container{{Name: "c"}}},
				Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "c"}}},
			})
		}
	}
	cs := fake.NewSimpleClientset(objs...)
	var mu sync.Mutex
	calls := map[string]int{}
	cs.PrependReactor("*", "*", func(a k8stesting.Action) (bool, runtime.Object, error) {
		key := a.GetVerb() + " " + a.GetResource().Resource
		if s := a.GetSubresource(); s != "" {
			key += "/" + s
		}
		mu.Lock()
		calls[key]++
		mu.Unlock()
		return false, nil, nil
	})
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		review := a.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		review.Status.Allowed = true
		return true, review, nil
	})
	counts := func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := make(map[string]int, len(calls))
		for k, v := range calls {
			out[k] = v
		}
		return out
	}
	return &Client{Clientset: cs, Config: &rest.Config{Host: srv.URL}, Context: "lab", Namespace: "big"}, counts
}

// A whole-namespace snapshot must not read Deployments, ReplicaSets or
// EndpointSlices once per object: in a namespace with hundreds of
// Deployments that is thousands of requests behind the client rate limit.
func TestCollectBigNamespaceListsOncePerType(t *testing.T) {
	t.Parallel()
	const n = 40
	client, counts := bigNamespaceClient(t, n)
	b, err := (&Collector{Client: client}).Collect(context.Background(), CollectOptions{Namespace: "big"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Workloads) != n || len(b.Services) != n || len(b.Pods) != 2*n || len(b.Nodes) != 4 {
		t.Fatalf("workloads=%d services=%d pods=%d nodes=%d", len(b.Workloads), len(b.Services), len(b.Pods), len(b.Nodes))
	}
	c := counts()
	for call, want := range map[string]int{
		"get deployments":     0,
		"list deployments":    1,
		"list replicasets":    1,
		"list endpointslices": 1,
		"get nodes":           4,
	} {
		if c[call] != want {
			t.Errorf("%s: %d calls, want %d (all calls: %v)", call, c[call], want, c)
		}
	}
	for _, s := range b.Services {
		if s.ReadyEndpoints != 1 || s.TotalEndpoints != 1 {
			t.Fatalf("service %s endpoints = %d/%d, want 1/1", s.Name, s.ReadyEndpoints, s.TotalEndpoints)
		}
	}
	if len(b.WorkloadChanges) != 1 || b.WorkloadChanges[0].Name != "app0" || b.WorkloadChanges[0].Revision != 2 {
		t.Fatalf("change markers = %+v, want one for app0 revision 2", b.WorkloadChanges)
	}
}

// A query naming one Deployment reads just that one, not the namespace.
func TestCollectOneDeploymentGetsIt(t *testing.T) {
	t.Parallel()
	client, counts := bigNamespaceClient(t, 40)
	b, err := (&Collector{Client: client}).Collect(context.Background(), CollectOptions{Namespace: "big", Query: "deployment/app0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Workloads) != 1 || b.Workloads[0].Name != "app0" {
		t.Fatalf("workloads = %+v", b.Workloads)
	}
	if c := counts(); c["list deployments"] != 0 || c["get deployments"] != 1 {
		t.Fatalf("deployment calls: get %d, list %d; want 1 get", c["get deployments"], c["list deployments"])
	}
	if len(b.WorkloadChanges) != 1 {
		t.Fatalf("change markers = %+v", b.WorkloadChanges)
	}
}
