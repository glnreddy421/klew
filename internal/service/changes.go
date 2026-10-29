package service

import (
	"context"

	"github.com/glnreddy421/klew/internal/changes"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// WorkloadChanges returns a workload's revision history, newest first
// (Deployment, StatefulSet, DaemonSet, or a HelmRelease). Read-only. Helm
// history is decoded only on request — never during investigation refreshes.
func WorkloadChanges(ctx context.Context, client *kube.Client, kind, namespace, name string) (model.WorkloadChanges, error) {
	if kind == "HelmRelease" {
		return helmReleaseChanges(ctx, client, namespace, name)
	}
	wc, err := changes.ForWorkload(ctx, client.Clientset, kind, namespace, name)
	if err != nil || wc.AccessState != model.ChangesAccessAllowed || wc.HelmRelease == "" {
		return wc, err
	}
	revs, access, herr := kube.ListHelmReleaseHistory(ctx, client, namespace, wc.HelmRelease)
	switch {
	case herr != nil:
		wc.HelmNote = "Helm history unavailable"
	case access == model.ResourceAccessForbidden:
		wc.HelmNote = "No permission to read Helm release history (secrets list)"
	case len(revs) > 0:
		changes.MergeHelm(&wc, changes.HelmChanges(revs))
	}
	return wc, nil
}

func helmReleaseChanges(ctx context.Context, client *kube.Client, ns, release string) (model.WorkloadChanges, error) {
	out := model.WorkloadChanges{Kind: "HelmRelease", Namespace: ns, Name: release, HelmRelease: release, AccessState: model.ChangesAccessAllowed}
	revs, access, err := kube.ListHelmReleaseHistory(ctx, client, ns, release)
	if err != nil {
		return out, err
	}
	if access == model.ResourceAccessForbidden {
		out.AccessState = model.ChangesAccessDenied
		out.AccessMessage = "No permission to read revision history (secrets list)"
		return out, nil
	}
	out.Changes = changes.HelmChanges(revs)
	for _, c := range out.Changes {
		if c.Current && c.HelmRevision != nil {
			out.CurrentRevision = int64(*c.HelmRevision)
			out.LastChangedAt = c.Time
		}
	}
	return out, nil
}
