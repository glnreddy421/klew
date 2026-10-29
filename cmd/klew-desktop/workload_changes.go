package main

import (
	"context"
	"fmt"
	"time"

	"github.com/glnreddy421/klew/internal/changes"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/service"
)

const workloadChangesTimeout = 20 * time.Second

// GetWorkloadChanges returns a workload's revision history, newest first
// (Deployment, StatefulSet, DaemonSet, or a HelmRelease). Read-only. Helm
// history is decoded only here — when the Changes tab opens — never during
// investigation refreshes.
func (a *App) GetWorkloadChanges(kind, namespace, name string) (model.WorkloadChanges, error) {
	client, ns, err := a.resolveBrowseClient(namespace)
	if err != nil {
		return model.WorkloadChanges{}, err
	}
	ctx, cancel := context.WithTimeout(a.baseContext(), workloadChangesTimeout)
	defer cancel()

	return service.WorkloadChanges(ctx, client, kind, ns, name)
}

// GetPodRevision reports which workload revision a Pod runs and whether it is
// behind the current one.
func (a *App) GetPodRevision(namespace, name string) (model.PodRevision, error) {
	client, ns, err := a.resolveBrowseClient(namespace)
	if err != nil {
		return model.PodRevision{}, err
	}
	if name == "" {
		return model.PodRevision{}, fmt.Errorf("pod name is required")
	}
	ctx, cancel := context.WithTimeout(a.baseContext(), workloadChangesTimeout)
	defer cancel()
	return changes.ForPod(ctx, client.Clientset, ns, name)
}

func (a *App) baseContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
