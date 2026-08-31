package kube

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/glnreddy421/klew/internal/model"
)

const catalogEntityListLimit = 500

// ListCatalogEntities lists lightweight entity references for a discovered GVR.
func ListCatalogEntities(ctx context.Context, client *Client, namespace, resourceID string) (model.CatalogEntityList, error) {
	empty := model.CatalogEntityList{AccessState: model.ResourceAccessUnknown}
	if client == nil || client.Clientset == nil {
		return empty, fmt.Errorf("kubernetes client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	group, version, resource, err := ParseResourceID(resourceID)
	if err != nil {
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}, nil
	}
	dyn, err := dynamicClient(client)
	if err != nil {
		return empty, err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	var res dynamic.ResourceInterface
	if namespace != "" {
		res = dyn.Resource(gvr).Namespace(namespace)
	} else {
		res = dyn.Resource(gvr)
	}
	ul, err := res.List(ctx, metav1.ListOptions{Limit: catalogEntityListLimit})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return model.CatalogEntityList{AccessState: model.ResourceAccessForbidden}, nil
		}
		if apierrors.IsNotFound(err) {
			return model.CatalogEntityList{AccessState: model.ResourceAccessUnavailable}, nil
		}
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}, nil
	}
	out := make([]model.CatalogEntity, 0, len(ul.Items))
	for _, item := range ul.Items {
		entity := model.CatalogEntity{
			ResourceID:      resourceID,
			Name:            item.GetName(),
			Namespace:       item.GetNamespace(),
			UID:             string(item.GetUID()),
			ResourceVersion: item.GetResourceVersion(),
			Kind:            item.GetKind(),
			APIVersion:      item.GetAPIVersion(),
			StatusHint:      entityStatusHint(item.Object),
		}
		if ts := item.GetCreationTimestamp(); !ts.IsZero() {
			entity.CreationTimestamp = ts.Format(time.RFC3339)
		}
		out = append(out, entity)
	}
	return model.CatalogEntityList{
		Entities:    out,
		AccessState: model.ResourceAccessAllowed,
	}, nil
}

func entityStatusHint(obj map[string]interface{}) string {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return ""
	}
	if phase, ok := status["phase"]; ok {
		return fmt.Sprintf("%v", phase)
	}
	return ""
}
