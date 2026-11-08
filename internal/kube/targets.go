package kube

import (
	"context"
	"sort"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// targetLister lists the names of one kind for the Investigate picker.
type targetLister struct {
	collector string
	kind      string // static kind, known for the typed listers
	custom    bool
	access    Access
	// list returns kind and names, or an error.
	list func() (kind string, names []string, err error)
}

// ListTargets lists what an investigation can target in ns, grouped by kind:
// the workloads the pod-centric collectors read, and every object a
// namespaced generic collector lists, including user collectors (extra).
// Access is reviewed first; kinds that cannot be listed are reported in
// Unavailable with the reason, never silently left out.
func ListTargets(ctx context.Context, c *Client, ns string, extra []EvidenceCollector) model.TargetList {
	listers := typedTargetListers(ctx, c, ns)
	builtin := builtinResourceNames()
	env := &CollectEnv{Client: c, Namespace: ns}
	for _, col := range append(resourceCollectors(), extra...) {
		rc, ok := col.(*resourceCollector)
		if !ok || rc.spec.ClusterScoped {
			continue
		}
		listers = append(listers, targetLister{
			collector: rc.spec.Name, custom: !builtin[rc.spec.Name], access: rc.Access()[0],
			list: func() (string, []string, error) {
				objs, err := rc.list(ctx, env)
				if err != nil {
					return "", nil, err
				}
				kind := ""
				names := make([]string, 0, len(objs))
				for _, o := range objs {
					kind = nestedString(o, "kind")
					names = append(names, nestedString(o, "metadata", "name"))
				}
				return kind, names, nil
			},
		})
	}

	accesses := make([]Access, len(listers))
	for i, l := range listers {
		accesses[i] = l.access
	}
	reviewed := checkAccess(ctx, c, ns, accesses)
	env.serverGroups()

	var mu sync.Mutex
	byKind := map[string]*model.TargetGroup{}
	var unavailable []model.CollectorStatus
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentLogFetches)
	for _, l := range listers {
		if r := reviewed[l.access.key(ns)]; !r.Allowed && r.Error == "" {
			unavailable = append(unavailable, model.CollectorStatus{Collector: l.collector, Status: StatusNoPermission, Detail: "needs " + l.access.String()})
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			kind, names, err := safeList(l.list)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case errNotServed(err):
				if l.custom {
					unavailable = append(unavailable, model.CollectorStatus{Collector: l.collector, Status: StatusNotInstalled})
				}
			case err != nil:
				unavailable = append(unavailable, model.CollectorStatus{Collector: l.collector, Status: StatusFailed, Detail: err.Error()})
			case kind != "" && len(names) > 0:
				g := byKind[kind]
				if g == nil {
					g = &model.TargetGroup{Kind: kind, Collector: l.collector, Custom: l.custom}
					byKind[kind] = g
				}
				g.Names = append(g.Names, names...)
			}
		}()
	}
	wg.Wait()

	out := model.TargetList{Groups: make([]model.TargetGroup, 0, len(byKind))}
	for _, g := range byKind {
		sort.Strings(g.Names)
		g.Names = uniqueSorted(g.Names)
		out.Groups = append(out.Groups, *g)
	}
	sort.Slice(out.Groups, func(i, j int) bool {
		ri, rj := kindRank(out.Groups[i].Kind), kindRank(out.Groups[j].Kind)
		if ri != rj {
			return ri < rj
		}
		return out.Groups[i].Kind < out.Groups[j].Kind
	})
	sort.Slice(unavailable, func(i, j int) bool { return unavailable[i].Collector < unavailable[j].Collector })
	out.Unavailable = unavailable
	return out
}

// safeList runs one lister, turning a panic into an error.
func safeList(f func() (string, []string, error)) (kind string, names []string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = errFromPanic(p)
		}
	}()
	return f()
}

// typedTargetListers list the kinds the pod-centric collectors read.
// Called with a nil client only to read collector, kind and access.
func typedTargetListers(ctx context.Context, c *Client, ns string) []targetLister {
	opts := metav1.ListOptions{}
	names := func(n int, at func(int) string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = at(i)
		}
		return out
	}
	return []targetLister{
		{collector: "workloads", kind: "Deployment", access: list("apps", "deployments"), list: func() (string, []string, error) {
			l, err := c.Clientset.AppsV1().Deployments(ns).List(ctx, opts)
			if err != nil {
				return "", nil, err
			}
			return "Deployment", names(len(l.Items), func(i int) string { return l.Items[i].Name }), nil
		}},
		{collector: "replicasets", kind: "ReplicaSet", access: list("apps", "replicasets"), list: func() (string, []string, error) {
			l, err := c.Clientset.AppsV1().ReplicaSets(ns).List(ctx, opts)
			if err != nil {
				return "", nil, err
			}
			return "ReplicaSet", names(len(l.Items), func(i int) string { return l.Items[i].Name }), nil
		}},
		{collector: "pods", kind: "Pod", access: list("", "pods"), list: func() (string, []string, error) {
			l, err := c.Clientset.CoreV1().Pods(ns).List(ctx, opts)
			if err != nil {
				return "", nil, err
			}
			return "Pod", names(len(l.Items), func(i int) string { return l.Items[i].Name }), nil
		}},
		{collector: "jobs", kind: "Job", access: list("batch", "jobs"), list: func() (string, []string, error) {
			l, err := c.Clientset.BatchV1().Jobs(ns).List(ctx, opts)
			if err != nil {
				return "", nil, err
			}
			return "Job", names(len(l.Items), func(i int) string { return l.Items[i].Name }), nil
		}},
		{collector: "services", kind: "Service", access: list("", "services"), list: func() (string, []string, error) {
			l, err := c.Clientset.CoreV1().Services(ns).List(ctx, opts)
			if err != nil {
				return "", nil, err
			}
			return "Service", names(len(l.Items), func(i int) string { return l.Items[i].Name }), nil
		}},
		{collector: "ingresses", kind: "Ingress", access: list("networking.k8s.io", "ingresses"), list: func() (string, []string, error) {
			l, err := c.Clientset.NetworkingV1().Ingresses(ns).List(ctx, opts)
			if err != nil {
				return "", nil, err
			}
			return "Ingress", names(len(l.Items), func(i int) string { return l.Items[i].Name }), nil
		}},
	}
}

// kindRank puts workloads first, then other built-in kinds, then the rest.
func kindRank(kind string) int {
	for i, k := range targetKindPriority {
		if k == kind {
			return i
		}
	}
	return len(targetKindPriority)
}

func uniqueSorted(in []string) []string {
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}
