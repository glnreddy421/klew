package kube

import (
	"context"
	"fmt"
	"strings"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
)

const admissionPermissionNote = "Insufficient permissions to list admission webhook configurations (cluster-scoped). Perimeter layer is unknown — not a clean bill of health."

// WorkloadMissingPods is true when a targeted Deployment expects replicas but no scoped pods exist.
func WorkloadMissingPods(b model.EvidenceBundle) bool {
	if len(b.Workloads) == 0 {
		return false
	}
	rsOwners := replicaSetOwners(b.ReplicaSets)
	for _, w := range b.Workloads {
		if w.Replicas <= 0 {
			continue
		}
		if w.Ready >= w.Replicas {
			continue
		}
		if countPodsForDeployment(b.Pods, w.Name, rsOwners) == 0 {
			return true
		}
	}
	return false
}

func replicaSetOwners(rss []model.ReplicaSetSummary) map[string]string {
	out := make(map[string]string, len(rss))
	for _, rs := range rss {
		if rs.DeploymentOwner != "" {
			out[rs.Name] = rs.DeploymentOwner
		}
	}
	return out
}

func countPodsForDeployment(pods []model.PodSummary, deployName string, rsOwners map[string]string) int {
	n := 0
	for _, p := range pods {
		if podBelongsToDeployment(p, deployName, rsOwners) {
			n++
		}
	}
	return n
}

func podBelongsToDeployment(p model.PodSummary, deployName string, rsOwners map[string]string) bool {
	for _, o := range p.OwnerRefs {
		if o.Kind == "ReplicaSet" {
			if owner := rsOwners[o.Name]; owner == deployName {
				return true
			}
		}
		if o.Kind == "Deployment" && o.Name == deployName {
			return true
		}
	}
	return false
}

// AdmissionWebhookListAccess probes cluster-scoped list on mutating and validating webhook configs.
func AdmissionWebhookListAccess(ctx context.Context, c *Client) (mutating, validating bool) {
	mutating = selfCanListWebhookConfigs(ctx, c, "mutatingwebhookconfigurations")
	validating = selfCanListWebhookConfigs(ctx, c, "validatingwebhookconfigurations")
	return mutating, validating
}

func selfCanListWebhookConfigs(ctx context.Context, c *Client, resource string) bool {
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:     "list",
				Resource: resource,
			},
		},
	}
	result, err := c.Clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	return err == nil && result.Status.Allowed
}

// CollectAdmissionPerimeter runs when opt-in and workloads are missing pods.
func (col *Collector) CollectAdmissionPerimeter(ctx context.Context, b *model.EvidenceBundle) *model.AdmissionPerimeterSummary {
	if b == nil || !WorkloadMissingPods(*b) {
		return nil
	}
	now := model.TimestampFrom(time.Now().UTC())
	summary := &model.AdmissionPerimeterSummary{
		CheckedAt:       now,
		MissingWorkload: true,
		Status:          "ok",
	}
	mutatingOK, validatingOK := AdmissionWebhookListAccess(ctx, col.Client)
	if !mutatingOK && !validatingOK {
		summary.Status = "denied"
		summary.PermissionNote = admissionPermissionNote
		return summary
	}

	nsLabels, err := col.namespaceLabels(ctx, b.Namespace)
	if err != nil {
		b.Warnings = append(b.Warnings, fmt.Sprintf("admission perimeter: namespace labels: %v", err))
	}

	var candidates []model.AdmissionWebhookCandidate
	if mutatingOK {
		list, err := col.Client.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
		if err != nil {
			summary.Status = "denied"
			summary.PermissionNote = admissionPermissionNote
			return summary
		}
		for _, cfg := range list.Items {
			candidates = append(candidates, mutatingWebhookCandidates(cfg, b.Namespace, nsLabels)...)
		}
	}
	if validatingOK {
		list, err := col.Client.Clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
		if err != nil {
			summary.Status = "denied"
			summary.PermissionNote = admissionPermissionNote
			return summary
		}
		for _, cfg := range list.Items {
			candidates = append(candidates, validatingWebhookCandidates(cfg, b.Namespace, nsLabels)...)
		}
	}

	for i := range candidates {
		candidates[i].RecentSignals = admissionSignalsFromEvents(b.Events, candidates[i])
	}
	summary.Candidates = candidates
	if len(candidates) > 0 {
		summary.Status = "candidates"
	}
	return summary
}

func (col *Collector) namespaceLabels(ctx context.Context, ns string) (map[string]string, error) {
	if ns == "" {
		return map[string]string{}, nil
	}
	obj, err := col.Client.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		return map[string]string{}, err
	}
	return obj.Labels, nil
}

func mutatingWebhookCandidates(cfg admissionregistrationv1.MutatingWebhookConfiguration, ns string, nsLabels map[string]string) []model.AdmissionWebhookCandidate {
	var out []model.AdmissionWebhookCandidate
	for _, h := range cfg.Webhooks {
		if c, ok := candidateFromHook(cfg.Name, "MutatingWebhookConfiguration", h.Name, h.ClientConfig, h.Rules, h.FailurePolicy, h.TimeoutSeconds, h.NamespaceSelector, h.ObjectSelector, ns, nsLabels); ok {
			out = append(out, c)
		}
	}
	return out
}

