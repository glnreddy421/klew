package kube

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// scopeClient is a namespace with a Deployment "api" and things it does not own.
// Discovery uses a dynamic client, so a test server answers its Deployment list.
func scopeClient(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/apis/apps/v1/namespaces/shop/deployments" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","code":404}`)
			return
		}
		fmt.Fprint(w, `{"kind":"DeploymentList","apiVersion":"apps/v1","items":[{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api","namespace":"shop"}}]}`)
	}))
	t.Cleanup(srv.Close)

	labelled := map[string]string{"app": "api"}
	pod := func(name, ownerKind, owner string, lbl map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "shop", Labels: lbl,
			OwnerReferences: []metav1.OwnerReference{{Kind: ownerKind, Name: owner}},
		}}
	}
	cs := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "shop"}},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "api-7c9", Namespace: "shop", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "api"}}},
			Spec:       appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: labelled}},
		},
		pod("api-7c9-x", "ReplicaSet", "api-7c9", labelled),
		pod("agent-node1", "DaemonSet", "agent", map[string]string{"app": "agent"}),
		pod("db-0", "StatefulSet", "db", map[string]string{"app": "db"}),
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "shop"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "db"}}},
		&autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "shop"},
			Spec:       autoscalingv2.HorizontalPodAutoscalerSpec{ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "StatefulSet", Name: "db"}},
		},
	)
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		review := a.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		review.Status.Allowed = true
		return true, review, nil
	})
	return &Client{Clientset: cs, Config: &rest.Config{Host: srv.URL}, Context: "lab", Namespace: "shop"}
}

func podNames(pods []model.PodSummary) []string {
	out := make([]string, 0, len(pods))
	for _, p := range pods {
		out = append(out, p.Name)
	}
	slices.Sort(out)
	return out
}

func TestCollectWholeNamespaceKeepsObjectsNoDeploymentOwns(t *testing.T) {
	t.Parallel()
	col := &Collector{Client: scopeClient(t)}
	b, err := col.Collect(context.Background(), CollectOptions{Namespace: "shop"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"agent-node1", "api-7c9-x", "db-0"}
	if got := podNames(b.Pods); !slices.Equal(got, want) {
		t.Fatalf("pods = %v, want %v", got, want)
	}
	if len(b.Services) != 1 || len(b.HPAs) != 1 {
		t.Fatalf("services = %d, hpas = %d; want the StatefulSet's of each", len(b.Services), len(b.HPAs))
	}
	if len(b.Workloads) != 1 || b.Workloads[0].Name != "api" {
		t.Fatalf("workloads = %+v; Deployments must still be collected", b.Workloads)
	}
}

func TestCollectWithQueryStaysNarrow(t *testing.T) {
	t.Parallel()
	col := &Collector{Client: scopeClient(t)}
	b, err := col.Collect(context.Background(), CollectOptions{Namespace: "shop", Query: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if got := podNames(b.Pods); !slices.Equal(got, []string{"api-7c9-x"}) {
		t.Fatalf("pods = %v, want only the api pod", got)
	}
	if len(b.Services) != 0 || len(b.HPAs) != 0 {
		t.Fatalf("services = %d, hpas = %d; want none for api", len(b.Services), len(b.HPAs))
	}
}
