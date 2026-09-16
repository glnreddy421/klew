package engine

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestShouldRefreshOnStructuralChange(t *testing.T) {
	tests := []struct {
		name string
		ev   model.EvidenceEvent
		want bool
	}{
		{
			name: "container backoff",
			ev: model.EvidenceEvent{
				SourceType: model.SourceObjectChange,
				SourceKind: "Container",
				Reason:     "CrashLoopBackOff",
				Severity:   model.SeverityHigh,
			},
			want: true,
		},
		{
			name: "pod failed phase",
			ev: model.EvidenceEvent{
				SourceType: model.SourceObjectChange,
				SourceKind: "Pod",
				Reason:     "MODIFIED",
				Message:    "Pod api-1 phase=Failed",
				Severity:   model.SeverityInfo,
			},
			want: true,
		},
		{
			name: "pod running modified ignored",
			ev: model.EvidenceEvent{
				SourceType: model.SourceObjectChange,
				SourceKind: "Pod",
				Reason:     "MODIFIED",
				Message:    "Pod api-1 phase=Running",
				Severity:   model.SeverityInfo,
			},
			want: false,
		},
		{
			name: "k8s oom event",
			ev: model.EvidenceEvent{
				SourceType: model.SourceK8sEvent,
				Reason:     "OOMKilling",
				Severity:   model.SeverityCritical,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRefreshOnStructuralChange(tt.ev); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
