package engine

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// Rules for workload objects, ConfigMaps, Ingresses and admission webhooks.
// Pod failures already appear through the pod-state rule; these add what only
// the owning object shows.

func workloadRules() []Rule {
	return []Rule{
		findingRule("cronjob", cronJobFindings),
		findingRule("statefulset", statefulSetFindings),
		findingRule("daemonset", daemonSetFindings),
		findingRule("configmap-missing", configMapFindings),
		findingRule("ingress", ingressFindings),
		findingRule("webhook-backend", webhookFindings),
	}
}

func cronJobFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, obj := range b.Resources["cronjobs"] {
		cj, ok := typed[batchv1.CronJob](obj)
		if !ok {
			continue
		}
		ref := objRef(obj, "CronJob")
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityWarning, State: "CronJobSuspended",
				Facts: []string{fmt.Sprintf("CronJob %s is suspended, so schedule %q does not run", cj.Name, cj.Spec.Schedule)}})
			continue
		}
		last, success := cj.Status.LastScheduleTime, cj.Status.LastSuccessfulTime
		if last != nil && (success == nil || success.Before(last)) && len(cj.Status.Active) == 0 {
			since := "it has never succeeded"
			if success != nil {
				since = "last success " + success.UTC().Format("2006-01-02 15:04 MST")
			}
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityHigh, State: "CronJobFailing",
				Facts: []string{fmt.Sprintf("last run %s did not succeed; %s", last.UTC().Format("2006-01-02 15:04 MST"), since)}})
		}
		if msg := cronJobScheduleProblem(b, cj.Name); msg != "" {
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityHigh, State: "CronJobMissedSchedule", Facts: []string{msg}})
		}
	}
	return out
}

// cronJobScheduleProblem is the newest event saying the controller missed or
// could not start a scheduled run.
func cronJobScheduleProblem(b model.EvidenceBundle, name string) string {
	for _, e := range b.Events {
		if currentEvent(b, e) && e.InvolvedObject.Kind == "CronJob" && e.InvolvedObject.Name == name &&
			(e.Reason == "FailedNeedsStart" || e.Reason == "TooManyMissedTimes" || e.Reason == "FailedCreate") {
			return e.Reason + ": " + e.Message
		}
	}
	return ""
}

func statefulSetFindings(b model.EvidenceBundle) []model.Finding {
	services := map[string]bool{}
	for _, s := range b.Services {
		services[s.Name] = true
	}
	var out []model.Finding
	for _, obj := range b.Resources["statefulsets"] {
		sts, ok := typed[appsv1.StatefulSet](obj)
		if !ok {
			continue
		}
		ref := objRef(obj, "StatefulSet")
		want := int32(1)
		if sts.Spec.Replicas != nil {
			want = *sts.Spec.Replicas
		}
		st := sts.Status
		if st.UpdateRevision != "" && st.CurrentRevision != st.UpdateRevision && st.UpdatedReplicas < want {
			fact := fmt.Sprintf("rollout to %s has updated %d of %d pods", st.UpdateRevision, st.UpdatedReplicas, want)
			if sts.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
				fact += "; updateStrategy OnDelete waits for pods to be deleted by hand"
			} else if st.ReadyReplicas < want {
				fact += fmt.Sprintf("; it waits for each pod to be Ready (%d of %d are)", st.ReadyReplicas, want)
			}
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityWarning, State: "RolloutStalled", Facts: []string{fact}})
		}
		if b.Query == "" && sts.Spec.ServiceName != "" && !services[sts.Spec.ServiceName] {
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityWarning, State: "GoverningServiceMissing",
				Facts: []string{fmt.Sprintf("serviceName %s does not exist, so pods get no stable DNS names", sts.Spec.ServiceName)}})
		}
	}
	return out
}

func daemonSetFindings(b model.EvidenceBundle) []model.Finding {
	var out []model.Finding
	for _, obj := range b.Resources["daemonsets"] {
		ds, ok := typed[appsv1.DaemonSet](obj)
		if !ok {
			continue
		}
		ref := objRef(obj, "DaemonSet")
		st := ds.Status
		switch {
		case st.DesiredNumberScheduled == 0 && len(ds.Spec.Template.Spec.NodeSelector) > 0:
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityWarning, State: "DaemonSetOnNoNodes",
				Facts: []string{fmt.Sprintf("nodeSelector %s matches no node", selectorString(ds.Spec.Template.Spec.NodeSelector))}})
		case st.NumberUnavailable > 0:
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityHigh, State: "DaemonSetUnavailable",
				Facts: []string{fmt.Sprintf("%d of %d nodes have no available pod", st.NumberUnavailable, st.DesiredNumberScheduled)}})
		}
		if st.NumberMisscheduled > 0 {
			out = append(out, model.Finding{Object: ref, Severity: model.SeverityWarning, State: "DaemonSetMisscheduled",
				Facts: []string{fmt.Sprintf("%d pods run on nodes they should not", st.NumberMisscheduled)}})
		}
	}
	return out
}

