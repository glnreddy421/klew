package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func TestParseInvestigate(t *testing.T) {
	var errOut bytes.Buffer
	o, code := parseInvestigate([]string{"checkout", "-n", "shop", "-only", "failures,Patterns", "-observe", "30s"}, &errOut)
	if code != -1 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if o.namespace != "shop" || o.workload != "checkout" || o.observe != 30*time.Second || o.limit != defaultLimit {
		t.Errorf("parsed %+v", o)
	}
	if !o.show["failures"] || !o.show["patterns"] || o.show["overview"] || o.show["evidence"] {
		t.Errorf("-only gave %v", o.show)
	}

	o, _ = parseInvestigate([]string{"-all", "-no-history"}, &errOut)
	if o.limit != 0 || o.historyDir != "" || !o.show["overview"] {
		t.Errorf("-all -no-history gave %+v", o)
	}

	dir := t.TempDir()
	o, code = parseInvestigate([]string{"-extensions-dir", dir}, &errOut)
	if code != -1 || o.extensionsDir != dir {
		t.Errorf("-extensions-dir %s gave exit %d, %+v", dir, code, o)
	}

	for _, args := range [][]string{
		{"a", "b"}, {"-o", "yaml"}, {"-only", "graph"}, {"-observe", "1h"}, {"-no-such-flag"},
		{"-extensions-dir", filepath.Join(dir, "missing")},
	} {
		errOut.Reset()
		if _, code := parseInvestigate(args, &errOut); code != ExitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, ExitUsage)
		}
	}
	errOut.Reset()
	if _, code := parseInvestigate([]string{"-h"}, &errOut); code != 0 || !strings.Contains(errOut.String(), "Exit status:") {
		t.Errorf("-h: exit %d\n%s", code, errOut.String())
	}
}

func TestExitFor(t *testing.T) {
	for s, want := range map[model.VerdictStatus]int{
		model.VerdictHealthy: ExitHealthy, model.VerdictUnknown: ExitHealthy,
		model.VerdictWarning: ExitWarning, model.VerdictCritical: ExitCritical,
	} {
		if got := exitFor(s); got != want {
			t.Errorf("%s: exit %d, want %d", s, got, want)
		}
	}
}

