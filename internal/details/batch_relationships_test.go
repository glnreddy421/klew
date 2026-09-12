package details

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodReadyLabel(t *testing.T) {
	pod := corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
	if got := podReadyLabel(pod); got != "True" {
		t.Fatalf("ready = %q", got)
	}
}

func TestOwnerRefsRelationshipSection(t *testing.T) {
	sec := ownerRefsRelationshipSection([]metav1.OwnerReference{{
		Kind: "CronJob",
		Name: "nightly",
		UID:  "uid-1",
	}}, "default")
	if sec.Empty() {
		t.Fatal("expected owner section")
	}
	if sec.Table == nil || len(sec.Table.Rows) != 1 {
		t.Fatalf("rows = %+v", sec.Table)
	}
	if sec.Table.Rows[0][0] != "CronJob" || sec.Table.Rows[0][1] != "nightly" {
		t.Fatalf("row = %+v", sec.Table.Rows[0])
	}
}

func TestJobsForCronJobRequiresClient(t *testing.T) {
	if rows := jobsForCronJob(t.Context(), &Request{}, "nightly"); rows != nil {
		t.Fatalf("expected nil without client, got %v", rows)
	}
}

func TestPodsForJobRequiresClient(t *testing.T) {
	if rows := podsForJob(t.Context(), &Request{}, "hello"); rows != nil {
		t.Fatalf("expected nil without client, got %v", rows)
	}
}

func TestPodSummaryFields(t *testing.T) {
	if got := podSummaryFields(nil); got != nil {
		t.Fatalf("nil rows = %v", got)
	}
	if got := podSummaryFields([][]string{{"hello-world-abc", "Succeeded", "True"}}); len(got) != 2 || got[0] != "Pod" || got[1] != "hello-world-abc" {
		t.Fatalf("single pod = %v", got)
	}
	if got := podSummaryFields([][]string{
		{"job-a", "Succeeded", "True"},
		{"job-b", "Failed", "False"},
	}); len(got) != 2 || got[0] != "Pods" || got[1] != "job-a, job-b" {
		t.Fatalf("multiple pods = %v", got)
	}
}

func TestOwnerRefsRelationshipSectionEmpty(t *testing.T) {
	sec := ownerRefsRelationshipSection(nil, "default")
	if !sec.Empty() {
		t.Fatal("expected empty section")
	}
}

