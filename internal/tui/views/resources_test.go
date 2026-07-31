package views

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/engine"
)

func TestResourcesViewInvestigationLayout(t *testing.T) {
	st := engine.DemoState()
	out := ResourcesView(st, 120, 28)
	for _, want := range []string{
		"Workload Capacity",
		"Node Footprint",
		"Investigation Pods",
		"Co-located on Node",
		"Resource Investigation",
		"Findings",
		"Recommendations",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing section %q", want)
		}
	}
}

func TestGenerateResourceFindingsOOM(t *testing.T) {
	st := engine.DemoState()
	findings := generateResourceFindings(st, st.Snapshot, st.Snapshot.Metrics)
	if len(findings) == 0 {
		t.Fatal("expected findings for OOM demo")
	}
	foundOOM := false
	for _, f := range findings {
		if strings.Contains(f.text, "memory limit") || strings.Contains(f.text, "OOM") || strings.Contains(f.text, "Memory") {
			foundOOM = true
		}
	}
	if !foundOOM {
		t.Fatalf("expected memory-related finding, got %#v", findings)
	}
}

func TestGenerateResourceRecommendationsEvidenceBacked(t *testing.T) {
	st := engine.DemoState()
	findings := generateResourceFindings(st, st.Snapshot, st.Snapshot.Metrics)
	recs := generateResourceRecommendations(st, st.Snapshot, st.Snapshot.Metrics, findings)
	if len(recs) == 0 {
		t.Fatal("expected recommendations when OOM evidence present")
	}
}

func TestResourcesViewNoMetricsOOM(t *testing.T) {
	st := engine.DemoState()
	st.Snapshot.Metrics.Available = false
	st.Snapshot.Metrics.MemUsageMi = 0
	out := ResourcesView(st, 120, 28)
	if strings.Contains(out, "32Mi") {
		t.Error("should not show fake estimated usage when metrics unavailable")
	}
	if !strings.Contains(out, "at limit") && !strings.Contains(out, "OOMKilled") {
		t.Error("expected OOM/limit signal in resources view without metrics")
	}
	if !strings.Contains(out, "Allocatable") && !strings.Contains(out, "request") {
		t.Error("expected capacity/allocatable labeling")
	}
}

func TestParseMetricMessage(t *testing.T) {
	cpu, mem := parseMetricMessage("memory 780Mi · cpu 300m")
	if cpu != 300 || mem != 780 {
		t.Fatalf("got cpu=%d mem=%d", cpu, mem)
	}
	cpu, mem = parseMetricMessage("memory 2.4Gi · cpu 1200m")
	if cpu != 1200 || mem != 2457 {
		t.Fatalf("got cpu=%d mem=%d", cpu, mem)
	}
}