func selectorString(m map[string]string) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// configMapFindings reports unready pods that use a ConfigMap that does not
// exist. Ready pods are skipped: their reference may be optional.
func configMapFindings(b model.EvidenceBundle) []model.Finding {
	maps, ok := b.Resources["configmaps"]
	if !ok || len(maps) >= kube.MaxResourceLimit {
		// At the collector's cap the list may be partial.
		return nil
	}
	have := names(maps)
	var out []model.Finding
	for _, p := range b.Pods {
		if !PodCountsAsUnready(p) {
			continue
		}
		for _, cm := range p.ConfigMapRefs {
			if !have[cm] {
				out = append(out, model.Finding{
					Object:   model.ObjectRef{Kind: "Pod", Name: p.Name, Namespace: p.Namespace},
					Severity: model.SeverityCritical, State: "ConfigMapMissing",
					Facts: []string{fmt.Sprintf("pod %s uses ConfigMap %s, which does not exist", p.Name, cm)},
				})
			}
		}
	}
	return out
}

func ingressFindings(b model.EvidenceBundle) []model.Finding {
	services := map[string]model.ServiceSummary{}
	for _, s := range b.Services {
		services[s.Name] = s
	}
	classes, haveClasses := b.Resources["ingressclasses"]
	classNames := names(classes)
	defaultClass := ""
	for _, c := range classes {
		if nestedStr(c, "metadata", "annotations", "ingressclass.kubernetes.io/is-default-class") == "true" {
			defaultClass = objName(c)
		}
	}
	var out []model.Finding
	for _, ing := range b.Ingresses {
		f := model.Finding{Object: model.ObjectRef{Kind: "Ingress", Name: ing.Name, Namespace: ing.Namespace}, Severity: model.SeverityHigh}
		switch {
		case ing.ClassName != "" && haveClasses && !classNames[ing.ClassName]:
			f.State = "IngressClassMissing"
			f.Facts = append(f.Facts, fmt.Sprintf("IngressClass %s does not exist, so no controller serves this Ingress", ing.ClassName))
		case ing.ClassName == "" && haveClasses && defaultClass == "":
			f.Severity, f.State = model.SeverityWarning, "IngressClassUnset"
			f.Facts = append(f.Facts, "no ingressClassName and the cluster has no default IngressClass")
		}
		if b.Query == "" {
			for _, name := range uniqueStrings(ing.Backends) {
				svc, ok := services[name]
				switch {
				case !ok:
					if f.State == "" {
						f.State = "IngressBackendMissing"
					}
					f.Facts = append(f.Facts, fmt.Sprintf("backend Service %s does not exist", name))
				case svc.TotalEndpoints > 0 && svc.ReadyEndpoints == 0:
					if f.State == "" {
						f.State = "IngressBackendUnready"
					}
					f.Facts = append(f.Facts, fmt.Sprintf("backend Service %s has 0/%d endpoints ready", name, svc.TotalEndpoints))
				}
			}
		}
		if f.State != "" {
			out = append(out, f)
		}
	}
	return out
}

// webhookFindings reports admission webhooks served from this namespace whose
// Service is missing or has no ready endpoints. With failurePolicy Fail every
// matching API request in the cluster is rejected while they are down.
func webhookFindings(b model.EvidenceBundle) []model.Finding {
	if b.Query != "" {
		return nil
	}
	services := map[string]model.ServiceSummary{}
	for _, s := range b.Services {
		services[s.Name] = s
	}
	var out []model.Finding
	for _, kind := range []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"} {
		for _, cfg := range b.Resources[kind] {
			hooks, _ := nested(cfg, "webhooks").([]any)
			for _, h := range hooks {
				hm, _ := h.(map[string]any)
				if nestedStr(hm, "clientConfig", "service", "namespace") != b.Namespace {
					continue
				}
				svcName := nestedStr(hm, "clientConfig", "service", "name")
				svc, ok := services[svcName]
				var problem string
				switch {
				case !ok:
					problem = fmt.Sprintf("its Service %s does not exist", svcName)
				case svc.ReadyEndpoints == 0:
					problem = fmt.Sprintf("its Service %s has 0/%d endpoints ready", svcName, svc.TotalEndpoints)
				default:
					continue
				}
				policy := nestedStr(hm, "failurePolicy")
				if policy == "" {
					policy = "Fail"
				}
				f := model.Finding{Object: objRef(cfg, webhookKinds[kind]), Severity: model.SeverityWarning, State: "WebhookUnavailable",
					Facts: []string{fmt.Sprintf("webhook %s is down: %s", nestedStr(hm, "name"), problem)}}
				if policy == "Fail" {
					f.Severity = model.SeverityCritical
					f.Facts = append(f.Facts, "failurePolicy Fail: matching create and update requests across the cluster are rejected until it is back")
				}
				out = append(out, f)
			}
		}
	}
	return out
}

var webhookKinds = map[string]string{
	"mutatingwebhookconfigurations":   "MutatingWebhookConfiguration",
	"validatingwebhookconfigurations": "ValidatingWebhookConfiguration",
}

func uniqueStrings(in []string) []string {
	var out []string
	for _, s := range in {
		out = appendUniqueString(out, s)
	}
	return out
}
