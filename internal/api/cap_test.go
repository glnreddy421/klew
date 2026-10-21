package api

import (
	"fmt"
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestCapEventsPrefersWarnings(t *testing.T) {
	var events []model.EventRecord
	events = append(events, model.EventRecord{UID: "old-warning", Type: "Warning"})
	for i := 0; i < 150; i++ {
		events = append(events, model.EventRecord{UID: fmt.Sprintf("n%d", i), Type: "Normal"})
	}
	out := capEvents(events, 100)
	if len(out) != 100 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0].UID != "old-warning" {
		t.Fatalf("warning dropped; first = %s", out[0].UID)
	}
	if out[99].UID != "n149" {
		t.Fatalf("newest normal dropped; last = %s", out[99].UID)
	}
}
