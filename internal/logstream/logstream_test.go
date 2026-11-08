package logstream

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

func pod(name string, lbls map[string]string, containers ...string) *corev1.Pod {
	if len(containers) == 0 {
		containers = []string{"app"}
	}
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "shop", Labels: lbls},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	for _, c := range containers {
		p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: c})
	}
	return p
}

func deployment(name string, sel map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "shop"},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: sel}},
	}
}

// recorder collects batches from a session.
type recorder struct {
	mu    sync.Mutex
	lines []Line
}

func (r *recorder) onBatch(b Batch) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, b.Lines...)
}

func (r *recorder) sources() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]bool{}
	for _, l := range r.lines {
		out[l.Pod+"/"+l.Container] = true
	}
	return out
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func newSession(objs ...runtime.Object) (*Session, *fake.Clientset, *recorder) {
	cs := fake.NewSimpleClientset(objs...)
	rec := &recorder{}
	return &Session{Client: &kube.Client{Clientset: cs}, OnBatch: rec.onBatch}, cs, rec
}

func TestWorkloadFollowsSelectorIncludingNewPods(t *testing.T) {
	s, cs, rec := newSession(
		deployment("web", map[string]string{"app": "web"}),
		pod("web-1", map[string]string{"app": "web"}),
		pod("db-1", map[string]string{"app": "db"}),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx, Spec{Targets: []Target{{Namespace: "shop", Kind: "Deployment", Name: "web"}}}); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	st := s.Status()
	if !st.Active || len(st.Sources) != 1 || st.Sources[0].Label != "Deployment/web" || !st.Sources[0].Follows {
		t.Fatalf("status = %+v", st)
	}
	waitFor(t, "web-1 logs", func() bool { return rec.sources()["web-1/app"] })

	if _, err := cs.CoreV1().Pods("shop").Create(ctx, pod("web-2", map[string]string{"app": "web"}), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "web-2 logs", func() bool { return rec.sources()["web-2/app"] })

	if rec.sources()["db-1/app"] {
		t.Fatal("followed a pod outside the selector")
	}
}

