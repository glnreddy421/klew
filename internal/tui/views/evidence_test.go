package views

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/engine"
)

func TestFinalReportLayout(t *testing.T) {
	st := engine.DemoState()
	out := EvidenceView(st, "", 0, 28, 120)
	for _, want := range []string{
		"Summary",
		"Hypothesis",
		"Evidence",
		"Claims",
		"Cross Correlation",
		"Confidence",
		"Gaps & Next",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing section %q", want)
		}
	}
	for _, banned := range []string{"Log Findings", "Logs", "15:04:05  LOG", "15:04:05  EVENT"} {
		if strings.Contains(out, banned) {
			t.Errorf("final report should not contain %q", banned)
		}
	}
}

func TestFinalReportClaims(t *testing.T) {
	st := engine.DemoState()
	claims := buildFinalClaims(st)
	if len(claims) == 0 {
		t.Fatal("expected claims for demo OOM scenario")
	}
	if claims[0].confidence <= 0 {
		t.Fatalf("primary claim should have confidence, got %v", claims[0].confidence)
	}
}

func TestFinalReportNoLogGroups(t *testing.T) {
	st := engine.DemoState()
	groups := collectEvidenceGroups(st)
	if _, ok := groups["Log Findings"]; ok {
		t.Fatal("final report should not have Log Findings group")
	}
	if _, ok := groups["Logs"]; ok {
		t.Fatal("final report should not have Logs group")
	}
}

func TestInvestigationGapsDeterministic(t *testing.T) {
	st := engine.DemoState()
	gaps := realInvestigationGaps(st)
	if len(gaps) == 0 {
		t.Fatalf("expected gaps in demo when permissions limited, got %v", gaps)
	}
}

func TestConfidenceTimelineMonotonic(t *testing.T) {
	st := engine.DemoState()
	steps := buildConfidenceTimeline(st)
	if len(steps) < 2 {
		t.Fatalf("expected timeline steps, got %d", len(steps))
	}
	for i := 1; i < len(steps); i++ {
		if steps[i].conf < steps[i-1].conf {
			t.Fatalf("confidence should be monotonic: %v then %v", steps[i-1], steps[i])
		}
	}
}
