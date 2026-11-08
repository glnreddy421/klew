package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestWorkloadRules(t *testing.T) {
	suspended := with(with(meta("CronJob", "report"), "spec.suspend", true), "spec.schedule", "0 * * * *")
	failing := with(with(with(meta("CronJob", "backup"), "spec.schedule", "*/5 * * * *"),
		"status.lastScheduleTime", "2026-11-05T10:00:00Z"), "status.lastSuccessfulTime", "2026-11-04T10:00:00Z")
	healthyCron := with(with(with(meta("CronJob", "ok"), "spec.schedule", "@hourly"),
		"status.lastScheduleTime", "2026-11-05T10:00:00Z"), "status.lastSuccessfulTime", "2026-11-05T10:00:05Z")
	sts := with(with(with(meta("StatefulSet", "db"), "spec.replicas", int64(3)), "spec.serviceName", "db-headless"),
		"status", obj{"currentRevision": "db-1", "updateRevision": "db-2", "updatedReplicas": int64(1), "readyReplicas": int64(2)})
	dsNoNodes := with(with(meta("DaemonSet", "gpu-agent"), "spec.template.spec.nodeSelector", obj{"gpu": "true"}), "status.desiredNumberScheduled", int64(0))
	dsDown := with(meta("DaemonSet", "log-shipper"), "status", obj{"desiredNumberScheduled": int64(4), "numberUnavailable": int64(2)})
	whFail := with(meta("ValidatingWebhookConfiguration", "policy"), "webhooks", []any{obj{"name": "check.example.com",
		"clientConfig": obj{"service": obj{"namespace": "ns", "name": "policy-svc"}}}})
	whIgnore := with(meta("MutatingWebhookConfiguration", "inject"), "webhooks", []any{obj{"name": "inject.example.com", "failurePolicy": "Ignore",
		"clientConfig": obj{"service": obj{"namespace": "ns", "name": "absent"}}}})
	whElsewhere := with(meta("MutatingWebhookConfiguration", "other"), "webhooks", []any{obj{"name": "x",
		"clientConfig": obj{"service": obj{"namespace": "kube-system", "name": "absent"}}}})

	unready := model.PodSummary{Name: "app-1", Namespace: "ns", Phase: "Pending", ConfigMapRefs: []string{"app-config", "present"}}
	ready := model.PodSummary{Name: "app-2", Namespace: "ns", Phase: "Running", Ready: true, ConfigMapRefs: []string{"optional-missing"}}
	b := model.EvidenceBundle{
		Namespace: "ns",
		Pods:      []model.PodSummary{unready, ready},
		Services:  []model.ServiceSummary{{Name: "web", TotalEndpoints: 2}, {Name: "policy-svc", TotalEndpoints: 1}},
		Ingresses: []model.IngressSummary{
			{Name: "shop", Namespace: "ns", ClassName: "nginx-internal", Backends: []string{"web", "web", "api"}},
		},
		Events: []model.EventRecord{{Reason: "TooManyMissedTimes", Message: "too many missed start times. Set or decrease .spec.startingDeadlineSeconds",
			InvolvedObject: model.ObjectRef{Kind: "CronJob", Name: "backup"}}},
		Resources: map[string][]map[string]any{
			"cronjobs":                        {suspended, failing, healthyCron},
			"statefulsets":                    {sts},
			"daemonsets":                      {dsNoNodes, dsDown},
			"configmaps":                      {with(meta("ConfigMap", "present"), "keys", []string{"a"})},
			"ingressclasses":                  {meta("IngressClass", "nginx")},
			"validatingwebhookconfigurations": {whFail},
			"mutatingwebhookconfigurations":   {whIgnore, whElsewhere},
		},
	}
	cases := []struct {
		rule, state string
		sev         model.Severity
		facts       []string
	}{
		{"cronjob", "CronJobSuspended", model.SeverityWarning, []string{`schedule "0 * * * *" does not run`}},
		{"cronjob", "CronJobFailing", model.SeverityHigh, []string{"last run 2026-11-05 10:00 UTC did not succeed; last success 2026-11-04 10:00 UTC"}},
		{"cronjob", "CronJobMissedSchedule", model.SeverityHigh, []string{"TooManyMissedTimes"}},
		{"statefulset", "RolloutStalled", model.SeverityWarning, []string{"rollout to db-2 has updated 1 of 3 pods; it waits for each pod to be Ready (2 of 3 are)"}},
		{"statefulset", "GoverningServiceMissing", model.SeverityWarning, []string{"serviceName db-headless does not exist"}},
		{"daemonset", "DaemonSetOnNoNodes", model.SeverityWarning, []string{"nodeSelector gpu=true matches no node"}},
		{"daemonset", "DaemonSetUnavailable", model.SeverityHigh, []string{"2 of 4 nodes have no available pod"}},
		{"configmap-missing", "ConfigMapMissing", model.SeverityCritical, []string{"pod app-1 uses ConfigMap app-config, which does not exist"}},
		{"ingress", "IngressClassMissing", model.SeverityHigh, []string{"IngressClass nginx-internal does not exist", "backend Service web has 0/2 endpoints ready", "backend Service api does not exist"}},
		{"webhook-backend", "WebhookUnavailable", model.SeverityCritical, []string{"webhook check.example.com is down: its Service policy-svc has 0/1 endpoints ready", "failurePolicy Fail"}},
		{"webhook-backend", "WebhookUnavailable", model.SeverityWarning, []string{"webhook inject.example.com is down: its Service absent does not exist"}},
	}
	got := map[string][]model.Finding{}
	for _, f := range DefaultRegistry().Evaluate(b).Findings {
		got[f.Rule] = append(got[f.Rule], f)
	}
	for _, tc := range cases {
		found := false
		for _, f := range got[tc.rule] {
			facts := strings.Join(f.Facts, "; ")
			all := f.State == tc.state && f.Severity == tc.sev
			for _, w := range tc.facts {
				all = all && strings.Contains(facts, w)
			}
			found = found || all
		}
		if !found {
			t.Errorf("%s: no %s/%s finding with %q; got %+v", tc.rule, tc.state, tc.sev, tc.facts, got[tc.rule])
		}
	}
	for _, f := range got["cronjob"] {
		if f.Object.Name == "ok" {
			t.Errorf("healthy CronJob reported: %+v", f)
		}
	}
	for _, f := range got["configmap-missing"] {
		if f.Object.Name == "app-2" {
			t.Errorf("ready pod with a possibly optional ConfigMap reported: %+v", f)
		}
	}
	if n := len(got["webhook-backend"]); n != 2 {
		t.Errorf("webhook findings = %d, want 2 (the kube-system webhook is out of scope)", n)
	}
	if n := len(got["ingress"]); n != 1 || strings.Count(strings.Join(got["ingress"][0].Facts, ";"), "Service web") != 1 {
		t.Errorf("ingress findings = %+v, want one, naming web once", got["ingress"])
	}
}

