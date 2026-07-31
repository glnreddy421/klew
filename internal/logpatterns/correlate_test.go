package logpatterns

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
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
		{ID: "tpl_ev_med", Template: "[Unhealthy] probe failed", VolumeHistory: medEv, Count: 3},
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
