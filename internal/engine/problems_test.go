package engine

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func ownedPod(name, ownerKind, owner string, ready bool, cs ...model.ContainerStatus) model.PodSummary {
	p := model.PodSummary{
		Name: name, Namespace: "ns", Phase: "Running", Ready: ready,
		OwnerRefs: []model.ObjectRef{{Kind: ownerKind, Name: owner}},
	}
	for _, c := range cs {
		if !c.Init {
			c.Ready = ready
		}
		p.RestartCount += c.RestartCount
		p.Containers = append(p.Containers, c)
	}
	return p
}

func problemsFor(b model.EvidenceBundle) map[string]model.Problem {
	res := DefaultRegistry().Evaluate(b)
	out := map[string]model.Problem{}
	for _, p := range BuildProblems(b, res.Findings) {
		out[p.Object] = p
	}
	return out
}

func wantFacts(t *testing.T, p model.Problem, want ...string) {
	t.Helper()
	facts := strings.Join(p.Facts, "; ")
	for _, w := range want {
		if !strings.Contains(facts, w) {
			t.Errorf("%s facts = %q, want %q", p.Object, facts, w)
		}
	}
}

var endpointsReadable = []model.PermissionCheck{{Resource: "endpointslices", Verb: "list", Allowed: true}}

func TestProblemsGroupPodsByWorkloadAndSkipHealthyOnes(t *testing.T) {
	queue := ownedPod("queue-1", "ReplicaSet", "queue-aaa", false, model.ContainerStatus{Name: "queue", State: "waiting",
		Reason: "CrashLoopBackOff", RestartCount: 9, LastReason: "OOMKilled", LastExitCode: 137, LimitsMem: "64Mi", RequestsMem: "32Mi"})
	queue2 := ownedPod("queue-2", "ReplicaSet", "queue-aaa", false, model.ContainerStatus{Name: "queue", State: "waiting", Reason: "CrashLoopBackOff", RestartCount: 8})
	mailer := ownedPod("mailer-1", "ReplicaSet", "mailer-aaa", true, model.ContainerStatus{Name: "mailer", State: "running"})
	b := model.EvidenceBundle{
		ReplicaSets: []model.ReplicaSetSummary{{Name: "queue-aaa", DeploymentOwner: "queue"}, {Name: "mailer-aaa", DeploymentOwner: "mailer"}},
		Pods:        []model.PodSummary{queue, queue2, mailer},
	}
	got := problemsFor(b)
	if len(got) != 1 {
		t.Fatalf("problems = %+v, want only Deployment/queue", got)
	}
	q := got["Deployment/queue"]
	if q.Pods != 2 || q.Unready != 2 || q.Restarts != 17 || q.State != "CrashLoopBackOff" || q.Severity != model.SeverityCritical {
		t.Fatalf("queue problem = %+v", q)
	}
	wantFacts(t, q, "last exited 137 (OOMKilled)", "memory limit 64Mi")
}

func TestProblemsReportProbePortsRequestsAndMounts(t *testing.T) {
	frontend := ownedPod("frontend-1", "ReplicaSet", "frontend-aaa", false, model.ContainerStatus{Name: "nginx", State: "running",
		ReadinessProbe: "http-get :9090/", Ports: []string{"80/TCP"}})
	batch := ownedPod("batch-1", "ReplicaSet", "batch-aaa", false, model.ContainerStatus{Name: "batch", RequestsCPU: "64"})
	batch.Phase = "Pending"
	batch.Unschedulable = "Unschedulable: 0/3 nodes are available: 3 Insufficient cpu."
	store := ownedPod("store-0", "StatefulSet", "store", false, model.ContainerStatus{Name: "store", State: "waiting",
		Reason: "CrashLoopBackOff", RestartCount: 3, LastReason: "Error", LastExitCode: 2})
	b := model.EvidenceBundle{
		ReplicaSets: []model.ReplicaSetSummary{{Name: "frontend-aaa", DeploymentOwner: "frontend"}, {Name: "batch-aaa", DeploymentOwner: "batch"}},
		Pods:        []model.PodSummary{frontend, batch, store},
	}
	got := problemsFor(b)
	wantFacts(t, got["Deployment/frontend"], "readiness probe http-get :9090/", "declared ports 80/TCP")
	wantFacts(t, got["Deployment/batch"], "Insufficient cpu", "requests cpu 64")
	wantFacts(t, got["StatefulSet/store"], "last exited 2 (Error)", "no volume mounts")
	if s := got["Deployment/batch"].State; s != "Unschedulable" {
		t.Errorf("batch state = %q", s)
	}
}

