package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// notifierBundle is a Ready pod owned by Deployment/notifier with the given log lines.
func notifierBundle(ready bool, lines ...string) model.EvidenceBundle {
	pod := ownedPod("notifier-1", "ReplicaSet", "notifier-aaa", ready, model.ContainerStatus{Name: "notifier", State: "running"})
	return model.EvidenceBundle{
		ReplicaSets: []model.ReplicaSetSummary{{Name: "notifier-aaa", DeploymentOwner: "notifier"}},
		Pods:        []model.PodSummary{pod},
		Logs:        []model.LogRecord{{PodName: "notifier-1", ContainerName: "notifier", Lines: lines}},
	}
}

func repeat(n int, format string) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf(format, i))
	}
	return out
}

func interleave(a, b []string) []string {
	var out []string
	for i := range max(len(a), len(b)) {
		if i < len(a) {
			out = append(out, a[i])
		}
		if i < len(b) {
			out = append(out, b[i])
		}
	}
	return out
}

func TestReadyPodLoggingTheSameErrorIsAProblem(t *testing.T) {
	errs := repeat(20, "2026-11-25T17:28:%02d+00:00 ERROR failed to deliver audit record to http://auditlog.holdout.svc.cluster.local/: wget: bad address")
	warns := repeat(20, "2026-11-25T17:28:%02d+00:00 WARN queued notification for retry, backlog growing")
	p, ok := problemsFor(notifierBundle(true, interleave(errs, warns)...))["Deployment/notifier"]
	if !ok {
		t.Fatal("a Ready pod logging only errors is not reported")
	}
	if p.State != "ErrorsWhileReady" || p.Severity != model.SeverityHigh {
		t.Errorf("problem = %+v, want ErrorsWhileReady, high (half the lines are errors)", p)
	}
	wantFacts(t, p, "container notifier is Ready but 20 of its last 40 log lines are errors",
		"the same error repeats 20 times", model.LogExcerptPrefix+"2026-11-25T17:28:19+00:00 ERROR failed to deliver audit record")
}

func TestErrorsWhileReadyIsAWarningBelowHalf(t *testing.T) {
	lines := append(repeat(30, "%d INFO handled request"), repeat(10, "%d ERROR upstream refused connection")...)
	p := problemsFor(notifierBundle(true, lines...))["Deployment/notifier"]
	if p.State != "ErrorsWhileReady" || p.Severity != model.SeverityWarning {
		t.Errorf("problem = %+v, want ErrorsWhileReady, warning", p)
	}
}

func TestErrorsWhileReadyIgnoresNoise(t *testing.T) {
	cases := map[string][]string{
		"an occasional error":       append(repeat(50, "%d INFO ok"), repeat(4, "%d ERROR blip")...),
		"a small share of errors":   append(repeat(80, "%d INFO ok"), repeat(10, "%d ERROR blip")...),
		"recovered since":           append(repeat(20, "%d ERROR db down"), repeat(15, "%d INFO connected, serving")...),
		"info that mentions errors": repeat(40, "2026-11-25T17:10:%02d+00:00 INFO flushed batch events=0 errors=0"),
	}
	for name, lines := range cases {
		if p, ok := problemsFor(notifierBundle(true, lines...))["Deployment/notifier"]; ok {
			t.Errorf("%s: reported %+v", name, p)
		}
	}
}

func TestErrorsWhileReadySkipsPodsAlreadyFailing(t *testing.T) {
	b := notifierBundle(false, repeat(20, "%d ERROR crash")...)
	b.Pods[0].Containers[0].State, b.Pods[0].Containers[0].Reason = "waiting", "CrashLoopBackOff"
	b.Pods[0].Containers[0].RestartCount = 5
	res := DefaultRegistry().Evaluate(b)
	for _, f := range res.Findings {
		if f.State == "ErrorsWhileReady" {
			t.Errorf("unready pod also reported as ErrorsWhileReady: %+v", f)
		}
	}
	if !strings.Contains(fmt.Sprint(BuildProblems(b, res.Findings)), "CrashLoopBackOff") {
		t.Error("the crash loop itself is still reported")
	}
}

func TestErrorsWhileReadyIgnoresPreviousContainerLogs(t *testing.T) {
	b := notifierBundle(true)
	b.Logs = []model.LogRecord{{PodName: "notifier-1", ContainerName: "notifier", Previous: true, Lines: repeat(20, "%d ERROR before restart")}}
	if p, ok := problemsFor(b)["Deployment/notifier"]; ok {
		t.Errorf("logs of the previous instance reported: %+v", p)
	}
}
