package changes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/glnreddy421/klew/internal/model"
)

const (
	revisionAnnotation        = "deployment.kubernetes.io/revision"
	revisionHistoryAnnotation = "deployment.kubernetes.io/revision-history"
	changeCauseAnnotation     = "kubernetes.io/change-cause"
	// DefaultHistoryLimit is Kubernetes' default revisionHistoryLimit.
	DefaultHistoryLimit = 10
	// actorMatchWindow is how close a managedFields time must be to a revision.
	actorMatchWindow = 90 * time.Second
)

// revision is one stored revision before diffing.
type revision struct {
	number     int64
	time       time.Time
	template   *corev1.PodTemplateSpec
	name       string
	cause      string
	rollback   bool
	rollbackTo int64
}

// ForWorkload returns the revision history of a Deployment, StatefulSet or
// DaemonSet, newest first. Only Get and List are used. Permission problems
// come back as AccessState "denied", not as an error.
func ForWorkload(ctx context.Context, cs kubernetes.Interface, kind, namespace, name string) (model.WorkloadChanges, error) {
	out := model.WorkloadChanges{Kind: kind, Namespace: namespace, Name: name, AccessState: model.ChangesAccessAllowed}
	switch kind {
	case "Deployment":
		return forDeployment(ctx, cs, out)
	case "StatefulSet", "DaemonSet":
		return forControllerRevisions(ctx, cs, out)
	default:
		return out, fmt.Errorf("revision history is not available for %s", kind)
	}
}

func forDeployment(ctx context.Context, cs kubernetes.Interface, out model.WorkloadChanges) (model.WorkloadChanges, error) {
	d, err := cs.AppsV1().Deployments(out.Namespace).Get(ctx, out.Name, metav1.GetOptions{})
	if err != nil {
		return deniedOr(out, err, "deployments get")
	}
	out.UID, out.Generation = string(d.UID), d.Generation
	out.HistoryLimit = historyLimit(d.Spec.RevisionHistoryLimit)
	out.HelmRelease = helmReleaseOf(d.Annotations)

	sel, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return out, err
	}
	list, err := cs.AppsV1().ReplicaSets(out.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return deniedOr(out, err, "replicasets list")
	}
	var revs []revision
	for i := range list.Items {
		rs := &list.Items[i]
		if !ownedBy(rs.OwnerReferences, d.UID) {
			continue
		}
		n, err := strconv.ParseInt(rs.Annotations[revisionAnnotation], 10, 64)
		if err != nil {
			continue
		}
		r := revision{
			number: n, time: rs.CreationTimestamp.Time, template: &rs.Spec.Template,
			name: rs.Name, cause: rs.Annotations[changeCauseAnnotation],
		}
		// A rollback reuses the old ReplicaSet and bumps its revision; the
		// previous numbers are kept in revision-history. Its creation time is
		// the original rollout, so use its last spec/metadata update instead.
		if hist := rs.Annotations[revisionHistoryAnnotation]; hist != "" {
			r.rollback = true
			r.rollbackTo = lowestRevision(hist)
			if t := lastNonStatusUpdate(rs.ManagedFields); !t.IsZero() {
				r.time = t
			}
		}
		revs = append(revs, r)
	}
	out.Changes = buildChanges(revs, d.ManagedFields)
	finish(&out)
	return out, nil
}

