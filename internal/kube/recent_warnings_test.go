package kube

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func warningEvent(name, typ, reason, pod, msg string, last time.Time, count int32) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: name, Namespace: "shop", UID: types.UID("uid-" + name)},
		Type:           typ,
		Reason:         reason,
		Message:        msg,
		Count:          count,
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: pod, Namespace: "shop"},
		FirstTimestamp: metav1.NewTime(last.Add(-time.Minute)),
		LastTimestamp:  metav1.NewTime(last),
	}
}

func TestListRecentWarningsNewestFirstAndCollapsed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	cs := fake.NewSimpleClientset(
		warningEvent("a1", "Warning", "BackOff", "api", "old backoff", now.Add(-10*time.Minute), 3),
		warningEvent("a2", "Warning", "BackOff", "api", "new backoff", now.Add(-time.Minute), 2),
		warningEvent("b", "Warning", "FailedScheduling", "worker", "0/2 nodes are available", now.Add(-5*time.Minute), 1),
		warningEvent("n", "Normal", "Pulled", "api", "pulled image", now, 1),
	)
	got, err := ListRecentWarnings(context.Background(), &Client{Clientset: cs}, []string{"shop"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessState != model.ResourceAccessAllowed {
		t.Fatalf("access = %q", got.AccessState)
	}
	if got.Total != 2 || len(got.Events) != 2 {
		t.Fatalf("events = %+v", got.Events)
	}
	first := got.Events[0]
	if first.Reason != "BackOff" || first.Count != 5 || first.Message != "new backoff" {
		t.Fatalf("collapsed row = %+v", first)
	}
	if got.Events[1].Reason != "FailedScheduling" {
		t.Fatalf("second row = %+v", got.Events[1])
	}
}

func TestListRecentWarningsRedactsAndShortens(t *testing.T) {
	now := time.Now().UTC()
	long := "failed: password=hunter2 " + strings.Repeat("x", 400)
	cs := fake.NewSimpleClientset(warningEvent("a", "Warning", "Failed", "api", long, now, 1))
	got, err := ListRecentWarnings(context.Background(), &Client{Clientset: cs}, []string{"shop"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	msg := got.Events[0].Message
	if strings.Contains(msg, "hunter2") {
		t.Fatalf("message not redacted: %q", msg)
	}
	if n := len([]rune(msg)); n > recentWarningMessageMax {
		t.Fatalf("message length = %d", n)
	}
}

func TestListRecentWarningsCapsRows(t *testing.T) {
	now := time.Now().UTC()
	var objs []runtime.Object
	for i := 0; i < 5; i++ {
		name := string(rune('a' + i))
		objs = append(objs, warningEvent(name, "Warning", "BackOff", "pod-"+name, "x", now.Add(-time.Duration(i)*time.Minute), 1))
	}
	got, err := ListRecentWarnings(context.Background(), &Client{Clientset: fake.NewSimpleClientset(objs...)}, []string{"shop"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 5 || len(got.Events) != 3 || got.Events[0].InvolvedObject.Name != "pod-a" {
		t.Fatalf("got total=%d events=%+v", got.Total, got.Events)
	}
}

func TestListRecentWarningsForbidden(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "events", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "events"}, "", errors.New("denied"))
	})
	got, err := ListRecentWarnings(context.Background(), &Client{Clientset: cs}, []string{"shop"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessState != model.ResourceAccessForbidden || len(got.Events) != 0 {
		t.Fatalf("got %+v", got)
	}
}