func TestProblemsInitContainerFailure(t *testing.T) {
	p := ownedPod("api-1", "ReplicaSet", "api-aaa", false,
		model.ContainerStatus{Name: "wait-db", Init: true, State: "waiting", Reason: "CrashLoopBackOff", RestartCount: 12, LastReason: "Error", LastExitCode: 1},
		model.ContainerStatus{Name: "api", State: "waiting", Reason: "PodInitializing"})
	p.Phase = "Pending"
	b := model.EvidenceBundle{ReplicaSets: []model.ReplicaSetSummary{{Name: "api-aaa", DeploymentOwner: "api"}}, Pods: []model.PodSummary{p}}
	got := problemsFor(b)["Deployment/api"]
	if got.State != "Init:CrashLoopBackOff" {
		t.Fatalf("state = %q, want Init:CrashLoopBackOff", got.State)
	}
	wantFacts(t, got, "container wait-db (init) last exited 1 (Error)")
}

func TestProblemsLivenessKill(t *testing.T) {
	p := ownedPod("proxy-1", "ReplicaSet", "proxy-aaa", false, model.ContainerStatus{Name: "proxy", State: "waiting",
		Reason: "CrashLoopBackOff", RestartCount: 30, LastReason: "Completed", LivenessProbe: "http-get :80/health"})
	b := model.EvidenceBundle{
		ReplicaSets: []model.ReplicaSetSummary{{Name: "proxy-aaa", DeploymentOwner: "proxy"}},
		Pods:        []model.PodSummary{p},
		Events: []model.EventRecord{{Reason: "Unhealthy", InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "proxy-1"},
			Message: "Liveness probe failed: HTTP probe failed with statuscode: 404"}},
	}
	wantFacts(t, problemsFor(b)["Deployment/proxy"], "restarted by its liveness probe http-get :80/health: Liveness probe failed: HTTP probe failed with statuscode: 404")
}

func TestProblemsPodCreationRejectedWithoutPods(t *testing.T) {
	b := model.EvidenceBundle{ReplicaSets: []model.ReplicaSetSummary{{Name: "worker-aaa", DeploymentOwner: "worker",
		CreateFailure: `pods "worker-aaa-x" is forbidden: exceeded quota: compute, requested: cpu=2, limited: cpu=1`}}}
	got := problemsFor(b)["Deployment/worker"]
	if got.State != "PodCreationRejected" || got.Pods != 0 {
		t.Fatalf("worker problem = %+v", got)
	}
	wantFacts(t, got, "exceeded quota")
}

func TestProblemsFailedJobGroupsItsPods(t *testing.T) {
	pod := ownedPod("nightly-28-abc", "Job", "nightly-28", false, model.ContainerStatus{Name: "run", State: "terminated", Reason: "Error", ExitCode: 1})
	pod.Phase = "Failed"
	ok := ownedPod("cleanup-1", "Job", "cleanup", false, model.ContainerStatus{Name: "run", State: "terminated", Reason: "Completed"})
	ok.Phase = "Succeeded"
	b := model.EvidenceBundle{
		Jobs: []model.JobSummary{
			{Name: "nightly-28", CronJob: "nightly", Failed: 3, FailedReason: "BackoffLimitExceeded", FailedMessage: "Job has reached the specified backoff limit"},
			{Name: "cleanup", Succeeded: 1, Complete: true},
		},
		Pods: []model.PodSummary{pod, ok},
	}
	got := problemsFor(b)
	if len(got) != 1 {
		t.Fatalf("problems = %+v, want only CronJob/nightly", got)
	}
	n := got["CronJob/nightly"]
	if n.State != "BackoffLimitExceeded" || n.Pods != 1 {
		t.Fatalf("nightly problem = %+v", n)
	}
	wantFacts(t, n, "backoff limit", "3 failed pods", "container run exited 1 (Error)")
}

func TestProblemsServiceSelectorMatchesNoPods(t *testing.T) {
	b := model.EvidenceBundle{
		Permissions: endpointsReadable,
		Services:    []model.ServiceSummary{{Name: "edge", Selector: "app=edge,tier=public"}},
		Pods: []model.PodSummary{
			{Name: "edge-1", Phase: "Running", Ready: true, Labels: map[string]string{"app": "edge", "tier": "internal"}},
			{Name: "other-1", Phase: "Running", Ready: true, Labels: map[string]string{"app": "other"}},
		},
	}
	got := problemsFor(b)["Service/edge"]
	if got.State != "SelectorMatchesNoPods" {
		t.Fatalf("edge problem = %+v", got)
	}
	wantFacts(t, got, "selector app=edge,tier=public matches no pods", "closest pod edge-1 has tier=internal, not tier=public")
}

func TestServiceSelectorRuleNeedsEndpointAccess(t *testing.T) {
	b := model.EvidenceBundle{Services: []model.ServiceSummary{{Name: "edge", Selector: "app=edge"}}}
	if got := problemsFor(b); len(got) != 0 {
		t.Fatalf("problems without endpointslices access = %+v, want none (0 endpoints is unknown, not empty)", got)
	}
}

