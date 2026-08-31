package details

import (
	"context"
	"fmt"
	"strings"
	"sync"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
)

// Provider builds a kind-specific ObjectDetail.
type Provider interface {
	Kind() string
	Build(ctx context.Context, req *Request) (*ObjectDetail, error)
}

// Request carries live client + investigation snapshot for a single object.
type Request struct {
	Client   *kube.Client
	Snapshot model.EvidenceBundle
	Ref      model.ObjectRef
	State    model.InvestigationState
}

var (
	regMu    sync.RWMutex
	registry = map[string]Provider{}
)

func register(p Provider) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[strings.ToLower(p.Kind())] = p
}

func init() {
	register(&podProvider{})
	register(&deploymentProvider{})
	register(&replicaSetProvider{})
	register(&statefulSetProvider{})
	register(&daemonSetProvider{})
	register(&serviceProvider{})
	register(&ingressProvider{})
	register(&endpointSliceProvider{})
	register(&nodeProvider{})
	register(&namespaceProvider{})
	register(&configMapProvider{})
	register(&secretProvider{})
	register(&serviceAccountProvider{})
	register(&roleProvider{})
	register(&roleBindingProvider{})
	register(&clusterRoleProvider{})
	register(&clusterRoleBindingProvider{})
	register(&pvcProvider{})
	register(&pvProvider{})
	register(&storageClassProvider{})
	register(&jobProvider{})
	register(&cronJobProvider{})
	register(&hpaProvider{})
	register(&networkPolicyProvider{})
	register(&gatewayProvider{})
	register(&httpRouteProvider{})
	register(&kyvernoProvider{kind: "ClusterPolicy"})
	register(&kyvernoProvider{kind: "Policy"})
	register(&genericProvider{})
}

// Build returns a kind-aware ObjectDetail. Unknown kinds use the generic provider.
func Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	if req == nil {
		return nil, fmt.Errorf("details: nil request")
	}
	kind := normalizeKind(strings.TrimSpace(req.Ref.Kind))
	req.Ref.Kind = kind
	name := strings.TrimSpace(req.Ref.Name)
	if kind == "" || name == "" {
		return nil, fmt.Errorf("details: kind and name are required")
	}
	if isClusterScopedKind(kind) {
		req.Ref.Namespace = ""
	} else if req.Ref.Namespace == "" && req.Client != nil {
		req.Ref.Namespace = req.Client.Namespace
	}

	regMu.RLock()
	p, ok := registry[strings.ToLower(kind)]
	regMu.RUnlock()
	if !ok {
		p = &genericProvider{}
	}

	detail, err := p.Build(ctx, req)
	if err != nil {
		return nil, wrapDetailErr(err, kind, name)
	}
	if detail == nil {
		return nil, fmt.Errorf("details: empty result for %s/%s", kind, name)
	}
	detail.Kind = kind
	detail.Ref = req.Ref
	if detail.Title == "" {
		detail.Title = kind + "/" + name
	}
	if detail.Category == "" {
		detail.Category = categoryFor(kind)
	}
	detail.Sections = prune(detail.Sections)

	// Always append events when present (provider may already have added).
	if !hasSection(detail.Sections, "events") {
		if ev := eventsSection(req); !ev.Empty() {
			detail.Sections = append(detail.Sections, ev)
		}
	}
	return detail, nil
}

func hasSection(sections []Section, id string) bool {
	for _, s := range sections {
		if s.ID == id {
			return true
		}
	}
	return false
}

func normalizeKind(kind string) string {
	switch strings.ToLower(kind) {
	case "pvc", "persistentvolumeclaim":
		return "PersistentVolumeClaim"
	case "pv", "persistentvolume":
		return "PersistentVolume"
	case "hpa", "horizontalpodautoscaler":
		return "HorizontalPodAutoscaler"
	case "sc", "storageclass":
		return "StorageClass"
	case "sa", "serviceaccount":
		return "ServiceAccount"
	case "netpol", "networkpolicy":
		return "NetworkPolicy"
	case "deploy", "deployment":
		return "Deployment"
	case "sts", "statefulset":
		return "StatefulSet"
	case "ds", "daemonset":
		return "DaemonSet"
	case "rs", "replicaset":
		return "ReplicaSet"
	case "ing", "ingress":
		return "Ingress"
	case "svc", "service":
		return "Service"
	case "cm", "configmap":
		return "ConfigMap"
	case "cj", "cronjob":
		return "CronJob"
	case "ns", "namespace":
		return "Namespace"
	default:
		return kind
	}
}

func isClusterScopedKind(kind string) bool {
	switch kind {
	case "Node", "Namespace", "PersistentVolume", "StorageClass",
		"ClusterRole", "ClusterRoleBinding", "ClusterPolicy":
		return true
	default:
		return false
	}
}

func wrapDetailErr(err error, kind, name string) error {
	if err == nil {
		return nil
	}
	if k8serrors.IsForbidden(err) {
		return fmt.Errorf("you don't have permission to view %s %q", kind, name)
	}
	if k8serrors.IsUnauthorized(err) {
		return fmt.Errorf("not authorized to view %s %q — check your kubeconfig credentials", kind, name)
	}
	if k8serrors.IsNotFound(err) {
		return fmt.Errorf("%s %q was not found in the cluster", kind, name)
	}
	return err
}
