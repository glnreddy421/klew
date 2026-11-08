package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/model"
)

func TestBusIdleCountsDroppedAndInFlightEvents(t *testing.T) {
	b := NewBus(2)
	if !b.Idle() {
		t.Fatal("new bus should be idle")
	}
	for range 5 { // three are dropped
		b.Publish(model.EvidenceEvent{})
	}
	if b.Idle() {
		t.Fatal("queued events should not be idle")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	var applied atomic.Int32
	go b.RunConsumer(ctx, func(model.EvidenceEvent) {
		<-release
		applied.Add(1)
	})
	release <- struct{}{} // first event finishes; the second is held in the consumer
	deadline := time.Now().Add(time.Second)
	for applied.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if b.Idle() {
		t.Fatal("an event still being applied should not be idle")
	}
	close(release)
	for !b.Idle() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !b.Idle() {
		t.Fatalf("bus never went idle: queued=%d handled=%d", b.queued.Load(), b.handled.Load())
	}
}

// A zero duration must still return a verdict built from the snapshot's
// evidence, not the state from before the bus was drained.
func TestRunForZeroDurationAppliesSnapshotEvidence(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "e1", Namespace: "shop", UID: "e1"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "api-1", Namespace: "shop"},
		Type:           "Warning", Reason: "BackOff", Message: "back-off restarting failed container",
		LastTimestamp: metav1.NewTime(time.Now()),
	})
	// Real watches close when their context ends; the fake's never do, and
	// watchPods and watchDeployments depend on that to stop.
	endedWatch := func(k8stesting.Action) (bool, watch.Interface, error) { return true, watch.NewEmptyWatch(), nil }
	cs.PrependWatchReactor("pods", endedWatch)
	cs.PrependWatchReactor("deployments", endedWatch)
	client := &kube.Client{Clientset: cs, Context: "lab", Namespace: "shop"}
	st, err := RunFor(context.Background(), client, LiveOptions{Namespace: "shop"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if st.Counters.EventsIngested == 0 {
		t.Fatal("snapshot events were not applied")
	}
	if st.LogPatterns == nil {
		t.Fatal("log patterns were not computed")
	}
}
