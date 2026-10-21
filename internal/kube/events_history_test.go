package kube

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/glnreddy421/klew/internal/model"
)

type fakeHistory struct {
	recs     []model.EventRecord
	gotCtx   string
	gotHost  string
	gotNS    string
	gotSince time.Time
	gotLimit int
}

func (f *fakeHistory) RecordedEvents(_ context.Context, contextName, serverURL string, q model.EventHistoryQuery) ([]model.EventRecord, model.EventHistoryCoverage, error) {
	f.gotCtx, f.gotHost, f.gotNS, f.gotSince, f.gotLimit = contextName, serverURL, q.Namespace, q.Since, q.Limit
	return f.recs, model.EventHistoryCoverage{
		Since: model.TimestampFrom(time.Now().Add(-5 * time.Hour)),
		Gaps:  []model.TimeRange{{From: "2026-10-20T10:00:00Z", To: "2026-10-20T11:00:00Z"}},
	}, nil
}

func TestCollectEventsMergesRecordedByUID(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	live := metav1.NewTime(now.Add(-5 * time.Minute))
	cs := fake.NewSimpleClientset(
		&corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: "e-live", Namespace: "klew-lab", UID: types.UID("uid-live")},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "payment-api-1", Namespace: "klew-lab"},
			Type:           "Warning", Reason: "BackOff", Message: "back-off", Count: 2,
			FirstTimestamp: live, LastTimestamp: live,
		},
		// Same object through events.k8s.io/v1: must not duplicate.
		&eventsv1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: "e-live", Namespace: "klew-lab", UID: types.UID("uid-live")},
			Regarding:  corev1.ObjectReference{Kind: "Pod", Name: "payment-api-1", Namespace: "klew-lab"},
			Type:       "Warning", Reason: "BackOff", Note: "back-off (v1 wording)",
			EventTime:  metav1.NewMicroTime(live.Time),
		},
	)
	hist := &fakeHistory{recs: []model.EventRecord{
		{ // Same event, recorded earlier: earlier FirstSeen and higher count survive.
			UID: "uid-live", Type: "Warning", Reason: "BackOff", Message: "back-off", Count: 9,
			Timestamp: model.TimestampFrom(live.Time), FirstSeen: model.TimestampFrom(now.Add(-3 * time.Hour)),
			InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-api-1", Namespace: "klew-lab"}, Recorded: true,
		},
		{ // Expired from the API long ago (older than the 1h TTL).
			UID: "uid-old", Type: "Warning", Reason: "Failed", Message: "ErrImagePull", Count: 4,
			Timestamp: model.TimestampFrom(now.Add(-2 * time.Hour)), FirstSeen: model.TimestampFrom(now.Add(-150 * time.Minute)),
			InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-api-0", Namespace: "klew-lab"}, Recorded: true,
		},
		{ // Out of investigation scope.
			UID: "uid-other", Type: "Warning", Reason: "Failed",
			Timestamp:      model.TimestampFrom(now.Add(-2 * time.Hour)),
			InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "billing-7", Namespace: "klew-lab"}, Recorded: true,
		},
	}}
	col := &Collector{Client: &Client{
		Clientset: cs, Config: &rest.Config{Host: "https://127.0.0.1:6443"},
		Context: "docker-desktop", History: hist,
	}}

	got, cov := col.collectEvents(context.Background(), "klew-lab", "payment-api", nil, []string{"payment-api-1"}, true)

	if hist.gotCtx != "docker-desktop" || hist.gotHost != "https://127.0.0.1:6443" || hist.gotNS != "klew-lab" {
		t.Fatalf("history query = %q %q %q", hist.gotCtx, hist.gotHost, hist.gotNS)
	}
	if since := time.Since(hist.gotSince); since < RecordedEventsLookback-time.Minute || since > RecordedEventsLookback+time.Minute {
		t.Fatalf("lookback = %v", since)
	}
	if hist.gotLimit < 5000 {
		t.Fatalf("history limit %d is too small for scope filtering in Go", hist.gotLimit)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d: %+v", len(got), got)
	}
	// Sorted oldest first: the recorded expired event leads.
	old, cur := got[0], got[1]
	if old.UID != "uid-old" || !old.Recorded || old.Count != 4 {
		t.Fatalf("old = %+v", old)
	}
	if cur.UID != "uid-live" || cur.Recorded {
		t.Fatalf("live = %+v", cur)
	}
	if cur.Count != 9 || cur.FirstSeen != model.TimestampFrom(now.Add(-3*time.Hour)) {
		t.Fatalf("live row should keep history's count/firstSeen: %+v", cur)
	}
	if cov == nil || cov.RecordedEvents != 1 || len(cov.Gaps) != 1 {
		t.Fatalf("coverage = %+v", cov)
	}
}

