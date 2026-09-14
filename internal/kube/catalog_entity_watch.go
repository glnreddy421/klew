package kube

import (
	"context"
	"errors"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/glnreddy421/klew/internal/model"
)

const (
	catalogWatchPollWide     = 45 * time.Second
	catalogWatchPollVirtual  = 30 * time.Second
	catalogWatchDebounce     = 400 * time.Millisecond
	catalogWatchMinRelistGap = 2 * time.Second
)

// CatalogEntityWatchSink receives entity list snapshots while a browse watch is active.
type CatalogEntityWatchSink func(list model.CatalogEntityList, live bool)

// CatalogEntityWatchSupports reports whether a low-latency watch can be used for this scope.
// Wide scopes fall back to periodic polling.
func CatalogEntityWatchSupports(resourceID string, scope CatalogEntityScope) bool {
	if IsVirtualCatalogResource(resourceID) {
		return false
	}
	if scope.AllNamespaces || len(scope.Namespaces) > 1 {
		return false
	}
	return true
}

// RunCatalogEntityWatch blocks until ctx is cancelled. It emits full snapshots via sink.
func RunCatalogEntityWatch(ctx context.Context, client *Client, resourceID string, scope CatalogEntityScope, sink CatalogEntityWatchSink) {
	if client == nil || sink == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if IsVirtualCatalogResource(resourceID) {
		runCatalogEntityPoll(ctx, client, resourceID, scope, sink, catalogWatchPollVirtual)
		return
	}
	if !CatalogEntityWatchSupports(resourceID, scope) {
		runCatalogEntityPoll(ctx, client, resourceID, scope, sink, catalogWatchPollWide)
		return
	}
	runCatalogEntityWatchScoped(ctx, client, resourceID, scope, sink)
}

func runCatalogEntityPoll(ctx context.Context, client *Client, resourceID string, scope CatalogEntityScope, sink CatalogEntityWatchSink, every time.Duration) {
	emit := func() {
		list, err := ListCatalogEntities(ctx, client, resourceID, scope)
		if err != nil && list.AccessState == "" {
			list = model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}
		}
		sink(list, false)
	}
	emit()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			emit()
		}
	}
}

func runCatalogEntityWatchScoped(ctx context.Context, client *Client, resourceID string, scope CatalogEntityScope, sink CatalogEntityWatchSink) {
	group, version, resource, err := ParseResourceID(resourceID)
	if err != nil {
		sink(model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}, false)
		return
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	ns := scope.Namespace
	if ns == "" && len(scope.Namespaces) == 1 {
		ns = scope.Namespaces[0]
	}

	emit := func(live bool) {
		listScope := CatalogEntityScope{
			Namespace:     ns,
			ClusterScoped: scope.ClusterScoped,
		}
		list, err := ListCatalogEntities(ctx, client, resourceID, listScope)
		if err != nil && list.AccessState == "" {
			list = model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}
		}
		sink(list, live)
	}

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		established, watchErr := watchCatalogEntitiesOnce(ctx, client, gvr, ns, scope.ClusterScoped, emit)
		if ctx.Err() != nil {
			return
		}
		if established {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		_ = watchErr
		if !established {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func watchCatalogEntitiesOnce(
	ctx context.Context,
	client *Client,
	gvr schema.GroupVersionResource,
	namespace string,
	clusterScoped bool,
	emit func(live bool),
) (established bool, err error) {
	emit(false)

	dyn, err := dynamicClient(client)
	if err != nil {
		return false, err
	}
	var res dynamic.ResourceInterface
	if clusterScoped || namespace == "" {
		res = dyn.Resource(gvr)
	} else {
		res = dyn.Resource(gvr).Namespace(namespace)
	}

	listCtx, cancel := context.WithTimeout(ctx, catalogRequestTimeout)
	ul, err := res.List(listCtx, metav1.ListOptions{Limit: catalogEntityListLimit})
	cancel()
	if err != nil {
		if apierrors.IsForbidden(err) {
			return false, nil
		}
		return false, err
	}

	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()
	rv, _, _ := unstructured.NestedString(ul.Object, "metadata", "resourceVersion")
	watcher, err := res.Watch(watchCtx, metav1.ListOptions{ResourceVersion: rv})
	if err != nil {
		return false, err
	}
	defer watcher.Stop()

	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	defer debounce.Stop()

	pending := false
	lastRelist := time.Now()

	for {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case <-debounce.C:
			if !pending {
				continue
			}
			if time.Since(lastRelist) < catalogWatchMinRelistGap {
				debounce.Reset(catalogWatchMinRelistGap - time.Since(lastRelist))
				continue
			}
			emit(true)
			lastRelist = time.Now()
			pending = false
		case ev, ok := <-watcher.ResultChan():
			if !ok {
				return true, errors.New("watch closed")
			}
			switch ev.Type {
			case watch.Added, watch.Modified, watch.Deleted, watch.Bookmark:
				pending = true
				debounce.Reset(catalogWatchDebounce)
			case watch.Error:
				return true, errors.New("watch error event")
			}
		}
	}
}
