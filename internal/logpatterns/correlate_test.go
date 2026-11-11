package logpatterns

import (
	"testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func TestJaccardActiveSlots(t *testing.T) {
	a := []int64{0, 1, 1, 0, 1}
	b := []int64{0, 1, 0, 1, 1}
	// active A: {1,2,4} B: {1,3,4} → ∩={1,4} ∪={1,2,3,4} → 2/4 = 0.5
	got := jaccardActiveSlots(a, b)
	if got != 0.5 {
		t.Fatalf("jaccard=%v want 0.5", got)
	}
	if jaccardActiveSlots(nil, a) != 0 {
		t.Fatal("empty should be 0")
	}
	if jaccardActiveSlots([]int64{0, 0}, []int64{0, 0}) != 0 {
		t.Fatal("all-zero union should be 0")
	}
	identical := []int64{1, 0, 1, 1}
	if jaccardActiveSlots(identical, identical) != 1 {
		t.Fatal("identical active slots should be 1")
	}
}

func TestBuildEvidenceBoard_ThresholdAndSort(t *testing.T) {
	// 15-minute windows; correlate on overlapping active minutes.
	mkVol := func(active ...int) []int64 {
		v := make([]int64, 15)
		for _, i := range active {
			v[i] = 1
		}
		return v
	}
	// Strong overlap (12 shared of 12 union-ish) → high Jaccard
	strongLog := mkVol(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
	strongEv := mkVol(0, 1, 2, 3, 4, 5, 6, 7, 8, 9) // Jaccard = 1.0
	// Weak overlap
	weakLog := mkVol(0, 1)
	weakEv := mkVol(10, 11, 12, 13, 14) // Jaccard = 0
	// Medium: 3/5 = 0.6 exactly
	medLog := mkVol(0, 1, 2)
	medEv := mkVol(0, 1, 2, 3, 4) // ∩=3 ∪=5 → 0.6

	logs := []model.LogTemplate{
		{ID: "tpl_log_strong", Template: "connection refused", VolumeHistory: strongLog, Count: 10},
		{ID: "tpl_log_weak", Template: "health ok", VolumeHistory: weakLog, Count: 2},
		{ID: "tpl_log_med", Template: "timeout waiting", VolumeHistory: medLog, Count: 5},
	}
	events := []model.LogTemplate{
		{ID: "tpl_ev_strong", Template: "[Failed] image pull", VolumeHistory: strongEv, Count: 4},
		{ID: "tpl_ev_weak", Template: "[Scheduled] assigned", VolumeHistory: weakEv, Count: 1},
		// Non-failure reason so only Jaccard applies (containment is for BackOff/OOM/…).
		{ID: "tpl_ev_med", Template: "[Probe] timeout waiting for endpoints", VolumeHistory: medEv, Count: 3},
	}

	board := BuildEvidenceBoard(logs, events, DefaultJaccardThreshold, 15)
	if board == nil {
		t.Fatal("nil board")
	}
	if board.CardCount != 2 {
		t.Fatalf("cardCount=%d want 2 (strong+med); cards=%#v", board.CardCount, board.Cards)
	}
	// Highest confidence first
	if board.Cards[0].Confidence < board.Cards[1].Confidence {
		t.Fatalf("not sorted by confidence: %#v", board.Cards)
	}
	if board.Cards[0].Confidence != 1 {
		t.Fatalf("top confidence=%v want 1", board.Cards[0].Confidence)
	}
	if board.Cards[0].RootEvent.ID != "tpl_ev_strong" {
		t.Fatalf("root=%s", board.Cards[0].RootEvent.ID)
	}
	if len(board.Cards[0].TriggeredLogs) != 1 || board.Cards[0].TriggeredLogs[0].ID != "tpl_log_strong" {
		t.Fatalf("triggered=%#v", board.Cards[0].TriggeredLogs)
	}
	if board.Cards[1].Confidence < 0.59 || board.Cards[1].Confidence > 0.61 {
		t.Fatalf("med confidence=%v want ~0.6", board.Cards[1].Confidence)
	}
	if board.Cards[0].EvidenceID == "" || board.Cards[0].EvidenceID == board.Cards[1].EvidenceID {
		t.Fatalf("evidence IDs invalid: %s / %s", board.Cards[0].EvidenceID, board.Cards[1].EvidenceID)
	}

	// Mutation isolation: mutating source must not alter card
	logs[0].Template = "MUTATED"
	if board.Cards[0].TriggeredLogs[0].Template == "MUTATED" {
		t.Fatal("triggered log not isolated")
	}
}

func TestBuildEvidenceBoard_Empty(t *testing.T) {
	board := BuildEvidenceBoard(nil, nil, 0, 0)
	if board == nil || board.Threshold != DefaultJaccardThreshold || board.WindowMinutes != defaultSparklineMinutes {
		t.Fatalf("%#v", board)
	}
}

func TestBuildEvidenceBoard_NeedsSharedPod(t *testing.T) {
	vol := []int64{1, 1, 0, 1}
	ev := model.LogTemplate{ID: "e1", Template: "[BackOff] Back-off restarting failed container", VolumeHistory: vol, Pods: []string{"billing-1"}}
	logs := []model.LogTemplate{
		{ID: "l1", Template: "FATAL required_version file not found", VolumeHistory: vol, Pods: []string{"billing-1"}},
		{ID: "l2", Template: "writing to /exports/invoices.csv", VolumeHistory: vol, Pods: []string{"invoice-export-x"}},
		{ID: "l3", Template: "no pods recorded", VolumeHistory: vol},
	}
	b := BuildEvidenceBoard(logs, []model.LogTemplate{ev}, 0.6, 4)
	if len(b.Cards) != 1 {
		t.Fatalf("cards=%d want 1", len(b.Cards))
	}
	var got []string
	for _, l := range b.Cards[0].TriggeredLogs {
		got = append(got, l.ID)
	}
	if len(got) != 2 || got[0] == "l2" || got[1] == "l2" {
		t.Fatalf("linked %v, want l1 and l3 but not another workload's l2", got)
	}
}

func TestContainmentActiveSlots(t *testing.T) {
	// Dense event, sparse log in the last minute → containment 1, Jaccard 1/15.
	ev := make([]int64, 15)
	for i := range ev {
		ev[i] = 2
	}
	log := make([]int64, 15)
	log[14] = 1
	if g := containmentActiveSlots(ev, log); g != 1 {
		t.Fatalf("containment=%v want 1", g)
	}
	if g := jaccardActiveSlots(ev, log); g >= DefaultJaccardThreshold {
		t.Fatalf("jaccard=%v should stay below the co-movement bar", g)
	}
}

func TestBuildEvidenceBoard_ChronicBackOffCoversSparseFatal(t *testing.T) {
	mk := func(fillAll, lastOnly bool) []int64 {
		v := make([]int64, 15)
		if fillAll {
			for i := range v {
				v[i] = 2
			}
		}
		if lastOnly {
			v[14] = 1
		}
		return v
	}
	createdOnly := make([]int64, 15)
	createdOnly[0] = 1 // no overlap with the log's last minute
	logs := []model.LogTemplate{
		{ID: "l-fatal", Template: "FATAL required_version file not found", VolumeHistory: mk(false, true), Pods: []string{"billing"}},
		{ID: "l-info", Template: "INFO checking schema version", VolumeHistory: mk(false, true), Pods: []string{"billing"}},
	}
	events := []model.LogTemplate{
		{ID: "e-backoff", Template: "[BackOff] Back-off restarting failed container check-migrations", VolumeHistory: mk(true, false), Pods: []string{"billing"}, Count: 5260},
		{ID: "e-pulled", Template: "[Pulled] Container image busybox already present", VolumeHistory: mk(false, true), Pods: []string{"billing"}, Count: 1216},
		{ID: "e-created", Template: "[Created] Container created", VolumeHistory: createdOnly, Pods: []string{"billing"}, Count: 1},
	}
	board := BuildEvidenceBoard(logs, events, DefaultJaccardThreshold, 15)
	if board.CardCount < 1 {
		t.Fatal("expected at least the BackOff coverage card")
	}
	if board.Cards[0].RootEvent.ID != "e-backoff" {
		t.Fatalf("top card=%s want e-backoff (failure coverage + sort)", board.Cards[0].RootEvent.ID)
	}
	if board.Cards[0].Confidence < DefaultContainmentThreshold {
		t.Fatalf("BackOff confidence=%v", board.Cards[0].Confidence)
	}
	if len(board.Cards[0].TriggeredLogs) != 1 || board.Cards[0].TriggeredLogs[0].ID != "l-fatal" {
		t.Fatalf("BackOff triggered=%v want FATAL only (INFO is not a signal log)", board.Cards[0].TriggeredLogs)
	}
	for _, c := range board.Cards {
		if c.RootEvent.ID == "e-created" {
			t.Fatal("Created with no shared active minutes must not link")
		}
	}
}

func TestBuildEvidenceBoard_ContainmentIgnoresUnrelatedWorkload(t *testing.T) {
	vol := make([]int64, 15)
	for i := range vol {
		vol[i] = 1
	}
	logSparse := make([]int64, 15)
	logSparse[14] = 1
	ev := model.LogTemplate{ID: "e1", Template: "[BackOff] restarting", VolumeHistory: vol, Pods: []string{"billing"}}
	logs := []model.LogTemplate{
		{ID: "other", Template: "FATAL elsewhere", VolumeHistory: logSparse, Pods: []string{"invoice"}},
	}
	b := BuildEvidenceBoard(logs, []model.LogTemplate{ev}, DefaultJaccardThreshold, 15)
	if b.CardCount != 0 {
		t.Fatalf("cards=%d want 0 across workloads", b.CardCount)
	}
}

func TestBuildEvidenceBoard_CollapsedCrashLoopCountExpands(t *testing.T) {
	// API shape: one BackOff LastTimestamp, Count=5260 → a single hot minute.
	backoff := make([]int64, 15)
	backoff[10] = 5260
	fatal := make([]int64, 15)
	fatal[14] = 1 // collected a few minutes later; no shared minute
	board := BuildEvidenceBoard(
		[]model.LogTemplate{{ID: "l1", Template: "FATAL required_version file not found", VolumeHistory: fatal, Pods: []string{"billing"}}},
		[]model.LogTemplate{{ID: "e1", Template: "[BackOff] Back-off restarting failed container", VolumeHistory: backoff, Count: 5260, Pods: []string{"billing"}}},
		DefaultJaccardThreshold, 15,
	)
	if board.CardCount != 1 {
		t.Fatalf("cards=%d want 1 after expanding collapsed CrashLoop volume", board.CardCount)
	}
	if board.Cards[0].RootEvent.ID != "e1" || board.Cards[0].TriggeredLogs[0].ID != "l1" {
		t.Fatalf("card=%#v", board.Cards[0])
	}
}
