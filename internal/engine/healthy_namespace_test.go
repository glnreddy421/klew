package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

const redisStartupWarning = "1:C 27 Oct 2026 09:00:00.000 # Warning: no config file specified, using the default config. In order to specify a config file use redis-server /path/to/redis.conf"

func TestClassifyLogReasonRedisStartupWarningIsNotTimeout(t *testing.T) {
	if got := ClassifyLogReason(redisStartupWarning); got == "Redis timeout" {
		t.Fatalf("redis startup warning classified as %q", got)
	}
	for _, line := range []string{
		"redis: connection timeout after 5s",
		"dial tcp 10.0.0.7:6379: connect: connection timed out",
		"Error: connect ETIMEDOUT 10.0.0.7:6379",
	} {
		if got := ClassifyLogReason(line); got != "Redis timeout" {
			t.Errorf("ClassifyLogReason(%q)=%q want Redis timeout", line, got)
		}
		if got := ClassifyLogSeverity(line); got == model.SeverityInfo {
			t.Errorf("ClassifyLogSeverity(%q)=info, want at least warning", line)
		}
	}
}

func TestCausalChainOmitsRedisTimeoutForStartupWarning(t *testing.T) {
	st := model.NewInvestigationState("klew-lab", model.ModeLive)
	r := NewReducer(&st)
	now := time.Now().UTC()
	r.ApplyEvent(model.EvidenceEvent{
		Timestamp: model.TimestampFrom(now.Add(-5 * time.Hour)), SourceType: model.SourceK8sEvent,
		SourceKind: "Pod", SourceName: "payment-api-84bd756749-j4tsh", Namespace: "klew-lab",
		Severity: model.SeverityWarning, Reason: "Unhealthy", Message: "Readiness probe failed",
	})
	r.ApplyEvent(model.EvidenceEvent{
		Timestamp: model.TimestampFrom(now), SourceType: model.SourceLog,
		SourceKind: "Pod", SourceName: "redis-7d9f8c6b5-x2k4p", Namespace: "klew-lab",
		Pod: "redis-7d9f8c6b5-x2k4p", Container: "redis",
		Severity: ClassifyLogSeverity(redisStartupWarning), Reason: ClassifyLogReason(redisStartupWarning),
		Message: redisStartupWarning,
	})
	for _, step := range r.State().CausalChain {
		if strings.Contains(step, "Redis timeout") {
			t.Fatalf("causal chain has Redis timeout from a startup warning: %v", r.State().CausalChain)
		}
	}
}

func healthyPod(name string) model.PodSummary {
	return model.PodSummary{
		Name: name, Namespace: "klew-lab", Phase: "Running", Ready: true,
		Containers: []model.ContainerStatus{{Name: "app", Ready: true, State: "running"}},
	}
}

func TestNextChecksDoNotNameHealthyPodAsFailing(t *testing.T) {
	b := model.EvidenceBundle{
		Namespace: "klew-lab",
		Workloads: []model.WorkloadSummary{{Kind: "Deployment", Name: "payment-api", Selector: "app=payment-api"}},
		Pods:      []model.PodSummary{healthyPod("payment-api-84bd756749-j4tsh"), healthyPod("redis-7d9f8c6b5-x2k4p")},
	}
	if pod, container := failingPod(b); pod != "" || container != "" {
		t.Fatalf("failingPod picked healthy %s/%s", pod, container)
	}
	for _, cat := range []string{"", "Unknown", "CrashLoopBackOff", "ImagePullBackOff", "Mount/config"} {
		for _, c := range investigationStepsFor(cat, b) {
			if strings.Contains(c, "payment-api-84bd756749-j4tsh") || strings.Contains(c, "redis-7d9f8c6b5-x2k4p") {
				t.Fatalf("category %q next check names a healthy pod: %q", cat, c)
			}
		}
	}
}

func TestFailingPodPrefersUnreadyOrRestartingPods(t *testing.T) {
	restarted := healthyPod("payment-api-restarted")
	restarted.RestartCount = 2
	restarted.Containers[0].RestartCount = 2
	unready := healthyPod("payment-api-unready")
	unready.Ready = false
	unready.Containers[0].Ready = false

	b := model.EvidenceBundle{Pods: []model.PodSummary{healthyPod("payment-api-ok"), restarted}}
	if pod, _ := failingPod(b); pod != "payment-api-restarted" {
		t.Fatalf("failingPod=%q want payment-api-restarted", pod)
	}
	b = model.EvidenceBundle{Pods: []model.PodSummary{healthyPod("payment-api-ok"), unready}}
	if pod, _ := failingPod(b); pod != "payment-api-unready" {
		t.Fatalf("failingPod=%q want payment-api-unready", pod)
	}
}
