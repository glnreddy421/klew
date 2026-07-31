package bundle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestBundleRoundTrip(t *testing.T) {
	b := model.EvidenceBundle{
		CollectedAt: time.Now().UTC(),
		Namespace:   "prod",
		Query:       "payment",
		Workloads: []model.WorkloadSummary{{
			Kind: "Deployment", Name: "payment", Namespace: "prod", Replicas: 3, Ready: 2,
		}},
		Events: []model.EventRecord{{
			Timestamp: time.Now(), Reason: "BackOff", Message: "back-off restarting",
			InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "payment-abc"},
		}},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.json")
	if err := Write(path, b); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Namespace != "prod" || got.Query != "payment" {
		t.Fatalf("unexpected bundle: %+v", got)
	}
	if len(got.Workloads) != 1 || got.Workloads[0].Name != "payment" {
		t.Fatal("workload not preserved")
	}
	_ = os.Remove(path)
}
