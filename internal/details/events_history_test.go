package details

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/rest"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

type stubHistory struct {
	q    model.EventHistoryQuery
	recs []model.EventRecord
	cov  model.EventHistoryCoverage
}

func (s *stubHistory) RecordedEvents(_ context.Context, _, _ string, q model.EventHistoryQuery) ([]model.EventRecord, model.EventHistoryCoverage, error) {
	s.q = q
	return s.recs, s.cov, nil
}

func TestEventsSectionIncludesRecordedHistory(t *testing.T) {
	now := time.Now().UTC()
	h := &stubHistory{
		recs: []model.EventRecord{
			{UID: "old", Type: "Warning", Reason: "Failed", Message: "ErrImagePull", Count: 4, Recorded: true,
				Timestamp: model.TimestampFrom(now.Add(-26 * time.Hour)), FirstSeen: model.TimestampFrom(now.Add(-27 * time.Hour)),
				InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-api-7d9-x"}},
			{UID: "dup", Type: "Warning", Reason: "BackOff", Message: "back-off", Recorded: true,
				Timestamp:      model.TimestampFrom(now.Add(-time.Minute)),
				InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-api-7d9-x"}},
			{UID: "other", Type: "Warning", Reason: "Failed", Recorded: true,
				Timestamp:      model.TimestampFrom(now.Add(-time.Minute)),
				InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "billing-1"}},
		},
		cov: model.EventHistoryCoverage{
			Since: model.TimestampFrom(now.Add(-72 * time.Hour)),
			Gaps: []model.TimeRange{
				{From: model.TimestampFrom(now.Add(-60 * time.Hour)), To: model.TimestampFrom(now.Add(-50 * time.Hour))}, // before the object existed
				{From: model.TimestampFrom(now.Add(-10 * time.Hour)), To: model.TimestampFrom(now.Add(-8 * time.Hour))},
			},
		},
	}
	req := &Request{
		Client: &kube.Client{Context: "c", Config: &rest.Config{Host: "h"}, History: h},
		Ref:    model.ObjectRef{Kind: "Deployment", Name: "payment-api", Namespace: "klew-lab"},
		Snapshot: model.EvidenceBundle{Events: []model.EventRecord{
			{UID: "dup", Type: "Warning", Reason: "BackOff", Message: "back-off", Count: 2,
				Timestamp:      model.TimestampFrom(now.Add(-time.Minute)),
				InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-api-7d9-x"}},
		}},
	}
	sec := eventsSection(context.Background(), req)

	if h.q.RegardingName != "payment-api" || !h.q.NamePrefix || h.q.Namespace != "klew-lab" {
		t.Fatalf("query = %+v", h.q)
	}
	if sec.Table == nil || len(sec.Table.Rows) != 2 {
		t.Fatalf("rows = %+v", sec.Table)
	}
	// Newest first; live row is not tagged, the expired one is.
	if sec.Table.Rows[0][2] != "BackOff" || sec.Table.RowTags[0] != "" {
		t.Fatalf("row0 = %v tag=%q", sec.Table.Rows[0], sec.Table.RowTags[0])
	}
	if sec.Table.Rows[1][2] != "Failed" || sec.Table.RowTags[1] != "recorded" {
		t.Fatalf("row1 = %v tag=%q", sec.Table.Rows[1], sec.Table.RowTags[1])
	}
	if sec.History == nil || sec.History.Since == "" {
		t.Fatal("missing history coverage")
	}
	if len(sec.History.Gaps) != 1 {
		t.Fatalf("gaps outside object lifetime should be dropped: %+v", sec.History.Gaps)
	}
}

func TestEventsSectionWithoutHistoryUnchanged(t *testing.T) {
	req := &Request{Ref: model.ObjectRef{Kind: "Pod", Name: "p"}}
	if sec := eventsSection(context.Background(), req); !sec.Empty() {
		t.Fatalf("expected empty section, got %+v", sec)
	}
}
