package changes

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func hashed(t corev1.PodTemplateSpec, hash string) corev1.PodTemplateSpec {
	t.Labels = map[string]string{"app": "pay", podTemplateHashLabel: hash}
	return t
}

func TestCompareRevisionsDeployment(t *testing.T) {
	v1 := hashed(tpl("repo/pay:v1",
		corev1.EnvVar{Name: "LOG_LEVEL", Value: "info"},
		corev1.EnvVar{Name: "DB_PASSWORD", Value: "hunter2-old"},
		corev1.EnvVar{Name: "API_TOKEN", Value: "same-token"},
	), "aaa")
	v2 := hashed(tpl("repo/pay:v2",
		corev1.EnvVar{Name: "LOG_LEVEL", Value: "debug"},
		corev1.EnvVar{Name: "DB_PASSWORD", Value: "hunter2-new"},
		corev1.EnvVar{Name: "API_TOKEN", Value: "same-token"},
	), "bbb")
	v2.Spec.Containers[0].Args = []string{"--db-password=s3cret", "--port=8080"}
	cs := fake.NewSimpleClientset(deployment("dep-1"),
		rs("pay-aaa", "dep-1", 1, t0, v1, nil),
		rs("pay-bbb", "dep-1", 2, t0.Add(time.Hour), v2, nil),
	)
	cmp, err := CompareRevisions(context.Background(), cs, "Deployment", "klew-lab", "payment-api", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if cmp.From.Revision != 1 || cmp.To.Revision != 2 || cmp.From.ObjectName != "pay-aaa" || cmp.AccessState != model.ChangesAccessAllowed {
		t.Fatalf("compare = %+v", cmp)
	}
	for _, want := range []string{"image: repo/pay:v1", "value: info", "app: pay"} {
		if !strings.Contains(cmp.From.YAML, want) {
			t.Errorf("from YAML lacks %q:\n%s", want, cmp.From.YAML)
		}
	}
	for _, want := range []string{"image: repo/pay:v2", "value: debug", Redacted + " (changed)", "--db-password=" + Redacted, "--port=8080"} {
		if !strings.Contains(cmp.To.YAML, want) {
			t.Errorf("to YAML lacks %q:\n%s", want, cmp.To.YAML)
		}
	}
	both := cmp.From.YAML + cmp.To.YAML
	for _, leaked := range []string{"hunter2", "same-token", "s3cret", podTemplateHashLabel, "creationTimestamp"} {
		if strings.Contains(both, leaked) {
			t.Errorf("YAML contains %q:\n%s", leaked, both)
		}
	}
	// API_TOKEN did not change, so it must not be marked as changed.
	if strings.Count(cmp.To.YAML, "(changed)") != 1 {
		t.Errorf("want exactly one changed marker:\n%s", cmp.To.YAML)
	}
	if cmp.Redacted != 5 {
		t.Errorf("redacted = %d, want 5", cmp.Redacted)
	}
}

func TestCompareRevisionsStatefulSet(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "klew-lab", UID: "sts-1"},
		Spec:       appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "pay"}}},
	}
	cs := fake.NewSimpleClientset(sts,
		controllerRevision("redis-aaa", "sts-1", 1, t0, tpl("redis:7.2")),
		controllerRevision("redis-bbb", "sts-1", 2, t0.Add(time.Hour), tpl("redis:7.4")),
	)
	cmp, err := CompareRevisions(context.Background(), cs, "StatefulSet", "klew-lab", "redis", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmp.From.YAML, "image: redis:7.2") || !strings.Contains(cmp.To.YAML, "image: redis:7.4") {
		t.Fatalf("compare = %+v", cmp)
	}
}

func TestCompareRevisionsMissingRevision(t *testing.T) {
	cs := fake.NewSimpleClientset(deployment("dep-1"), rs("pay-aaa", "dep-1", 1, t0, tpl("repo/pay:v1"), nil))
	_, err := CompareRevisions(context.Background(), cs, "Deployment", "klew-lab", "payment-api", 1, 9)
	if err == nil || !strings.Contains(err.Error(), "revision 9") {
		t.Fatalf("err = %v", err)
	}
	if _, err := CompareRevisions(context.Background(), cs, "Pod", "klew-lab", "x", 1, 2); err == nil {
		t.Fatal("want an error for a kind without revisions")
	}
}

func TestCompareRevisionsDenied(t *testing.T) {
	cs := fake.NewSimpleClientset(deployment("dep-1"))
	cs.PrependReactor("list", "replicasets", func(a k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(appsv1.Resource("replicasets"), "", fmt.Errorf("denied"))
	})
	cmp, err := CompareRevisions(context.Background(), cs, "Deployment", "klew-lab", "payment-api", 1, 2)
	if err != nil {
		t.Fatalf("denied must not be an error: %v", err)
	}
	if cmp.AccessState != model.ChangesAccessDenied || !strings.Contains(cmp.AccessMessage, "replicasets list") {
		t.Fatalf("compare = %+v", cmp)
	}
}
