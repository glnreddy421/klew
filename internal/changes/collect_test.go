package changes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/internal/model"
)

var t0 = time.Date(2026, 10, 21, 18, 0, 0, 0, time.UTC)

func tpl(image string, env ...corev1.EnvVar) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "pay"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: image, Env: env}}},
	}
}

func deployment(uid types.UID) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "payment-api", Namespace: "klew-lab", UID: uid, Generation: 7},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "pay"}},
			Template: tpl("repo/pay:v3"),
		},
	}
}

func rs(name string, owner types.UID, rev int, created time.Time, t corev1.PodTemplateSpec, ann map[string]string) *appsv1.ReplicaSet {
	a := map[string]string{revisionAnnotation: fmt.Sprint(rev)}
	for k, v := range ann {
		a[k] = v
	}
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "klew-lab", UID: types.UID("rs-" + name), Annotations: a,
			Labels:            map[string]string{"app": "pay"},
			CreationTimestamp: metav1.NewTime(created),
			OwnerReferences:   []metav1.OwnerReference{{Kind: "Deployment", Name: "payment-api", UID: owner, Controller: ptr(true)}},
		},
		Spec: appsv1.ReplicaSetSpec{Template: t},
	}
}

func ptr[T any](v T) *T { return &v }

func TestDeploymentHistoryNewestFirstWithDiffs(t *testing.T) {
	d := deployment("dep-1")
	cs := fake.NewSimpleClientset(d,
		rs("pay-1", "dep-1", 1, t0, tpl("repo/pay:v1"), nil),
		rs("pay-2", "dep-1", 2, t0.Add(time.Hour), tpl("repo/pay:v2", corev1.EnvVar{Name: "DB_POOL_SIZE", Value: "20"}), nil),
		rs("pay-3", "dep-1", 3, t0.Add(2*time.Hour), tpl("repo/pay:v3", corev1.EnvVar{Name: "DB_POOL_SIZE", Value: "5"}),
			map[string]string{changeCauseAnnotation: "kubectl set image deploy/payment-api app=repo/pay:v3"}),
		rs("other", "someone-else", 9, t0, tpl("x:1"), nil),
	)
	wc, err := ForWorkload(context.Background(), cs, "Deployment", "klew-lab", "payment-api")
	if err != nil {
		t.Fatal(err)
	}
	if len(wc.Changes) != 3 {
		t.Fatalf("want 3 owned revisions, got %+v", wc.Changes)
	}
	newest := wc.Changes[0]
	if newest.Revision != 3 || !newest.Current || wc.CurrentRevision != 3 {
		t.Fatalf("newest = %+v (current %d)", newest, wc.CurrentRevision)
	}
	if newest.Summary != "image v2 → v3, env DB_POOL_SIZE 20 → 5" {
		t.Fatalf("summary = %q", newest.Summary)
	}
	if newest.ChangeCause == "" || newest.Source != model.ChangeSourceRollout {
		t.Fatalf("cause/source = %+v", newest)
	}
	if wc.Changes[2].Summary != "Oldest retained revision" {
		t.Fatalf("oldest = %+v", wc.Changes[2])
	}
	if wc.HistoryLimit != DefaultHistoryLimit || wc.Generation != 7 || wc.LastChangedAt != model.TimestampFrom(t0.Add(2*time.Hour)) {
		t.Fatalf("meta = %+v", wc)
	}
}

func TestDeploymentRollbackDetection(t *testing.T) {
	d := deployment("dep-1")
	rollbackAt := t0.Add(5 * time.Hour)
	reused := rs("pay-1", "dep-1", 4, t0, tpl("repo/pay:v1"), map[string]string{revisionHistoryAnnotation: "1,3"})
	reused.ManagedFields = []metav1.ManagedFieldsEntry{
		{Manager: "kube-controller-manager", Operation: metav1.ManagedFieldsOperationUpdate, Time: ptr(metav1.NewTime(rollbackAt))},
		{Manager: "kube-controller-manager", Operation: metav1.ManagedFieldsOperationUpdate, Subresource: "status", Time: ptr(metav1.NewTime(rollbackAt.Add(time.Minute)))},
	}
	cs := fake.NewSimpleClientset(d,
		rs("pay-2", "dep-1", 2, t0.Add(time.Hour), tpl("repo/pay:v2"), nil),
		reused,
	)
	wc, _ := ForWorkload(context.Background(), cs, "Deployment", "klew-lab", "payment-api")
	top := wc.Changes[0]
	if top.Source != model.ChangeSourceRollback || top.RollbackTo != 1 || top.Summary != "Rollback to rev 1 template" {
		t.Fatalf("rollback row = %+v", top)
	}
	if top.Time != model.TimestampFrom(rollbackAt) {
		t.Fatalf("rollback time should be the reuse, not the original creation: %s", top.Time)
	}
}

