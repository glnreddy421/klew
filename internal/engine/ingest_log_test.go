package engine

import (
	"testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func TestIngestLogAcceptsOnlyInvestigatedPods(t *testing.T) {
	st := model.InvestigationState{}
	st.Snapshot.Pods = []model.PodSummary{{Name: "api-1", Namespace: "shop"}}
	s := &LiveSession{Reducer: NewReducer(&st), Bus: NewBus(8), logNS: "shop"}

	line := func(ns, pod string) model.EvidenceEvent {
		return model.EvidenceEvent{SourceType: model.SourceLog, Namespace: ns, Pod: pod, Raw: "x"}
	}
	if !s.IngestLog(line("shop", "api-1")) {
		t.Fatal("rejected a pod in the snapshot")
	}
	for name, e := range map[string]model.EvidenceEvent{
		"other pod":       line("shop", "db-1"),
		"other namespace": line("prod", "api-1"),
		"not a log":       {SourceType: model.SourceSystem, Namespace: "shop", Pod: "api-1"},
	} {
		if s.IngestLog(e) {
			t.Errorf("%s: accepted", name)
		}
	}
	if got := len(s.Bus.Events()); got != 1 {
		t.Fatalf("bus has %d events, want 1", got)
	}

	s.noLogs = true
	if s.IngestLog(line("shop", "api-1")) {
		t.Fatal("accepted a line for an investigation without logs")
	}
	var nilSession *LiveSession
	if nilSession.IngestLog(line("shop", "api-1")) {
		t.Fatal("nil session accepted a line")
	}
}
