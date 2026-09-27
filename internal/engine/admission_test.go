package engine

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestPrependAdmissionCausalChain(t *testing.T) {
	ap := &model.AdmissionPerimeterSummary{
		MissingWorkload: true,
		Status:          "candidates",
		Candidates: []model.AdmissionWebhookCandidate{{
			WebhookName:         "validate.kyverno.something",
			NamespaceScopeLabel: "env=prod",
			RecentSignals: []model.AdmissionWebhookSignal{{
				Outcome: "timeout",
				Message: "context deadline exceeded",
			}},
		}},
	}
	chain := prependAdmissionCausalChain(ap, []string{"FailedCreate", "BackOff"})
	if len(chain) < 2 || chain[0] == "" {
		t.Fatalf("expected admission head, got %v", chain)
	}
}

func TestScoreAdmissionDenied(t *testing.T) {
	b := model.EvidenceBundle{
		AdmissionPerimeter: &model.AdmissionPerimeterSummary{
			MissingWorkload: true,
			Status:          "denied",
			PermissionNote:  "no rbac",
		},
	}
	sigs := ScoreSignals(b)
	if len(sigs) == 0 || sigs[0].ID != "admission_perimeter_unknown" {
		t.Fatalf("expected rbac signal, got %+v", sigs)
	}
}