func TestWorkloadRulesSkipScopedChecksWithQuery(t *testing.T) {
	b := model.EvidenceBundle{
		Query:     "api",
		Ingresses: []model.IngressSummary{{Name: "i", Backends: []string{"not-collected"}}},
		Resources: map[string][]map[string]any{"statefulsets": {with(meta("StatefulSet", "db"), "spec.serviceName", "not-collected")}},
	}
	for _, f := range DefaultRegistry().Evaluate(b).Findings {
		if f.State == "IngressBackendMissing" || f.State == "GoverningServiceMissing" {
			t.Errorf("with a query Services are narrowed, so %s is a guess: %+v", f.State, f)
		}
	}
}

func TestRulesIgnoreRecordedAndOldEvents(t *testing.T) {
	now := time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC)
	ev := func(reason string, at time.Time, recorded bool) model.EventRecord {
		return model.EventRecord{Reason: reason, Message: "x", Recorded: recorded, LastSeen: model.TimestampFrom(at),
			InvolvedObject: model.ObjectRef{Kind: "CronJob", Name: "hello"}}
	}
	cron := with(with(meta("CronJob", "hello"), "spec.schedule", "* * * * *"), "status.lastSuccessfulTime", "2026-11-05T20:59:00Z")
	for name, e := range map[string]model.EventRecord{
		"recorded": ev("FailedCreate", now.Add(-5*time.Minute), true),
		"old":      ev("FailedCreate", now.Add(-3*time.Hour), false),
	} {
		b := model.EvidenceBundle{CollectedAt: model.TimestampFrom(now), Events: []model.EventRecord{e},
			Resources: map[string][]map[string]any{"cronjobs": {cron}}}
		for _, f := range DefaultRegistry().Evaluate(b).Findings {
			if f.State == "CronJobMissedSchedule" {
				t.Errorf("%s event raised %+v", name, f)
			}
		}
	}
	live := model.EvidenceBundle{CollectedAt: model.TimestampFrom(now), Events: []model.EventRecord{ev("FailedCreate", now.Add(-5*time.Minute), false)},
		Resources: map[string][]map[string]any{"cronjobs": {cron}}}
	found := false
	for _, f := range DefaultRegistry().Evaluate(live).Findings {
		found = found || f.State == "CronJobMissedSchedule"
	}
	if !found {
		t.Error("a live recent FailedCreate was ignored")
	}
}
