package kube

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// Generic read-only access to any discovered resource type, for callers that
// take a type name from a person or an assistant (klew mcp). Only list, get
// and pod log reads; nothing here writes to the cluster.

// DiscoverResources returns the cluster's preferred API resources (cached per
// context, like the browse catalog).
func DiscoverResources(client *Client) ([]DiscoveredResource, error) {
	if client == nil || client.Clientset == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	resources, _, err := discoverAPIResources(client)
	return resources, err
}

// GVR returns the resource's group, version and resource name.
func (d DiscoveredResource) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: d.Group, Version: d.Version, Resource: d.Resource}
}

// ResolveResourceType finds the resource a typed name refers to: a plural
// resource (deployments), a kind (Deployment), a short name (deploy) or
// resource.group (certificates.cert-manager.io). Matching ignores case. When
// several groups serve the name, the core and Kubernetes groups win; other
// ties are reported so the caller can name the group.
func ResolveResourceType(resources []DiscoveredResource, typ string) (DiscoveredResource, error) {
	want := strings.ToLower(strings.TrimSpace(typ))
	if want == "" {
		return DiscoveredResource{}, fmt.Errorf("resource type is required")
	}
	var matches []DiscoveredResource
	for _, r := range resources {
		if resourceMatchesKind(want, r.Kind, r.Resource, r.ShortNames) ||
			(r.Group != "" && want == r.Resource+"."+r.Group) ||
			(r.Group != "" && want == strings.ToLower(r.Kind)+"."+r.Group) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return DiscoveredResource{}, fmt.Errorf("no resource type %q in this cluster; use a kind (Deployment), plural (deployments), short name (deploy) or resource.group for custom resources", typ)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	sort.SliceStable(matches, func(i, j int) bool { return groupRank(matches[i].Group) < groupRank(matches[j].Group) })
	if groupRank(matches[0].Group) < groupRank(matches[1].Group) {
		return matches[0], nil
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.Resource+"."+m.Group)
	}
	return DiscoveredResource{}, fmt.Errorf("resource type %q is ambiguous; name one of: %s", typ, strings.Join(names, ", "))
}

// groupRank orders API groups for ambiguous names: core, then built-in
// groups (apps, batch, *.k8s.io), then everything else.
func groupRank(group string) int {
	switch {
	case group == "":
		return 0
	case !strings.Contains(group, ".") || strings.HasSuffix(group, ".k8s.io"):
		return 1
	}
	return 2
}

// ResourceListOptions narrows ListResources.
type ResourceListOptions struct {
	// Namespace is required for namespaced resources and ignored otherwise.
	Namespace     string
	LabelSelector string
	FieldSelector string
	Limit         int64
}

// ResourceList is the first page of a list.
type ResourceList struct {
	Entities []model.CatalogEntity
	// More is true when the API has items beyond this page.
	More bool
	// Remaining is the API's estimate of items left, when it gives one.
	Remaining *int64
}

// ListResources lists one page of r as browse rows. Unlike the browse
// catalog it keeps pods in every phase, so completed and failed pods show.
func ListResources(ctx context.Context, client *Client, r DiscoveredResource, opts ResourceListOptions) (ResourceList, error) {
	dyn, err := dynamicClient(client)
	if err != nil {
		return ResourceList{}, err
	}
	ns := ""
	if r.Namespaced {
		ns = opts.Namespace
	}
	ul, err := dyn.Resource(r.GVR()).Namespace(ns).List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector, FieldSelector: opts.FieldSelector, Limit: opts.Limit,
	})
	if err != nil {
		return ResourceList{}, err
	}
	id := ResourceID(r.Group, r.Version, r.Resource)
	var endpoints map[string]serviceEndpointCount
	if r.Resource == "services" && r.Group == "" && ns != "" {
		endpoints = catalogServiceEndpointCountsByName(ctx, client, ns)
	}
	out := ResourceList{Entities: make([]model.CatalogEntity, 0, len(ul.Items)), More: ul.GetContinue() != "", Remaining: ul.GetRemainingItemCount()}
	for _, item := range ul.Items {
		out.Entities = append(out.Entities, catalogEntityFor(item, id, r.Resource, endpoints))
	}
	if r.Resource == "pods" && r.Group == "" && len(out.Entities) > 0 {
		enrichListedPodMetrics(ctx, client, out.Entities, ns)
	}
	return out, nil
}

// GetResource gets one object of r. namespace is ignored for cluster-scoped types.
func GetResource(ctx context.Context, client *Client, r DiscoveredResource, namespace, name string) (*unstructured.Unstructured, model.CatalogEntity, error) {
	dyn, err := dynamicClient(client)
	if err != nil {
		return nil, model.CatalogEntity{}, err
	}
	if !r.Namespaced {
		namespace = ""
	}
	obj, err := dyn.Resource(r.GVR()).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, model.CatalogEntity{}, err
	}
	return obj, catalogEntityFor(*obj, ResourceID(r.Group, r.Version, r.Resource), r.Resource, nil), nil
}

// SummarizePod is the investigation engine's view of one pod.
func SummarizePod(pod corev1.Pod) model.PodSummary {
	return summarizePod(pod)
}

// PodLogLine is one container log line with the kubelet's timestamp.
type PodLogLine struct {
	Time time.Time
	Text string
}

// PodLogOptions selects which log lines ReadPodLogs returns.
type PodLogOptions struct {
	Container    string
	Previous     bool
	TailLines    int64
	SinceSeconds int64
	// LimitBytes caps what the kubelet sends; 0 means no cap.
	LimitBytes int64
}

// ReadPodLogs reads (does not follow) a container's recent log lines, oldest
// first. It reports truncated when LimitBytes cut the read short.
func ReadPodLogs(ctx context.Context, client *Client, namespace, pod string, opts PodLogOptions) (lines []PodLogLine, truncated bool, err error) {
	lo := &corev1.PodLogOptions{Container: opts.Container, Previous: opts.Previous, Timestamps: true}
	if opts.TailLines > 0 {
		lo.TailLines = &opts.TailLines
	}
	if opts.SinceSeconds > 0 {
		lo.SinceSeconds = &opts.SinceSeconds
	}
	if opts.LimitBytes > 0 {
		lo.LimitBytes = &opts.LimitBytes
	}
	rc, err := client.Clientset.CoreV1().Pods(namespace).GetLogs(pod, lo).Stream(ctx)
	if err != nil {
		return nil, false, err
	}
	defer rc.Close()
	var read int64
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		raw := sc.Text()
		read += int64(len(raw)) + 1
		t, text := splitLogTimestamp(raw, time.Time{})
		lines = append(lines, PodLogLine{Time: t, Text: text})
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return lines, true, err
	}
	return lines, opts.LimitBytes > 0 && read >= opts.LimitBytes, nil
}