func TestActorOnlyWhenMatched(t *testing.T) {
	d := deployment("dep-1")
	d.ManagedFields = []metav1.ManagedFieldsEntry{{
		Manager: "kubectl-edit", Operation: metav1.ManagedFieldsOperationUpdate,
		Time:     ptr(metav1.NewTime(t0.Add(2*time.Hour + 10*time.Second))),
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:template":{"f:spec":{}}}}`)},
	}}
	cs := fake.NewSimpleClientset(d,
		rs("pay-1", "dep-1", 1, t0, tpl("repo/pay:v1"), nil),
		rs("pay-2", "dep-1", 2, t0.Add(2*time.Hour), tpl("repo/pay:v2"), nil),
	)
	wc, _ := ForWorkload(context.Background(), cs, "Deployment", "klew-lab", "payment-api")
	if wc.Changes[0].Actor != "kubectl-edit" {
		t.Fatalf("recent revision actor = %q", wc.Changes[0].Actor)
	}
	if wc.Changes[1].Actor != "" {
		t.Fatalf("older revision must have no actor, got %q", wc.Changes[1].Actor)
	}
}

func controllerRevision(name string, owner types.UID, rev int64, created time.Time, t corev1.PodTemplateSpec) *appsv1.ControllerRevision {
	raw, _ := json.Marshal(map[string]any{"spec": map[string]any{"template": t}})
	return &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "klew-lab", Labels: map[string]string{"app": "pay"},
			CreationTimestamp: metav1.NewTime(created),
			OwnerReferences:   []metav1.OwnerReference{{Kind: "StatefulSet", UID: owner, Controller: ptr(true)}}},
		Data:     runtime.RawExtension{Raw: raw},
		Revision: rev,
	}
}

func TestStatefulSetControllerRevisions(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "klew-lab", UID: "sts-1"},
		Spec:       appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "pay"}}, RevisionHistoryLimit: ptr(int32(3))},
		Status:     appsv1.StatefulSetStatus{UpdateRevision: "redis-bbb"},
	}
	cs := fake.NewSimpleClientset(sts,
		controllerRevision("redis-aaa", "sts-1", 1, t0, tpl("redis:7.2")),
		controllerRevision("redis-bbb", "sts-1", 2, t0.Add(time.Hour), tpl("redis:7.4")),
	)
	wc, err := ForWorkload(context.Background(), cs, "StatefulSet", "klew-lab", "redis")
	if err != nil {
		t.Fatal(err)
	}
	if len(wc.Changes) != 2 || wc.Changes[0].Revision != 2 || !wc.Changes[0].Current || wc.HistoryLimit != 3 {
		t.Fatalf("sts = %+v", wc)
	}
	if wc.Changes[0].Summary != "image 7.2 → 7.4" {
		t.Fatalf("decoded template diff = %q", wc.Changes[0].Summary)
	}
}

func TestDaemonSetRollbackByReuse(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "klew-lab", UID: "ds-1"},
		Spec:       appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "pay"}}},
	}
	cs := fake.NewSimpleClientset(ds,
		controllerRevision("agent-new", "ds-1", 2, t0.Add(time.Hour), tpl("agent:2")),
		// Created first but now revision 3: brought back by a rollback.
		controllerRevision("agent-old", "ds-1", 3, t0, tpl("agent:1")),
	)
	wc, _ := ForWorkload(context.Background(), cs, "DaemonSet", "klew-lab", "agent")
	if wc.Changes[0].Source != model.ChangeSourceRollback || !strings.HasPrefix(wc.Changes[0].Summary, "Rollback") {
		t.Fatalf("ds rollback = %+v", wc.Changes[0])
	}
}

func TestRBACDeniedIsAStateNotAnError(t *testing.T) {
	cs := fake.NewSimpleClientset(deployment("dep-1"))
	cs.PrependReactor("list", "replicasets", func(a k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(appsv1.Resource("replicasets"), "", fmt.Errorf("denied"))
	})
	wc, err := ForWorkload(context.Background(), cs, "Deployment", "klew-lab", "payment-api")
	if err != nil {
		t.Fatalf("denied must not be an error: %v", err)
	}
	if wc.AccessState != model.ChangesAccessDenied || wc.AccessMessage != "No permission to read revision history (replicasets list)" {
		t.Fatalf("state = %+v", wc)
	}
}

func pod(name string, owner metav1.OwnerReference, labels map[string]string) *corev1.Pod {
	owner.Controller = ptr(true)
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "klew-lab", Labels: labels,
		OwnerReferences: []metav1.OwnerReference{owner}}}
}

func TestPodBehindCurrentRevision(t *testing.T) {
	d := deployment("dep-1")
	cs := fake.NewSimpleClientset(d,
		rs("pay-2", "dep-1", 2, t0, tpl("repo/pay:v2"), nil),
		rs("pay-3", "dep-1", 3, t0.Add(time.Hour), tpl("repo/pay:v3"), nil),
		pod("pay-2-abc", metav1.OwnerReference{Kind: "ReplicaSet", Name: "pay-2", UID: "rs-pay-2"}, nil),
		pod("pay-3-xyz", metav1.OwnerReference{Kind: "ReplicaSet", Name: "pay-3", UID: "rs-pay-3"}, nil),
	)
	behind, err := ForPod(context.Background(), cs, "klew-lab", "pay-2-abc")
	if err != nil {
		t.Fatal(err)
	}
	if !behind.Behind || behind.Revision != 2 || behind.CurrentRevision != 3 || behind.BehindSummary != "image" || behind.OwnerName != "payment-api" {
		t.Fatalf("behind = %+v", behind)
	}
	cur, _ := ForPod(context.Background(), cs, "klew-lab", "pay-3-xyz")
	if cur.Behind || cur.Revision != 3 {
		t.Fatalf("current = %+v", cur)
	}
}

