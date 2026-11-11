package kube

import (
	"context"
	"strconv"
	"sync"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/glnreddy421/klew/v2/internal/changes"
	"github.com/glnreddy421/klew/v2/internal/model"
)

// changeMarkerCache keeps each workload's latest change per UID + generation,
// so the investigation's periodic refresh does not re-list and re-diff
// revisions unless the spec changed. Helm history is never read here.
var changeMarkerCache = struct {
	sync.Mutex
	m map[string]*model.WorkloadChangeMarker
}{m: map[string]*model.WorkloadChangeMarker{}}

const changeMarkerCacheMax = 2048

// collectChangeMarkers returns the latest change of each in-scope workload
// that has one. A Deployment in deployments is read from replicaSets with no
// API calls; any other workload is read from the cluster. Errors and denied
// access are skipped silently.
func (col *Collector) collectChangeMarkers(ctx context.Context, workloads []model.WorkloadSummary, deployments map[string]*appsv1.Deployment, replicaSets []appsv1.ReplicaSet) []model.WorkloadChangeMarker {
	var out []model.WorkloadChangeMarker
	for _, w := range workloads {
		if w.UID == "" {
			continue
		}
		key := w.UID + "|" + strconv.FormatInt(w.Generation, 10)
		changeMarkerCache.Lock()
		m, ok := changeMarkerCache.m[key]
		changeMarkerCache.Unlock()
		if !ok {
			kind := w.Kind
			if kind == "" {
				kind = "Deployment"
			}
			if d := deployments[w.Name]; kind == "Deployment" && d != nil && string(d.UID) == w.UID {
				m = changes.LatestDeploymentChange(d, replicaSets)
			} else {
				var err error
				m, err = changes.LatestChange(ctx, col.Client.Clientset, kind, w.Namespace, w.Name)
				if err != nil {
					continue
				}
			}
			cacheChangeMarker(key, m)
		}
		if m != nil {
			out = append(out, *m)
		}
	}
	return out
}

// cacheChangeMarker stores m, making room by dropping one entry when the
// cache is full rather than emptying it, so a namespace with more workloads
// than the cache holds still hits it for most of them.
func cacheChangeMarker(key string, m *model.WorkloadChangeMarker) {
	changeMarkerCache.Lock()
	defer changeMarkerCache.Unlock()
	if _, ok := changeMarkerCache.m[key]; !ok && len(changeMarkerCache.m) >= changeMarkerCacheMax {
		for k := range changeMarkerCache.m {
			delete(changeMarkerCache.m, k)
			break
		}
	}
	changeMarkerCache.m[key] = m
}