func TestCollectEventsWithoutHistoryUnchanged(t *testing.T) {
	cs := fake.NewSimpleClientset()
	col := &Collector{Client: &Client{Clientset: cs, Config: &rest.Config{Host: "h"}}}
	got, cov := col.collectEvents(context.Background(), "ns", "", nil, nil, true)
	if len(got) != 0 || cov != nil {
		t.Fatalf("got %v %v", got, cov)
	}
}

func TestMergeRecordedEventsSkipsDuplicates(t *testing.T) {
	live := []model.EventRecord{{UID: "a", Reason: "A", Count: 1}}
	rec := []model.EventRecord{
		{UID: "a", Reason: "A", Count: 1, Recorded: true},
		{UID: "b", Reason: "B", Recorded: true},
		{UID: "b", Reason: "B", Recorded: true},
		{UID: "", Reason: "C", Recorded: true},
	}
	out, added := MergeRecordedEvents(live, rec)
	if len(out) != 2 || added != 1 || out[0].Recorded {
		t.Fatalf("out=%+v added=%d", out, added)
	}
}

// Separate Event objects for a recurring problem collapse into one row, as before.
func TestDedupeCollapsesSameContentAcrossUIDs(t *testing.T) {
	obj := model.ObjectRef{Kind: "Pod", Name: "p"}
	in := []model.EventRecord{
		{UID: "new", InvolvedObject: obj, Reason: "BackOff", Message: "back-off", Count: 2, FirstSeen: "2026-10-20T12:00:00Z"},
		{UID: "old", InvolvedObject: obj, Reason: "BackOff", Message: "back-off", Count: 9, FirstSeen: "2026-10-20T09:00:00Z", Recorded: true},
		{UID: "other", InvolvedObject: obj, Reason: "Failed", Message: "x"},
	}
	out := dedupeEventRecords(in)
	if len(out) != 2 {
		t.Fatalf("want 2 rows, got %+v", out)
	}
	if out[0].UID != "new" || out[0].Recorded || out[0].Count != 9 || out[0].FirstSeen != "2026-10-20T09:00:00Z" {
		t.Fatalf("collapsed row = %+v", out[0])
	}
}

func TestCollectEventsNoRecordedWithoutListPermission(t *testing.T) {
	hist := &fakeHistory{recs: []model.EventRecord{{UID: "u", Reason: "Failed", Recorded: true,
		InvolvedObject: model.ObjectRef{Kind: "Node", Name: "n1"}}}}
	col := &Collector{Client: &Client{Clientset: fake.NewSimpleClientset(), Config: &rest.Config{Host: "h"}, History: hist}}
	got, cov := col.collectEvents(context.Background(), "ns", "", nil, nil, false)
	if len(got) != 0 || cov != nil {
		t.Fatalf("recorded events leaked without list permission: %+v", got)
	}
}

func TestLiveWatcherSkipsSeededUIDs(t *testing.T) {
	var got []model.EvidenceEvent
	w := &LiveWatcher{Sink: func(e model.EvidenceEvent) { got = append(got, e) }}
	w.SeedEventUIDs([]string{"seen"})
	ev := func(uid, msg string) *corev1.Event {
		return &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{UID: types.UID(uid)},
			InvolvedObject: corev1.ObjectReference{Kind: "Node", Name: "n1"},
			Reason:         "NodeNotReady", Message: msg,
		}
	}
	w.emitCoreEvent(ev("seen", "a"), nil)
	w.emitCoreEvent(ev("new", "b"), nil)
	w.emitCoreEvent(ev("new", "b-updated"), nil)
	if len(got) != 1 || got[0].Message != "b" {
		t.Fatalf("emitted %+v", got)
	}
}
