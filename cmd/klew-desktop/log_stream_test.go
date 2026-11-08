package main

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/logstream"
)

func testPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "shop"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func newTestLogApp(objs ...runtime.Object) *App {
	cs := fake.NewSimpleClientset(objs...)
	return &App{
		logStreamClient: &kube.Client{Clientset: cs, Namespace: "shop"},
	}
}

func TestAppMultipleLogStreamsLifecycle(t *testing.T) {
	app := newTestLogApp(testPod("api-1"), testPod("web-1"), testPod("db-1"))
	defer app.stopAllLogStreams()

	st1, err := app.StartLogStream(LogStreamOptions{
		Targets: []logstream.Target{{Namespace: "shop", Pods: []string{"api-1"}}},
	})
	if err != nil {
		t.Fatalf("start stream 1: %v", err)
	}
	st2, err := app.StartLogStream(LogStreamOptions{
		Targets: []logstream.Target{{Namespace: "shop", Pods: []string{"web-1"}}},
	})
	if err != nil {
		t.Fatalf("start stream 2: %v", err)
	}
	if st1.ID == "" || st2.ID == "" || st1.ID == st2.ID {
		t.Fatalf("expected distinct stream IDs, got %q and %q", st1.ID, st2.ID)
	}

	listed := app.ListLogStreams()
	if len(listed) != 2 || listed[0].ID != st1.ID || listed[1].ID != st2.ID {
		t.Fatalf("ListLogStreams = %+v", listed)
	}

	if err := app.PauseLogStream(st1.ID); err != nil {
		t.Fatalf("pause stream 1: %v", err)
	}
	if !app.LogStreamStatus(st1.ID).Paused {
		t.Fatal("stream 1 should be paused")
	}
	if app.LogStreamStatus(st2.ID).Paused {
		t.Fatal("stream 2 should remain unpaused")
	}
	if err := app.ResumeLogStream(st1.ID); err != nil {
		t.Fatalf("resume stream 1: %v", err)
	}
	if app.LogStreamStatus(st1.ID).Paused {
		t.Fatal("stream 1 should be resumed")
	}

	// Merge into stream 2 only.
	merged, err := app.StartLogStream(LogStreamOptions{
		StreamID:        st2.ID,
		MergeIntoActive: true,
		Targets:         []logstream.Target{{Namespace: "shop", Pods: []string{"db-1"}}},
	})
	if err != nil {
		t.Fatalf("merge into stream 2: %v", err)
	}
	if merged.ID != st2.ID || len(merged.Sources) != 2 {
		t.Fatalf("merged status = %+v", merged)
	}
	if len(app.LogStreamStatus(st1.ID).Sources) != 1 {
		t.Fatalf("stream 1 sources changed unexpectedly: %+v", app.LogStreamStatus(st1.ID).Sources)
	}

	app.StopLogStream(st1.ID)
	if app.LogStreamStatus(st1.ID).Active {
		t.Fatal("stream 1 still active after StopLogStream")
	}
	if !app.LogStreamStatus(st2.ID).Active {
		t.Fatal("stream 2 should remain active after stopping stream 1")
	}
	if got := app.ListLogStreams(); len(got) != 1 || got[0].ID != st2.ID {
		t.Fatalf("ListLogStreams after stop = %+v", got)
	}
}

func TestAppEnforcesMaxLogStreamsCap(t *testing.T) {
	var objs []runtime.Object
	for i := 0; i < MaxLogStreams+1; i++ {
		objs = append(objs, testPod(fmt.Sprintf("p-%d", i)))
	}
	app := newTestLogApp(objs...)
	defer app.stopAllLogStreams()

	for i := 0; i < MaxLogStreams; i++ {
		if _, err := app.StartLogStream(LogStreamOptions{
			Targets: []logstream.Target{{Namespace: "shop", Pods: []string{fmt.Sprintf("p-%d", i)}}},
		}); err != nil {
			t.Fatalf("stream %d failed: %v", i, err)
		}
	}

	_, err := app.StartLogStream(LogStreamOptions{
		Targets: []logstream.Target{{Namespace: "shop", Pods: []string{fmt.Sprintf("p-%d", MaxLogStreams)}}},
	})
	if err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("expected max log streams error, got %v", err)
	}
}
