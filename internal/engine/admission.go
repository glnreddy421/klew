package engine

import (
	"fmt"

	"github.com/glnreddy421/klew/internal/model"
)

// admissionTimelineEvents surfaces perimeter checks ahead of workload-scoped timeline entries.
func admissionTimelineEvents(b model.EvidenceBundle) []model.TimelineEvent {
	ap := b.AdmissionPerimeter
	if ap == nil {
		return nil
	}
	var out []model.TimelineEvent
	ts := ap.CheckedAt
	if ap.Status == "denied" {
		out = append(out, model.TimelineEvent{
			Timestamp:  ts,
			Type:       "admission",
			Severity:   model.SeverityWarning,
			SourceKind: "AdmissionPerimeter",
			SourceName: "RBAC",
			Namespace:  b.Namespace,
			Message:    ap.PermissionNote,
			Reason:     "AdmissionPerimeterDenied",
			Confidence: 1,
		})
		return out
	}
	if len(ap.Candidates) == 0 && ap.MissingWorkload {
		out = append(out, model.TimelineEvent{
			Timestamp:  ts,
			Type:       "admission",
			Severity:   model.SeverityInfo,
			SourceKind: "AdmissionPerimeter",
			SourceName: "Check",
			Namespace:  b.Namespace,
			Message:    "No namespace-scoped admission webhooks matched this workload (pods missing)",
			Reason:     "AdmissionPerimeterClear",
			Confidence: 0.7,
		})
		return out
	}
	for _, c := range ap.Candidates {
		sev := model.SeverityWarning
		msg := fmt.Sprintf("%s/%s webhook %q applies to namespace (%s)", c.ConfigKind, c.ConfigName, c.WebhookName, c.NamespaceScopeLabel)
		if c.FailurePolicy == "Fail" {
			sev = model.SeverityHigh
		}
		if len(c.RecentSignals) > 0 {
			sig := c.RecentSignals[0]
			sev = model.SeverityCritical
			msg = msg + " — " + sig.Message
		} else if c.TimeoutSeconds > 0 {
			msg = fmt.Sprintf("%s (timeout %ds)", msg, c.TimeoutSeconds)
		}
		out = append(out, model.TimelineEvent{
			Timestamp:  ts,
			Type:       "admission",
			Severity:   sev,
			SourceKind: c.ConfigKind,
			SourceName: c.ConfigName,
			Namespace:  b.Namespace,
			Message:    msg,
			Reason:     admissionReason(c),
			Confidence: 0.85,
			InvolvedObject: model.ObjectRef{
				Kind: c.ConfigKind,
				Name: c.ConfigName,
			},
			EvidenceRefs: []string{fmt.Sprintf("admission:%s/%s", c.ConfigKind, c.WebhookName)},
		})
	}
	return out
}

func admissionReason(c model.AdmissionWebhookCandidate) string {
	if len(c.RecentSignals) > 0 {
		switch c.RecentSignals[0].Outcome {
		case "timeout":
			return "WebhookTimeout"
		case "deny":
			return "WebhookDenied"
		case "fail":
			return "WebhookFailure"
		default:
			return "WebhookFailure"
		}
	}
	return "WebhookCandidate"
}

func scoreAdmissionSignals(b model.EvidenceBundle) []model.Signal {
	ap := b.AdmissionPerimeter
	if ap == nil || !ap.MissingWorkload {
		return nil
	}
	if ap.Status == "denied" {
		return []model.Signal{{
			ID: "admission_perimeter_unknown", Label: "Admission perimeter unknown (RBAC)",
			Severity: model.SeverityWarning, Strength: "medium", Score: 72,
			Evidence: ap.PermissionNote,
		}}
	}
	var signals []model.Signal
	for _, c := range ap.Candidates {
		score := 78.0
		sev := model.SeverityHigh
		strength := "medium"
		label := "Admission webhook may block pod creation"
		evidence := fmt.Sprintf("%s %s/%s scope=%s", c.ConfigKind, c.ConfigName, c.WebhookName, c.NamespaceScopeLabel)
		if len(c.RecentSignals) > 0 {
			score = 94
			sev = model.SeverityCritical
			strength = "strong"
			label = "Admission webhook failure"
			evidence = c.RecentSignals[0].Message
		} else if c.FailurePolicy == "Fail" {
			score = 85
		}
		signals = append(signals, model.Signal{
			ID:       fmt.Sprintf("admission_webhook_%s_%s", c.ConfigName, c.WebhookName),
			Label:    label,
			Severity: sev,
			Strength: strength,
			Score:    score,
			Evidence: evidence,
			ObjectRef: model.ObjectRef{
				Kind: c.ConfigKind,
				Name: c.ConfigName,
			},
		})
	}
	return signals
}

func prependAdmissionCausalChain(ap *model.AdmissionPerimeterSummary, chain []string) []string {
	if ap == nil || !ap.MissingWorkload {
		return chain
	}
	var head []string
	if ap.Status == "denied" {
		head = []string{"Perimeter check: insufficient permissions"}
	} else {
		for _, c := range ap.Candidates {
			if len(c.RecentSignals) > 0 {
				head = append(head, fmt.Sprintf("Webhook %s: %s", c.WebhookName, c.RecentSignals[0].Outcome))
			} else {
				head = append(head, fmt.Sprintf("Webhook %s (%s)", c.WebhookName, c.NamespaceScopeLabel))
			}
			if len(head) >= 2 {
				break
			}
		}
	}
	if len(head) == 0 {
		return chain
	}
	return append(head, chain...)
}
