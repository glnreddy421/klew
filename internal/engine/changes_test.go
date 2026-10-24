package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func marker(name string, rev int64, at time.Time, areas ...string) model.WorkloadChangeMarker {
	return model.WorkloadChangeMarker{Kind: "Deployment", Name: name, Namespace: "klew-lab", Revision: rev,
		Time: model.TimestampFrom(at), Source: model.ChangeSourceRollout, Areas: areas, ObjectName: name + "-rs"}
}

func TestPinChangeWithinWindow(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 0, 0, 0, time.UTC)
	pin := PinChange([]model.WorkloadChangeMarker{marker("payment-api", 14, now.Add(-12*time.Minute), "image", "env")}, nil, now, 15*time.Minute)
	if pin == nil || pin.Text != "payment-api changed 12m ago: rev 14 (image, env)" {
		t.Fatalf("pin = %+v", pin)
	}
}

func TestPinChangeOlderThanWindowNotPinned(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 0, 0, 0, time.UTC)
	if pin := PinChange([]model.WorkloadChangeMarker{marker("payment-api", 14, now.Add(-3*time.Hour), "image")}, nil, now, 15*time.Minute); pin != nil {
		t.Fatalf("old change pinned: %+v", pin)
	}
}

func TestPinChangeBeforeFirstIncident(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 0, 0, 0, time.UTC)
	incidentStart := now.Add(-2 * time.Hour)
	incidents := []model.Incident{{Start: model.TimestampFrom(incidentStart), End: model.TimestampFrom(now.Add(-90 * time.Minute))}}
	changes := []model.WorkloadChangeMarker{
		marker("payment-api", 14, incidentStart.Add(-12*time.Minute), "image", "env"), // 12m before: pinned
		marker("redis", 3, incidentStart.Add(-3*time.Hour), "image"),                  // too early
	}
	pin := PinChange(changes, incidents, now, 15*time.Minute)
	if pin == nil || !pin.BeforeIncident || pin.Text != "payment-api changed 12m before the incident started: rev 14 (image, env)" {
		t.Fatalf("pin = %+v", pin)
	}
	// More than an hour before the incident and outside the window: nothing.
	if p := PinChange(changes[1:], incidents, now, 15*time.Minute); p != nil {
		t.Fatalf("pinned a change 3h before the incident: %+v", p)
	}
}

func TestPinChangeNoMarkers(t *testing.T) {
	if PinChange(nil, nil, time.Now(), 15*time.Minute) != nil {
		t.Fatal("nothing to pin")
	}
}

func TestRolloutTimelineEnrichedAndInChain(t *testing.T) {
	now := time.Now().UTC()
	changeAt := now.Add(-10 * time.Minute)
	b := model.EvidenceBundle{
		Namespace: "klew-lab",
		ReplicaSets: []model.ReplicaSetSummary{
			{Name: "payment-api-rs", Namespace: "klew-lab", CreatedAt: model.TimestampFrom(now.Add(-48 * time.Hour)), DeploymentOwner: "payment-api"},
			{Name: "payment-api-old", Namespace: "klew-lab", CreatedAt: model.TimestampFrom(now.Add(-72 * time.Hour)), DeploymentOwner: "payment-api"},
		},
		WorkloadChanges: []model.WorkloadChangeMarker{{
			Kind: "Deployment", Name: "payment-api", Namespace: "klew-lab", Revision: 14, Time: model.TimestampFrom(changeAt),
			Source: model.ChangeSourceRollout, Summary: "image v2 → v3", Areas: []string{"image"}, ObjectName: "payment-api-rs",
		}},
		Events: []model.EventRecord{{Type: "Warning", Reason: "BackOff", Timestamp: model.TimestampFrom(now.Add(-5 * time.Minute)),
			InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-api-x"}}},
	}
	tl := BuildTimeline(b)
	var enriched, plain int
	for _, e := range tl {
		if e.Type != "rollout" {
			continue
		}
		if e.Reason == "Rollout: image" {
			enriched++
			if e.Timestamp != model.TimestampFrom(changeAt) || !strings.Contains(e.Message, "rev 14: image v2 → v3") {
				t.Fatalf("enriched entry = %+v", e)
			}
		} else {
			plain++
		}
	}
	if enriched != 1 || plain != 1 {
		t.Fatalf("enriched=%d plain=%d", enriched, plain)
	}
	chain := strings.Join(buildCausalChain(tl), " | ")
	if !strings.HasPrefix(chain, "Rollout: image") || !strings.Contains(chain, "BackOff") {
		t.Fatalf("change should lead the chain before the failure: %q", chain)
	}
}
