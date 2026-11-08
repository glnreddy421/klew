package kube

import (
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestSummarizePendingPodKeepsSpecFacts(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "reports", Image: "busybox",
			Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("40")}},
		}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
				Reason: "Unschedulable", Message: "0/1 nodes are available: 1 Insufficient cpu.",
			}},
		},
	}
	ps := summarizePod(pod)
	if ps.Ready {
		t.Fatal("pending pod with no container statuses counted as ready")
	}
	if len(ps.Containers) != 1 || ps.Containers[0].RequestsCPU != "40" {
		t.Fatalf("containers = %+v, want reports requesting cpu 40", ps.Containers)
	}
	if want := "Unschedulable: 0/1 nodes are available: 1 Insufficient cpu."; ps.Unschedulable != want {
		t.Fatalf("Unschedulable = %q, want %q", ps.Unschedulable, want)
	}
}

func TestSummarizePodDescribesProbesPortsAndMounts(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name:  "nginx",
			Ports: []corev1.ContainerPort{{ContainerPort: 80, Protocol: corev1.ProtocolTCP}},
			ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Port: intstr.FromInt32(8080), Path: "/"},
			}},
			LivenessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString("http")},
			}},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: "/data"},
				{Name: "kube-api-access-abcde", MountPath: "/var/run/secrets/kubernetes.io/serviceaccount"},
			},
		}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
			Name: "nginx", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		}}},
	}
	c := summarizePod(pod).Containers[0]
	if c.ReadinessProbe != "http-get :8080/" || c.LivenessProbe != "tcp :http" {
		t.Fatalf("probes = %q, %q", c.ReadinessProbe, c.LivenessProbe)
	}
	if !slices.Equal(c.Ports, []string{"80/TCP"}) || !slices.Equal(c.Mounts, []string{"/data (data)"}) {
		t.Fatalf("ports = %v, mounts = %v", c.Ports, c.Mounts)
	}
}

func TestSummarizePodIncludesInitContainers(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "migrate"}, {Name: "seed"}},
			Containers:     []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "migrate", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"}}},
				{Name: "seed", RestartCount: 4, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}},
			},
		},
	}
	ps := summarizePod(pod)
	if len(ps.Containers) != 3 || !ps.Containers[0].Init || !ps.Containers[1].Init || ps.Containers[2].Init {
		t.Fatalf("containers = %+v, want two init containers then app", ps.Containers)
	}
	if !ps.Containers[0].Ready || ps.Containers[1].Ready {
		t.Fatalf("completed init container should be ready, crashing one not: %+v", ps.Containers[:2])
	}
	if ps.RestartCount != 4 || ps.Ready {
		t.Fatalf("restarts = %d ready = %v, want 4 and false", ps.RestartCount, ps.Ready)
	}
}

func TestSummarizeRSReportsPodCreationFailure(t *testing.T) {
	rs := appsv1.ReplicaSet{Status: appsv1.ReplicaSetStatus{Conditions: []appsv1.ReplicaSetCondition{{
		Type: appsv1.ReplicaSetReplicaFailure, Status: corev1.ConditionTrue, Reason: "FailedCreate",
		Message: `pods "w-1" is forbidden: maximum cpu usage per Container is 500m, but limit is 1`,
	}}}}
	if got := summarizeRS(rs).CreateFailure; got != rs.Status.Conditions[0].Message {
		t.Fatalf("CreateFailure = %q", got)
	}
}

func TestSummarizeJob(t *testing.T) {
	j := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "nightly-1", OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "nightly"}}},
		Status: batchv1.JobStatus{Failed: 3, Conditions: []batchv1.JobCondition{{
			Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded", Message: "Job has reached the specified backoff limit",
		}}},
	}
	got := summarizeJob(j)
	if got.CronJob != "nightly" || got.Failed != 3 || got.FailedReason != "BackoffLimitExceeded" || got.Complete {
		t.Fatalf("summary = %+v", got)
	}
}
