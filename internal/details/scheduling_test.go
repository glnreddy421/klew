package details

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSchedulingSummaryPairs(t *testing.T) {
	spec := corev1.PodSpec{
		NodeSelector: map[string]string{"disktype": "ssd"},
		Tolerations: []corev1.Toleration{{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}},
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
					TopologyKey: "kubernetes.io/hostname",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "web"},
					},
				}},
			},
		},
	}
	pairs := schedulingSummaryPairs(spec)
	if len(pairs) != 6 {
		t.Fatalf("pairs = %v", pairs)
	}
}

func TestSchedulingSectionsIncludesTolerationsTable(t *testing.T) {
	spec := corev1.PodSpec{
		Tolerations: []corev1.Toleration{{
			Key:      "dedicated",
			Operator: corev1.TolerationOpEqual,
			Value:    "gpu",
			Effect:   corev1.TaintEffectNoSchedule,
		}},
	}
	sections := schedulingSections(spec)
	if len(sections) != 1 || sections[0].ID != "tolerations" {
		t.Fatalf("sections = %+v", sections)
	}
	if len(sections[0].Table.Rows) != 1 {
		t.Fatalf("rows = %v", sections[0].Table.Rows)
	}
}

func TestSchedulingSectionsNodeSelectorTable(t *testing.T) {
	spec := corev1.PodSpec{
		NodeSelector: map[string]string{"disktype": "ssd"},
	}
	sections := schedulingSections(spec)
	if len(sections) != 1 || sections[0].ID != "nodeSelector" {
		t.Fatalf("sections = %+v", sections)
	}
}

func TestTolerationsSummaryString(t *testing.T) {
	got := tolerationsSummaryString([]corev1.Toleration{{
		Key:               "node.kubernetes.io/not-ready",
		Operator:          corev1.TolerationOpExists,
		Effect:            corev1.TaintEffectNoExecute,
		TolerationSeconds: int64Ptr(300),
	}})
	if !strings.Contains(got, "not-ready") || !strings.Contains(got, "300s") {
		t.Fatalf("summary = %q", got)
	}
}

func int64Ptr(v int64) *int64 { return &v }