func TestNamedPodsAndContainerFilter(t *testing.T) {
	s, _, rec := newSession(
		pod("a", nil, "app", "sidecar"),
		pod("b", nil, "app"),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := s.Start(ctx, Spec{
		Targets:   []Target{{Namespace: "shop", Pods: []string{"a", "a"}}},
		Container: "app",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	if got := s.Status().Sources[0].Label; got != "Pod/a" {
		t.Fatalf("label = %q", got)
	}
	waitFor(t, "a/app logs", func() bool { return rec.sources()["a/app"] })
	time.Sleep(300 * time.Millisecond)
	src := rec.sources()
	if src["a/sidecar"] || src["b/app"] {
		t.Fatalf("followed unselected sources: %v", src)
	}
}

func TestStartRejectsBadTargets(t *testing.T) {
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "headless", Namespace: "shop"}}
	empty := deployment("all", nil)
	s, _, _ := newSession(svc, empty)
	many := make([]Target, MaxTargets+1)
	for i := range many {
		many[i] = Target{Namespace: "shop", Pods: []string{"p"}}
	}
	cases := map[string]Spec{
		"no targets":         {},
		"no namespace":       {Targets: []Target{{Pods: []string{"a"}}}},
		"nothing named":      {Targets: []Target{{Namespace: "shop"}}},
		"unsupported kind":   {Targets: []Target{{Namespace: "shop", Kind: "ConfigMap", Name: "x"}}},
		"missing workload":   {Targets: []Target{{Namespace: "shop", Kind: "Deployment", Name: "nope"}}},
		"empty selector":     {Targets: []Target{{Namespace: "shop", Kind: "Deployment", Name: "all"}}},
		"selectorless svc":   {Targets: []Target{{Namespace: "shop", Kind: "Service", Name: "headless"}}},
		"bad label selector": {Targets: []Target{{Namespace: "shop", LabelSelector: "app in ("}}},
		"too many targets":   {Targets: many},
	}
	for name, spec := range cases {
		if err := s.Start(context.Background(), spec); err == nil {
			s.Stop()
			t.Errorf("%s: want error", name)
		}
	}
	if s.Status().Active {
		t.Fatal("session active after rejected starts")
	}
}

func TestBufferKeepsNewestLines(t *testing.T) {
	s := &Session{BufferLines: 3}
	for i := 0; i < 5; i++ {
		s.sink(model.EvidenceEvent{SourceType: model.SourceLog, Pod: "p", Raw: string(rune('a' + i))})
	}
	var got []string
	for _, l := range s.Lines() {
		got = append(got, l.Text)
	}
	if strings.Join(got, "") != "cde" {
		t.Fatalf("lines = %v", got)
	}
}

func TestSystemEventsBecomeNotices(t *testing.T) {
	var statuses []Status
	s := &Session{OnStatus: func(st Status) { statuses = append(statuses, st) }}
	s.sink(model.EvidenceEvent{SourceType: model.SourceSystem, Message: "Reached concurrent log follow limit"})
	s.flush()
	if len(s.Lines()) != 0 {
		t.Fatal("system event buffered as a log line")
	}
	if len(statuses) != 1 || len(statuses[0].Notices) != 1 {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestMergeIntoAddsSourcesKeepsLines(t *testing.T) {
	s, _, rec := newSession(
		pod("a", nil),
		pod("b", nil),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx, Spec{Targets: []Target{{Namespace: "shop", Pods: []string{"a"}}}}); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	waitFor(t, "pod a", func() bool { return rec.sources()["a/app"] })

	nBefore := len(s.Lines())
	if err := s.MergeInto(ctx, Spec{Targets: []Target{{Namespace: "shop", Pods: []string{"b"}}}}); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if len(st.Sources) != 2 {
		t.Fatalf("sources = %+v", st.Sources)
	}
	waitFor(t, "pod b", func() bool { return rec.sources()["b/app"] })
	if len(s.Lines()) < nBefore {
		t.Fatalf("buffer shrank: before %d after %d", nBefore, len(s.Lines()))
	}
}

func TestPauseResume(t *testing.T) {
	s, _, rec := newSession(pod("a", nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Pause(); err == nil {
		t.Fatal("pause without a stream: want error")
	}
	if err := s.Start(ctx, Spec{Targets: []Target{{Namespace: "shop", Pods: []string{"a"}}}}); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	waitFor(t, "first line", func() bool { return len(rec.sources()) > 0 })

	if err := s.Pause(); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); !st.Active || !st.Paused {
		t.Fatalf("after pause: %+v", st)
	}
	buffered := len(s.Lines())
	if err := s.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.Paused {
		t.Fatalf("after resume: %+v", st)
	}
	waitFor(t, "lines after resume", func() bool { return len(s.Lines()) > buffered })

	s.Stop()
	if st := s.Status(); st.Active || len(s.Lines()) != 0 {
		t.Fatalf("after stop: %+v, %d lines", st, len(s.Lines()))
	}
}

func TestSessionIDCarriedOnStatusAndBatch(t *testing.T) {
	cs := fake.NewSimpleClientset(pod("a", nil))
	var (
		mu       sync.Mutex
		batchIDs []string
		statIDs  []string
	)
	s := &Session{
		ID:     "log-7",
		Client: &kube.Client{Clientset: cs},
		OnBatch: func(b Batch) {
			mu.Lock()
			batchIDs = append(batchIDs, b.ID)
			mu.Unlock()
		},
		OnStatus: func(st Status) {
			mu.Lock()
			statIDs = append(statIDs, st.ID)
			mu.Unlock()
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx, Spec{Targets: []Target{{Namespace: "shop", Pods: []string{"a"}}}}); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	if got := s.Status().ID; got != "log-7" {
		t.Fatalf("Status().ID = %q, want log-7", got)
	}
	waitFor(t, "batch with ID", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(batchIDs) > 0 && batchIDs[0] == "log-7"
	})
	mu.Lock()
	defer mu.Unlock()
	if len(statIDs) == 0 || statIDs[0] != "log-7" {
		t.Fatalf("statIDs = %v", statIDs)
	}
}