func forControllerRevisions(ctx context.Context, cs kubernetes.Interface, out model.WorkloadChanges) (model.WorkloadChanges, error) {
	var (
		uid       types.UID
		selector  *metav1.LabelSelector
		managed   []metav1.ManagedFieldsEntry
		currentCR string
	)
	switch out.Kind {
	case "StatefulSet":
		s, err := cs.AppsV1().StatefulSets(out.Namespace).Get(ctx, out.Name, metav1.GetOptions{})
		if err != nil {
			return deniedOr(out, err, "statefulsets get")
		}
		uid, selector, managed, currentCR = s.UID, s.Spec.Selector, s.ManagedFields, s.Status.UpdateRevision
		out.UID, out.Generation = string(s.UID), s.Generation
		out.HistoryLimit = historyLimit(s.Spec.RevisionHistoryLimit)
		out.HelmRelease = helmReleaseOf(s.Annotations)
	default:
		ds, err := cs.AppsV1().DaemonSets(out.Namespace).Get(ctx, out.Name, metav1.GetOptions{})
		if err != nil {
			return deniedOr(out, err, "daemonsets get")
		}
		uid, selector, managed = ds.UID, ds.Spec.Selector, ds.ManagedFields
		out.UID, out.Generation = string(ds.UID), ds.Generation
		out.HistoryLimit = historyLimit(ds.Spec.RevisionHistoryLimit)
		out.HelmRelease = helmReleaseOf(ds.Annotations)
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return out, err
	}
	list, err := cs.AppsV1().ControllerRevisions(out.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return deniedOr(out, err, "controllerrevisions list")
	}
	var revs []revision
	for i := range list.Items {
		cr := &list.Items[i]
		if !ownedBy(cr.OwnerReferences, uid) {
			continue
		}
		tpl, err := DecodeControllerRevision(cr)
		if err != nil {
			continue
		}
		revs = append(revs, revision{number: cr.Revision, time: cr.CreationTimestamp.Time, template: tpl, name: cr.Name,
			cause: cr.Annotations[changeCauseAnnotation]})
	}
	// ControllerRevisions are reused on rollback too: one created before a
	// lower revision was brought back.
	sort.Slice(revs, func(i, j int) bool { return revs[i].number < revs[j].number })
	for i := range revs {
		for j := 0; j < i; j++ {
			if revs[i].time.Before(revs[j].time) {
				revs[i].rollback = true
				if t := lastNonStatusUpdateFor(list.Items, revs[i].name); !t.IsZero() {
					revs[i].time = t
				}
				break
			}
		}
	}
	out.Changes = buildChanges(revs, managed)
	if currentCR != "" {
		for i := range out.Changes {
			out.Changes[i].Current = out.Changes[i].ObjectName == currentCR
		}
	}
	finish(&out)
	return out, nil
}