func crashingState() model.InvestigationState {
	pod := func(name string, ready bool, restarts int32, reason string) model.PodSummary {
		c := model.ContainerStatus{Name: "app", Image: "shop/api:2", Ready: ready, RestartCount: restarts, State: "running"}
		if reason != "" {
			c.State, c.Reason, c.LastReason, c.LastExitCode = "waiting", reason, "Error", 1
		}
		return model.PodSummary{Name: name, Namespace: "shop", Node: "node-1", Phase: "Running", Ready: ready, RestartCount: restarts, Containers: []model.ContainerStatus{c}}
	}
	return model.InvestigationState{
		CollectedAt: "2026-11-07T10:00:00Z",
		Query:       "api",
		Verdict: model.Verdict{
			Status: model.VerdictCritical, LeadingSignal: "CrashLoopBackOff", Confidence: 0.82,
			Summary:               "api containers are crash-looping after a new revision.",
			StrongSignals:         []model.Signal{{Label: "CrashLoopBackOff", Strength: "strong", Count: 7, Evidence: "7 restarts in 5m"}},
			WeakSignals:           []model.Signal{{Label: "Probe failures", Strength: "weak"}},
			RecommendedNextChecks: []string{"kubectl logs deploy/api --previous"},
			MissingDataWarnings:   []string{"metrics-server not installed"},
		},
		Problems:    []model.Problem{{Object: "Deployment/api", Severity: model.SeverityCritical, State: "CrashLoopBackOff", Facts: []string{"2 of 3 pods unready"}}},
		CausalChain: []string{"Revision 7 rolled out", "Containers exit 1 at start", "Pods enter CrashLoopBackOff"},
		Snapshot: model.EvidenceBundle{
			Namespace: "shop",
			Pods:      []model.PodSummary{pod("api-1", true, 0, ""), pod("api-2", false, 7, "CrashLoopBackOff"), pod("api-3", false, 3, "CrashLoopBackOff")},
			Events: []model.EventRecord{
				{Timestamp: "2026-11-07T09:59:00Z", Type: "Warning", Reason: "BackOff", Message: "Back-off restarting failed container", InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "api-2"}},
				{Timestamp: "2026-11-07T09:58:00Z", Type: "Normal", Reason: "Pulled", Message: "Pulled image", InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "api-1"}},
			},
			Coverage: []model.CollectorStatus{{Collector: "ingresses", Status: "no-permission", Detail: "forbidden"}, {Collector: "pods", Status: "collected"}},
		},
		LogPatterns: &model.LogPatterns{
			Templates: []model.LogTemplate{{Template: "dial tcp <*>:5432: connect: connection refused", Count: 42, Pct: 61, Trend: "↑", Pods: []string{"api-2", "api-3"},
				Samples: []model.LogSample{{Message: "dial tcp 10.0.0.5:5432: connect: connection refused"}}}},
			EventTemplates: []model.LogTemplate{{Template: "Back-off restarting failed container", Count: 9, Pct: 100}},
			EvidenceBoard: &model.EvidenceBoardPayload{Cards: []model.EvidenceCard{{
				Confidence: 0.9, RootEvent: model.LogTemplate{Template: "Back-off restarting failed container"},
				TriggeredLogs: []model.LogTemplate{{Template: "dial tcp <*>:5432: connect: connection refused"}},
			}}},
		},
		LiveEvidence: []model.EvidenceEvent{
			{Timestamp: "2026-11-07T09:57:00Z", SourceType: model.SourceObjectChange, SourceKind: "Deployment", SourceName: "api", Reason: "ImageChanged", Message: "shop/api:1 → shop/api:2"},
			{Timestamp: "2026-11-07T09:59:30Z", SourceType: model.SourceLog, Message: "raw log line"},
		},
	}
}

func TestReportFromState(t *testing.T) {
	r := buildReport(crashingState(), 10*time.Second, defaultLimit)
	if r.Pods.Ready != 1 || r.Pods.Unready != 2 {
		t.Errorf("pod counts %+v", r.Pods)
	}
	f := r.Failures
	if f.PodsTotal != 3 || f.FailingTotal != 2 || f.Pods[0].Name != "api-2" || f.Pods[0].Reason != "CrashLoopBackOff" {
		t.Errorf("failures: %+v", f)
	}
	if len(f.Pods[0].Events) != 1 || f.Pods[0].Events[0].Reason != "BackOff" || !f.Pods[0].Events[0].Warning {
		t.Errorf("api-2 events: %+v", f.Pods[0].Events)
	}
	if r.Patterns.LogsTotal != 1 || r.Patterns.Logs[0].Sample == "" {
		t.Errorf("patterns: %+v", r.Patterns)
	}
	e := r.Evidence
	if len(e.Correlated) != 1 || e.SignalsTotal != 2 || e.ObservationsTotal != 1 || e.Observations[0].Kind != "Change" {
		t.Errorf("evidence: %+v", e)
	}
	if len(e.NotCollected) != 1 || e.NotCollected[0].Collector != "ingresses" {
		t.Errorf("not collected: %+v", e.NotCollected)
	}
	if len(e.NextChecks) != 1 {
		t.Errorf("next checks fall back to the verdict's: %v", e.NextChecks)
	}

	if r := buildReport(crashingState(), 0, 1); len(r.Failures.Pods) != 1 || r.Failures.FailingTotal != 2 || len(r.Evidence.Signals) != 1 {
		t.Errorf("limit 1: %d pods of %d, %d signals", len(r.Failures.Pods), r.Failures.FailingTotal, len(r.Evidence.Signals))
	}
}