func TestObjectFindingsKeepVerdictFromCallingHealthy(t *testing.T) {
	b := model.EvidenceBundle{
		Permissions: endpointsReadable,
		Workloads:   []model.WorkloadSummary{{Kind: "Deployment", Name: "edge", Replicas: 1, Ready: 1, Available: 1}},
		Services:    []model.ServiceSummary{{Name: "edge", Selector: "app=edge,tier=public"}},
		Pods:        []model.PodSummary{healthyPod("edge-1")},
	}
	res := DefaultRegistry().Evaluate(b)
	corr := CorrelationEngine{Findings: res.Findings}.Correlate(b, nil, nil)
	if corr.HypothesisLabel == HealthyLabel {
		t.Fatalf("verdict healthy despite a Service selecting no pods: %+v", corr)
	}
}

func TestProblemsLivenessRestartsOnReadyPod(t *testing.T) {
	p := ownedPod("relay-1", "ReplicaSet", "relay-aaa", true, model.ContainerStatus{Name: "relay", State: "running",
		RestartCount: 4, LastReason: "Error", LivenessProbe: "tcp :7000"})
	quiet := ownedPod("relay-2", "ReplicaSet", "relay-aaa", true, model.ContainerStatus{Name: "relay", State: "running",
		RestartCount: 1, LivenessProbe: "tcp :7000"})
	b := model.EvidenceBundle{
		ReplicaSets: []model.ReplicaSetSummary{{Name: "relay-aaa", DeploymentOwner: "relay"}},
		Pods:        []model.PodSummary{p, quiet},
		Events: []model.EventRecord{{Reason: "Unhealthy", InvolvedObject: model.ObjectRef{Kind: "Pod", Name: "relay-1"},
			Message: "Liveness probe failed: dial tcp 10.0.0.1:7000: connect: connection refused"}},
	}
	got := problemsFor(b)["Deployment/relay"]
	if got.State != "LivenessRestarts" || got.Unready != 0 || got.Pods != 2 {
		t.Fatalf("relay problem = %+v", got)
	}
	wantFacts(t, got, "restarted 4 times by its liveness probe tcp :7000", "connection refused")
	if strings.Contains(strings.Join(got.Facts, " "), "relay-2") {
		t.Fatalf("pod without probe failures reported: %v", got.Facts)
	}
}

func TestProblemsHeadline(t *testing.T) {
	if got := ProblemsHeadline(nil); got != "" {
		t.Fatalf("headline for none = %q", got)
	}
	one := []model.Problem{{Object: "Deployment/api", Severity: model.SeverityCritical, State: "CrashLoopBackOff"}}
	if got, want := ProblemsHeadline(one), "1 problem: 1 critical. Worst: Deployment/api (CrashLoopBackOff)"; got != want {
		t.Fatalf("headline = %q, want %q", got, want)
	}
	many := []model.Problem{
		{Object: "Deployment/a", Severity: model.SeverityCritical, State: "A"},
		{Object: "Deployment/b", Severity: model.SeverityCritical, State: "B"},
		{Object: "Job/c", Severity: model.SeverityHigh, State: "C"},
		{Object: "Service/d", Severity: model.SeverityHigh, State: "D"},
		{Object: "NetworkPolicy/e", Severity: model.SeverityWarning, State: "E"},
	}
	want := "5 problems: 2 critical, 2 high, 1 warning. Worst: Deployment/a (A), Deployment/b (B), Job/c (C) and 2 more"
	if got := ProblemsHeadline(many); got != want {
		t.Fatalf("headline = %q, want %q", got, want)
	}
}

func TestGenericLeadingSignalNamesWorstProblem(t *testing.T) {
	problems := []model.Problem{{Object: "Deployment/billing", Severity: model.SeverityCritical, State: "Init:CrashLoopBackOff",
		Facts: []string{"container migrate (init) last exited 2 (Error)", "no volume mounts", "third"}}}
	corr := Correlation{LeadingSignal: "log signal", HypothesisLabel: "log signal"}
	leadWithWorstProblem(&corr, problems)
	if corr.LeadingSignal != "Init:CrashLoopBackOff" || corr.HypothesisLabel != "Init:CrashLoopBackOff on Deployment/billing" || len(corr.Reasons) != 2 {
		t.Fatalf("correlation = %+v", corr)
	}

	specific := Correlation{LeadingSignal: "OOMKilled", HypothesisLabel: "Memory limit exceeded"}
	leadWithWorstProblem(&specific, problems)
	if specific.LeadingSignal != "OOMKilled" {
		t.Fatalf("a specific leading signal was replaced: %+v", specific)
	}
	mild := Correlation{LeadingSignal: "Warning logs"}
	leadWithWorstProblem(&mild, []model.Problem{{Object: "NetworkPolicy/x", Severity: model.SeverityWarning, State: "DeniesEgress"}})
	if mild.LeadingSignal != "Warning logs" {
		t.Fatalf("a warning-level problem replaced the leading signal: %+v", mild)
	}
}
