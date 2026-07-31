package details

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/glnreddy421/klew/internal/model"
)

func cs(req *Request) kubernetes.Interface {
	if req == nil || req.Client == nil {
		return nil
	}
	return req.Client.Clientset
}

func nsOr(req *Request, fallback string) string {
	if req.Ref.Namespace != "" {
		return req.Ref.Namespace
	}
	if fallback != "" {
		return fallback
	}
	if req.Client != nil {
		return req.Client.Namespace
	}
	return "default"
}

func getPod(ctx context.Context, req *Request) (*corev1.Pod, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().Pods(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getDeployment(ctx context.Context, req *Request) (*appsv1.Deployment, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.AppsV1().Deployments(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getReplicaSet(ctx context.Context, req *Request) (*appsv1.ReplicaSet, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.AppsV1().ReplicaSets(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getStatefulSet(ctx context.Context, req *Request) (*appsv1.StatefulSet, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.AppsV1().StatefulSets(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getDaemonSet(ctx context.Context, req *Request) (*appsv1.DaemonSet, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.AppsV1().DaemonSets(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getService(ctx context.Context, req *Request) (*corev1.Service, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().Services(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getIngress(ctx context.Context, req *Request) (*networkingv1.Ingress, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.NetworkingV1().Ingresses(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getNode(ctx context.Context, req *Request) (*corev1.Node, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().Nodes().Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getNamespace(ctx context.Context, req *Request) (*corev1.Namespace, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().Namespaces().Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getConfigMap(ctx context.Context, req *Request) (*corev1.ConfigMap, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().ConfigMaps(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getSecret(ctx context.Context, req *Request) (*corev1.Secret, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().Secrets(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getServiceAccount(ctx context.Context, req *Request) (*corev1.ServiceAccount, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().ServiceAccounts(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getRole(ctx context.Context, req *Request) (*rbacv1.Role, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.RbacV1().Roles(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getRoleBinding(ctx context.Context, req *Request) (*rbacv1.RoleBinding, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.RbacV1().RoleBindings(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getClusterRole(ctx context.Context, req *Request) (*rbacv1.ClusterRole, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.RbacV1().ClusterRoles().Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getClusterRoleBinding(ctx context.Context, req *Request) (*rbacv1.ClusterRoleBinding, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.RbacV1().ClusterRoleBindings().Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getPVC(ctx context.Context, req *Request) (*corev1.PersistentVolumeClaim, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().PersistentVolumeClaims(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getPV(ctx context.Context, req *Request) (*corev1.PersistentVolume, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.CoreV1().PersistentVolumes().Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getStorageClass(ctx context.Context, req *Request) (*storagev1.StorageClass, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.StorageV1().StorageClasses().Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getJob(ctx context.Context, req *Request) (*batchv1.Job, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.BatchV1().Jobs(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getCronJob(ctx context.Context, req *Request) (*batchv1.CronJob, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.BatchV1().CronJobs(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getHPA(ctx context.Context, req *Request) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.AutoscalingV2().HorizontalPodAutoscalers(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getNetworkPolicy(ctx context.Context, req *Request) (*networkingv1.NetworkPolicy, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.NetworkingV1().NetworkPolicies(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getEndpointSlicesForService(ctx context.Context, req *Request, svcName string) ([]discoveryv1.EndpointSlice, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	list, err := c.DiscoveryV1().EndpointSlices(nsOr(req, "")).List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + svcName,
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func getEndpointSlice(ctx context.Context, req *Request) (*discoveryv1.EndpointSlice, error) {
	c := cs(req)
	if c == nil {
		return nil, fmt.Errorf("no client")
	}
	return c.DiscoveryV1().EndpointSlices(nsOr(req, "")).Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func getUnstructured(ctx context.Context, req *Request, gvr schema.GroupVersionResource, namespaced bool) (*unstructured.Unstructured, error) {
	if req.Client == nil || req.Client.Config == nil {
		return nil, fmt.Errorf("no client")
	}
	dyn, err := dynamic.NewForConfig(req.Client.Config)
	if err != nil {
		return nil, err
	}
	var ri dynamic.ResourceInterface
	if namespaced {
		ri = dyn.Resource(gvr).Namespace(nsOr(req, ""))
	} else {
		ri = dyn.Resource(gvr)
	}
	return ri.Get(ctx, req.Ref.Name, metav1.GetOptions{})
}

func selectorString(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func ownerRefsFromMeta(owners []metav1.OwnerReference, ns string) []model.ObjectRef {
	var out []model.ObjectRef
	for _, o := range owners {
		out = append(out, model.ObjectRef{Kind: o.Kind, Name: o.Name, Namespace: ns, UID: string(o.UID)})
	}
	return out
}

func podsMatchingLabels(snap model.EvidenceBundle, selector map[string]string) []model.PodSummary {
	if len(selector) == 0 {
		return nil
	}
	var out []model.PodSummary
	for _, p := range snap.Pods {
		if labelsMatch(p.Labels, selector) {
			out = append(out, p)
		}
	}
	return out
}

func labelsMatch(labels, selector map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func podsUsingConfig(snap model.EvidenceBundle, kind, name string) []string {
	var names []string
	for _, p := range snap.Pods {
		switch kind {
		case "ConfigMap":
			for _, r := range p.ConfigMapRefs {
				if r == name {
					names = append(names, p.Name)
					break
				}
			}
		case "Secret":
			for _, r := range p.SecretRefs {
				if r == name {
					names = append(names, p.Name)
					break
				}
			}
		case "PersistentVolumeClaim":
			for _, r := range p.PVCRefs {
				if r == name {
					names = append(names, p.Name)
					break
				}
			}
		}
	}
	return names
}