func TestTextShowsEveryView(t *testing.T) {
	r := buildReport(crashingState(), 10*time.Second, defaultLimit)
	all := map[string]bool{"overview": true, "failures": true, "patterns": true, "evidence": true}
	var out bytes.Buffer
	writeText(&out, r, all, false)
	text := out.String()
	for _, want := range []string{
		"shop / api", "CRITICAL  CrashLoopBackOff", "confidence 82%",
		"OVERVIEW", "Deployment/api", "3. Pods enter CrashLoopBackOff",
		"FAILURES", "2 of 3 pods failing", "api-2", "container app: waiting (CrashLoopBackOff) · last Error exit 1 · 7 restarts",
		"Back-off restarting failed container",
		"PATTERNS", "↑    42×  61%  dial tcp <*>:5432", "e.g. dial tcp 10.0.0.5:5432",
		"EVIDENCE", "90%  Back-off restarting", "strong CrashLoopBackOff ×7", "Deployment/api ImageChanged",
		"ingresses: no-permission — forbidden", "1. kubectl logs deploy/api --previous",
		"metrics-server not installed",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Error("plain output has ANSI codes")
	}
	if strings.Contains(text, "raw log line") {
		t.Error("log evidence belongs in Patterns, not Observations")
	}

	out.Reset()
	writeText(&out, r, map[string]bool{"failures": true}, true)
	if strings.Contains(out.String(), "PATTERNS") || !strings.Contains(out.String(), "\x1b[") {
		t.Errorf("-only failures with color:\n%s", out.String())
	}
}

func TestReportJSON(t *testing.T) {
	b, err := json.Marshal(buildReport(crashingState(), 10*time.Second, defaultLimit))
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"status", "overview", "failures", "patterns", "evidence"} {
		if _, ok := back[k]; !ok {
			t.Errorf("JSON lacks %q", k)
		}
	}
}

func TestMergeIncidents(t *testing.T) {
	rs := []model.ObjectRef{{Kind: "ReplicaSet", Name: "reconciler-7ddbd8966"}}
	inc := func(start, end string, reason string, objs []model.ObjectRef) model.Incident {
		return model.Incident{Start: model.Timestamp(start), End: model.Timestamp(end), LeadingReason: reason, Count: 1, Objects: objs, Recorded: true}
	}
	got := mergeIncidents([]model.Incident{
		inc("2026-11-08T06:28:00Z", "2026-11-08T06:28:00Z", "FailedCreate", rs),
		inc("2026-11-08T06:44:40Z", "2026-11-08T06:44:40Z", "FailedCreate", rs),
		inc("2026-11-08T07:01:20Z", "2026-11-08T07:01:20Z", "FailedCreate", rs),
		inc("2026-11-08T09:00:00Z", "2026-11-08T09:05:00Z", "BackOff", []model.ObjectRef{{Kind: "Pod", Name: "api-2"}}),
		inc("2026-11-08T10:00:00Z", "2026-11-08T10:00:00Z", "FailedCreate", rs),
	})
	if len(got) != 3 {
		t.Fatalf("merged into %d incidents, want 3: %+v", len(got), got)
	}
	if m := got[0]; m.Start != "2026-11-08T06:28:00Z" || m.End != "2026-11-08T07:01:20Z" || m.Count != 3 {
		t.Errorf("first incident %+v", m)
	}
	if l := latest(got, 2); len(l) != 2 || l[1].LeadingReason != "FailedCreate" || l[0].LeadingReason != "BackOff" {
		t.Errorf("latest 2: %+v", l)
	}
}

func TestLoadRegistryFromExtensionsDir(t *testing.T) {
	dir := t.TempDir()
	rule := `apiVersion: klew.dev/v1alpha1
kind: Rule
metadata: {name: single-replica}
spec:
  resource: workloads
  when: "object.replicas == 1"
  state: SingleReplica
`
	if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), []byte(rule), 0o600); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	reg := loadRegistry(dir, &errOut)
	found := false
	for _, r := range reg.Rules {
		if r.Name() == "single-replica" {
			found = true
		}
	}
	if !found || errOut.Len() > 0 {
		t.Errorf("rule from -extensions-dir not loaded; warnings: %s", errOut.String())
	}
}