func validatingWebhookCandidates(cfg admissionregistrationv1.ValidatingWebhookConfiguration, ns string, nsLabels map[string]string) []model.AdmissionWebhookCandidate {
	var out []model.AdmissionWebhookCandidate
	for _, h := range cfg.Webhooks {
		if c, ok := candidateFromHook(cfg.Name, "ValidatingWebhookConfiguration", h.Name, h.ClientConfig, h.Rules, h.FailurePolicy, h.TimeoutSeconds, h.NamespaceSelector, h.ObjectSelector, ns, nsLabels); ok {
			out = append(out, c)
		}
	}
	return out
}

func candidateFromHook(
	configName, configKind, hookName string,
	client admissionregistrationv1.WebhookClientConfig,
	rules []admissionregistrationv1.RuleWithOperations,
	failurePolicy *admissionregistrationv1.FailurePolicyType,
	timeout *int32,
	nsSel, objSel *metav1.LabelSelector,
	ns string,
	nsLabels map[string]string,
) (model.AdmissionWebhookCandidate, bool) {
	if !WebhookRulesMatchWorkloads(rules) {
		return model.AdmissionWebhookCandidate{}, false
	}
	match, err := NamespaceSelectorMatches(ns, nsLabels, nsSel)
	if err != nil || !match {
		return model.AdmissionWebhookCandidate{}, false
	}
	fp := ""
	if failurePolicy != nil {
		fp = string(*failurePolicy)
	}
	var timeoutSec int32
	if timeout != nil {
		timeoutSec = *timeout
	}
	scopeLabel := FormatNamespaceScopeLabel(nsSel)
	if objSel != nil && (len(objSel.MatchLabels) > 0 || len(objSel.MatchExpressions) > 0) {
		scopeLabel = scopeLabel + " · objects: " + FormatNamespaceScopeLabel(objSel)
	}
	kindNote := "mutating"
	if configKind == "ValidatingWebhookConfiguration" {
		kindNote = "validating (CREATE deny prevents persisting objects; FailedCreate events may still appear on ReplicaSet)"
	}
	return model.AdmissionWebhookCandidate{
		ConfigKind:          configKind,
		ConfigName:          configName,
		WebhookName:         hookName,
		NamespaceScopeLabel: scopeLabel,
		FailurePolicy:       fp,
		TimeoutSeconds:      timeoutSec,
		ServiceRef:          webhookServiceRef(client),
		MatchSummary:        kindNote,
	}, true
}

func webhookServiceRef(c admissionregistrationv1.WebhookClientConfig) string {
	if c.URL != nil && *c.URL != "" {
		return *c.URL
	}
	if c.Service != nil {
		port := int32(443)
		if c.Service.Port != nil {
			port = *c.Service.Port
		}
		path := ""
		if c.Service.Path != nil {
			path = *c.Service.Path
		}
		return fmt.Sprintf("%s/%s:%d%s", c.Service.Namespace, c.Service.Name, port, path)
	}
	return ""
}

func admissionSignalsFromEvents(events []model.EventRecord, c model.AdmissionWebhookCandidate) []model.AdmissionWebhookSignal {
	var out []model.AdmissionWebhookSignal
	needles := []string{
		strings.ToLower(c.WebhookName),
		strings.ToLower(c.ConfigName),
		"failed calling webhook",
		"admission webhook",
		"denied the request",
	}
	for _, e := range events {
		msg := strings.ToLower(e.Message)
		reason := strings.ToLower(e.Reason)
		if !eventMentionsAdmission(msg, reason, needles) {
			continue
		}
		if c.WebhookName != "" {
			wh := strings.ToLower(c.WebhookName)
			cn := strings.ToLower(c.ConfigName)
			if !strings.Contains(msg, wh) && !strings.Contains(msg, cn) &&
				!strings.Contains(msg, "failed calling webhook") {
				continue
			}
		}
		out = append(out, model.AdmissionWebhookSignal{
			Timestamp: e.Timestamp,
			Reason:    e.Reason,
			Message:   e.Message,
			Outcome:   classifyAdmissionOutcome(e.Reason, e.Message),
		})
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func eventMentionsAdmission(msg, reason string, needles []string) bool {
	if reason == "failedcreate" || strings.Contains(reason, "failed") {
		if strings.Contains(msg, "webhook") || strings.Contains(msg, "admission") {
			return true
		}
	}
	for _, n := range needles {
		if n != "" && strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

func classifyAdmissionOutcome(reason, message string) string {
	r := strings.ToLower(reason + " " + message)
	switch {
	case strings.Contains(r, "timeout"), strings.Contains(r, "deadline"), strings.Contains(r, "context deadline"):
		return "timeout"
	case strings.Contains(r, "denied"), strings.Contains(r, "deny"):
		return "deny"
	case strings.Contains(r, "failed calling webhook"):
		return "fail"
	default:
		return "warn"
	}
}
