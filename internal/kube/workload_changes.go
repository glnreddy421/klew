package kube

import (
	"context"
	"strconv"
	"sync"

	"github.com/glnreddy421/klew/internal/changes"
	"github.com/glnreddy421/klew/internal/model"
)

// changeMarkerCache keeps each workload's latest change per UID + generation,
// so the investigation's periodic refresh does not re-list and re-diff
// revisions unless the spec changed. Helm history is never read here.
var changeMarkerCache = struct {
	sync.Mutex
	m map[string]*model.WorkloadChangeMarker
}{m: map[string]*model.WorkloadChangeMarker{}}

const changeMarkerCacheMax = 256

// collectChangeMarkers returns the latest change of each in-scope workload
// that has one. Errors and denied access are skipped silently.
func (col *Collector) collectChangeMarkers(ctx context.Context, workloads []model.WorkloadSummary) []model.WorkloadChangeMarker {
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
			var err error
			m, err = changes.LatestChange(ctx, col.Client.Clientset, kind, w.Namespace, w.Name)
			if err != nil {
				continue
			}
			changeMarkerCache.Lock()
			if len(changeMarkerCache.m) >= changeMarkerCacheMax {
				changeMarkerCache.m = map[string]*model.WorkloadChangeMarker{}
			}
			changeMarkerCache.m[key] = m
			changeMarkerCache.Unlock()
		}
		if m != nil {
			out = append(out, *m)
		}
	}
	return out
}
