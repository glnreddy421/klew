package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
)

func TestLogStreamerMaxRequestsDefault(t *testing.T) {
	ls := &LogStreamer{}
	if ls.maxRequests() != DefaultMaxLogRequests {
		t.Fatalf("got %d want %d", ls.maxRequests(), DefaultMaxLogRequests)
	}
	ls.MaxLogRequests = 10
	if ls.maxRequests() != 10 {
		t.Fatalf("got %d want 10", ls.maxRequests())
	}
}

func TestAppContainerNamesSkipsInit(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "ns"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init"}},
			Containers:     []corev1.Container{{Name: "app"}, {Name: "sidecar"}},
		},
	}
	names := appContainerNames(pod)
	if len(names) != 2 || names[0] != "app" || names[1] != "sidecar" {
		t.Fatalf("unexpected names: %#v", names)
	}
}

func TestAppContainerNamesFallsBackToStatus(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{Name: "only"}},
		},
	}
	names := appContainerNames(pod)
	if len(names) != 1 || names[0] != "only" {
		t.Fatalf("unexpected names: %#v", names)
	}
}

func TestStripANSI(t *testing.T) {
	raw := "\x1b[90m2026-07-17T02:03:53Z\x1b[0m \x1b[34mTRC\x1b[0m \x1b[1mreflector.go:446\x1b[0m \x1b[36m > \x1b[0m Caches populated"
	got := stripANSI(raw)
	want := "2026-07-17T02:03:53Z TRC reflector.go:446  >  Caches populated"
	if got != want {
		t.Fatalf("stripANSI=%q want %q", got, want)
	}
	if stripANSI("plain log line") != "plain log line" {
		t.Fatal("plain line should be unchanged")
	}
}

func TestClassifyLogLine(t *testing.T) {
	if classifyLogLine("oom killer") != model.SeverityCritical {
		t.Fatal("expected critical")
	}
	if classifyLogLine("connection refused") != model.SeverityHigh {
		t.Fatal("expected high")
	}
	if classifyLogLine("timeout waiting") != model.SeverityWarning {
		t.Fatal("expected warning")
	}
	if classifyLogLine("hello") != model.SeverityInfo {
		t.Fatal("expected info")
	}
}

func TestLogTargetKey(t *testing.T) {
	tkey := logTarget{ns: "ns", pod: "p", container: "c"}.key()
	if tkey != "ns/p/c" {
		t.Fatalf("got %q", tkey)
	}
}