// DecodeControllerRevision reads the pod template stored in a ControllerRevision
// (a strategic-merge patch {"spec":{"template":…}} for StatefulSets and DaemonSets).
func DecodeControllerRevision(cr *appsv1.ControllerRevision) (*corev1.PodTemplateSpec, error) {
	raw := cr.Data.Raw
	if len(raw) == 0 && cr.Data.Object != nil {
		b, err := json.Marshal(cr.Data.Object)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	var doc struct {
		Spec struct {
			Template corev1.PodTemplateSpec `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return &doc.Spec.Template, nil
}

// buildChanges sorts revisions oldest→newest, diffs each against the one
// before it, and returns newest first with the highest marked current.
func buildChanges(revs []revision, managed []metav1.ManagedFieldsEntry) []model.WorkloadChange {
	sort.Slice(revs, func(i, j int) bool { return revs[i].number < revs[j].number })
	actors := templateManagers(managed)
	out := make([]model.WorkloadChange, 0, len(revs))
	for i, r := range revs {
		var prev *corev1.PodTemplateSpec
		if i > 0 {
			prev = revs[i-1].template
		}
		diffs := DiffPodTemplates(prev, r.template)
		ch := model.WorkloadChange{
			Revision: r.number, Time: model.TimestampFrom(r.time), Source: model.ChangeSourceRollout,
			ChangeCause: r.cause, Diffs: diffs, ObjectName: r.name,
		}
		switch {
		case r.rollback:
			ch.Source = model.ChangeSourceRollback
			ch.RollbackTo = r.rollbackTo
		case onlyRestart(diffs):
			ch.Source = model.ChangeSourceRestart
		}
		ch.Summary = summaryFor(ch, i == 0)
		ch.Actor = matchActor(actors, r.time)
		out = append(out, ch)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > 0 {
		out[0].Current = true
	}
	return out
}

func summaryFor(ch model.WorkloadChange, first bool) string {
	switch {
	case ch.Source == model.ChangeSourceRollback && ch.RollbackTo > 0:
		return fmt.Sprintf("Rollback to rev %d template", ch.RollbackTo)
	case ch.Source == model.ChangeSourceRollback:
		return "Rollback (template matches an earlier revision)"
	case first:
		return "Oldest retained revision"
	}
	return Summarize(ch.Diffs)
}

func onlyRestart(diffs []model.FieldDiff) bool {
	if len(diffs) == 0 {
		return false
	}
	for _, d := range diffs {
		if d.Area != AreaRestart {
			return false
		}
	}
	return true
}

// templateUpdate is a managedFields entry that touched the pod template.
type templateUpdate struct {
	manager string
	time    time.Time
}

// templateManagers lists managers whose last operation touched spec.template.
// managedFields keeps only the latest time per manager, so this can only
// attribute recent revisions — older ones get no actor.
func templateManagers(mf []metav1.ManagedFieldsEntry) []templateUpdate {
	var out []templateUpdate
	for _, m := range mf {
		if m.Subresource != "" || m.Time == nil || m.FieldsV1 == nil {
			continue
		}
		if !strings.Contains(string(m.FieldsV1.Raw), `"f:template"`) {
			continue
		}
		out = append(out, templateUpdate{manager: m.Manager, time: m.Time.Time})
	}
	return out
}

func matchActor(updates []templateUpdate, at time.Time) string {
	best, bestGap := "", actorMatchWindow+1
	for _, u := range updates {
		gap := u.time.Sub(at)
		if gap < 0 {
			gap = -gap
		}
		if gap <= actorMatchWindow && gap < bestGap {
			best, bestGap = NormalizeActor(u.manager), gap
		}
	}
	return best
}

// NormalizeActor maps field managers to short, recognisable names.
func NormalizeActor(manager string) string {
	m := strings.ToLower(manager)
	switch {
	case m == "":
		return ""
	case strings.HasPrefix(m, "helm"):
		return "helm"
	case strings.Contains(m, "argocd"):
		return "argocd"
	case strings.Contains(m, "kustomize-controller"), strings.Contains(m, "helm-controller"), strings.Contains(m, "flux"):
		return "flux"
	case strings.HasPrefix(m, "kubectl"):
		return manager // kubectl-edit, kubectl-rollout, kubectl-client-side-apply…
	}
	return manager
}

func lastNonStatusUpdate(mf []metav1.ManagedFieldsEntry) time.Time {
	var t time.Time
	for _, m := range mf {
		if m.Subresource == "" && m.Time != nil && m.Time.After(t) {
			t = m.Time.Time
		}
	}
	return t
}

func lastNonStatusUpdateFor(items []appsv1.ControllerRevision, name string) time.Time {
	for i := range items {
		if items[i].Name == name {
			return lastNonStatusUpdate(items[i].ManagedFields)
		}
	}
	return time.Time{}
}

func lowestRevision(history string) int64 {
	var low int64
	for _, part := range strings.Split(history, ",") {
		n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && (low == 0 || n < low) {
			low = n
		}
	}
	return low
}

func ownedBy(refs []metav1.OwnerReference, uid types.UID) bool {
	for _, r := range refs {
		if r.UID == uid {
			return true
		}
	}
	return false
}

func historyLimit(p *int32) int32 {
	if p == nil {
		return DefaultHistoryLimit
	}
	return *p
}

func helmReleaseOf(annotations map[string]string) string {
	return annotations["meta.helm.sh/release-name"]
}

func finish(out *model.WorkloadChanges) {
	for _, c := range out.Changes {
		if c.Current {
			out.CurrentRevision = c.Revision
			break
		}
	}
	if len(out.Changes) > 0 {
		latest := out.Changes[0].Time
		for _, c := range out.Changes {
			if c.Time.After(latest) {
				latest = c.Time
			}
		}
		out.LastChangedAt = latest
	}
}

// deniedOr turns RBAC errors into a readable state; other errors are returned.
func deniedOr(out model.WorkloadChanges, err error, what string) (model.WorkloadChanges, error) {
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
		out.AccessState = model.ChangesAccessDenied
		out.AccessMessage = "No permission to read revision history (" + what + ")"
		return out, nil
	}
	return out, err
}

// LatestChange is the newest revision's marker for investigations. It never
// touches Helm history.
func LatestChange(ctx context.Context, cs kubernetes.Interface, kind, namespace, name string) (*model.WorkloadChangeMarker, error) {
	wc, err := ForWorkload(ctx, cs, kind, namespace, name)
	if err != nil || wc.AccessState != model.ChangesAccessAllowed || len(wc.Changes) == 0 {
		return nil, err
	}
	c := wc.Changes[0]
	if len(wc.Changes) == 1 && c.Source == model.ChangeSourceRollout {
		// Only the oldest retained revision: nothing to say about a change.
		return nil, nil
	}
	return &model.WorkloadChangeMarker{
		Kind: kind, Namespace: namespace, Name: name, Revision: c.Revision, Time: c.Time,
		Source: c.Source, Summary: c.Summary, Areas: Areas(c.Diffs), ObjectName: c.ObjectName,
	}, nil
}