func TestStatefulSetPodRevisionByHash(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "klew-lab", UID: "sts-1"},
		Spec:       appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "pay"}}},
		Status:     appsv1.StatefulSetStatus{UpdateRevision: "redis-bbb"},
	}
	cs := fake.NewSimpleClientset(sts,
		controllerRevision("redis-aaa", "sts-1", 1, t0, tpl("redis:7.2")),
		controllerRevision("redis-bbb", "sts-1", 2, t0.Add(time.Hour), tpl("redis:7.4")),
		pod("redis-0", metav1.OwnerReference{Kind: "StatefulSet", Name: "redis", UID: "sts-1"}, map[string]string{"controller-revision-hash": "redis-aaa"}),
	)
	pr, _ := ForPod(context.Background(), cs, "klew-lab", "redis-0")
	if !pr.Behind || pr.Revision != 1 || pr.CurrentRevision != 2 {
		t.Fatalf("sts pod = %+v", pr)
	}
}

func TestBarePodHasNoRevision(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "debug", Namespace: "klew-lab"}})
	pr, err := ForPod(context.Background(), cs, "klew-lab", "debug")
	if err != nil || pr.Revision != 0 || pr.Behind {
		t.Fatalf("bare pod = %+v err=%v", pr, err)
	}
}

func TestHelmValuesDiffAndMerge(t *testing.T) {
	revs := []HelmRevision{
		{Revision: 1, Time: t0, Chart: "payment", ChartVersion: "1.0.0", Values: map[string]interface{}{
			"replicaCount": 2, "db": map[string]interface{}{"password": "old-pw", "pool": 20}}},
		{Revision: 2, Time: t0.Add(time.Hour), Chart: "payment", ChartVersion: "1.1.0", Values: map[string]interface{}{
			"replicaCount": 3, "db": map[string]interface{}{"password": "new-pw", "pool": 20}, "featureX": true}},
	}
	helm := HelmChanges(revs)
	if len(helm) != 2 || *helm[0].HelmRevision != 2 || !helm[0].Current {
		t.Fatalf("helm rows = %+v", helm)
	}
	diffs := helm[0].Diffs
	findDiff(t, diffs, "chart")
	if d := findDiff(t, diffs, "values.replicaCount"); d.Before != "2" || d.After != "3" {
		t.Fatalf("replicaCount = %+v", d)
	}
	if d := findDiff(t, diffs, "values.db.password"); d.Before != Redacted || d.After != Redacted {
		t.Fatalf("password must be redacted: %+v", d)
	}
	findDiff(t, diffs, "values.featureX")

	// The rev-2 upgrade caused a rollout 30s later: attach, don't duplicate.
	wc := model.WorkloadChanges{Changes: []model.WorkloadChange{
		{Revision: 5, Time: model.TimestampFrom(t0.Add(time.Hour + 30*time.Second)), Source: model.ChangeSourceRollout, Current: true,
			Diffs: []model.FieldDiff{{Path: "containers[app].image", Before: "a:1", After: "a:2", Kind: model.DiffChanged, Area: AreaImage}}},
	}}
	MergeHelm(&wc, helm)
	if len(wc.Changes) != 2 {
		t.Fatalf("want rollout row + unmatched helm rev 1, got %+v", wc.Changes)
	}
	top := wc.Changes[0]
	if top.Revision != 5 || top.Actor != "helm" || top.HelmRevision == nil || *top.HelmRevision != 2 || !top.Current {
		t.Fatalf("merged row = %+v", top)
	}
	if !strings.HasPrefix(top.Summary, "image 1 → 2") {
		t.Fatalf("merged summary = %q", top.Summary)
	}
}

func TestLatestChangeSkipsSingleRevision(t *testing.T) {
	cs := fake.NewSimpleClientset(deployment("dep-1"), rs("pay-1", "dep-1", 1, t0, tpl("repo/pay:v1"), nil))
	m, err := LatestChange(context.Background(), cs, "Deployment", "klew-lab", "payment-api")
	if err != nil || m != nil {
		t.Fatalf("a single revision is not a change: %+v %v", m, err)
	}
	cs = fake.NewSimpleClientset(deployment("dep-1"),
		rs("pay-1", "dep-1", 1, t0, tpl("repo/pay:v1"), nil),
		rs("pay-2", "dep-1", 2, t0.Add(time.Hour), tpl("repo/pay:v2"), nil))
	m, _ = LatestChange(context.Background(), cs, "Deployment", "klew-lab", "payment-api")
	if m == nil || m.Revision != 2 || len(m.Areas) != 1 || m.Areas[0] != AreaImage {
		t.Fatalf("marker = %+v", m)
	}
}
