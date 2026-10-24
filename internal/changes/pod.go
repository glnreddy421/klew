package changes

import (
	"context"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/glnreddy421/klew/internal/model"
)

// ForPod reports which revision of its workload a Pod runs and whether it is
// behind the current one. Pods without a revisioned owner return no revision.
func ForPod(ctx context.Context, cs kubernetes.Interface, namespace, name string) (model.PodRevision, error) {
	out := model.PodRevision{AccessState: model.ChangesAccessAllowed}
	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return podDeniedOr(out, err, "pods get")
	}
	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		return out, nil
	}

	var (
		wc       model.WorkloadChanges
		podRev   int64
		ownerErr error
	)
	switch owner.Kind {
	case "ReplicaSet":
		rs, err := cs.AppsV1().ReplicaSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return podDeniedOr(out, err, "replicasets get")
		}
		dep := metav1.GetControllerOf(rs)
		if dep == nil || dep.Kind != "Deployment" {
			return out, nil
		}
		out.OwnerKind, out.OwnerName = "Deployment", dep.Name
		podRev, _ = strconv.ParseInt(rs.Annotations[revisionAnnotation], 10, 64)
		wc, ownerErr = ForWorkload(ctx, cs, "Deployment", namespace, dep.Name)
	case "StatefulSet", "DaemonSet":
		out.OwnerKind, out.OwnerName = owner.Kind, owner.Name
		wc, ownerErr = ForWorkload(ctx, cs, owner.Kind, namespace, owner.Name)
		hash := pod.Labels[controllerRevLabel]
		for _, c := range wc.Changes {
			// StatefulSet pods carry the full revision name; DaemonSet pods the hash suffix.
			if c.ObjectName == hash || strings.HasSuffix(c.ObjectName, "-"+hash) {
				podRev = c.Revision
				break
			}
		}
	default:
		return out, nil
	}
	if ownerErr != nil {
		return out, ownerErr
	}
	if wc.AccessState != model.ChangesAccessAllowed {
		out.AccessState, out.AccessMessage = wc.AccessState, wc.AccessMessage
		return out, nil
	}
	out.Revision, out.CurrentRevision = podRev, wc.CurrentRevision
	for _, c := range wc.Changes {
		if c.Current {
			out.CurrentSince = c.Time
		}
	}
	if podRev > 0 && wc.CurrentRevision > podRev {
		out.Behind = true
		out.BehindSummary = strings.Join(areasSince(wc.Changes, podRev), ", ")
	}
	return out, nil
}

// areasSince lists what changed in revisions newer than rev.
func areasSince(changes []model.WorkloadChange, rev int64) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range changes {
		if c.Revision <= rev {
			continue
		}
		for _, a := range Areas(c.Diffs) {
			if !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
	}
	return out
}

func podDeniedOr(out model.PodRevision, err error, what string) (model.PodRevision, error) {
	wc, err := deniedOr(model.WorkloadChanges{}, err, what)
	if err != nil {
		return out, err
	}
	out.AccessState, out.AccessMessage = wc.AccessState, wc.AccessMessage
	return out, nil
}
