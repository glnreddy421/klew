package changes

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// CompareRevisions returns two stored revisions of a Deployment, StatefulSet
// or DaemonSet as pod-template YAML, for a line diff. Only Get and List are
// used. Secret-like values are redacted on both sides; a redacted value that
// differs between them reads "‹redacted› (changed)" on the to side, so the
// change still shows without revealing either value.
func CompareRevisions(ctx context.Context, cs kubernetes.Interface, kind, namespace, name string, from, to int64) (model.RevisionCompare, error) {
	out := model.RevisionCompare{Kind: kind, Namespace: namespace, Name: name, AccessState: model.ChangesAccessAllowed}
	wc := model.WorkloadChanges{Kind: kind, Namespace: namespace, Name: name, AccessState: model.ChangesAccessAllowed}
	var (
		revs []revision
		err  error
	)
	switch kind {
	case "Deployment":
		wc, revs, err = deploymentRevisionsFromCluster(ctx, cs, wc)
	case "StatefulSet", "DaemonSet":
		wc, revs, _, _, err = controllerRevisions(ctx, cs, wc)
	default:
		return out, fmt.Errorf("revision comparison is not available for %s", kind)
	}
	if err != nil {
		return out, err
	}
	if wc.AccessState != model.ChangesAccessAllowed {
		out.AccessState, out.AccessMessage = wc.AccessState, wc.AccessMessage
		return out, nil
	}
	byNumber := map[int64]revision{}
	for _, r := range revs {
		byNumber[r.number] = r
	}
	a, ok := byNumber[from]
	if !ok {
		return out, fmt.Errorf("revision %d of %s/%s is no longer kept by the cluster", from, strings.ToLower(kind), name)
	}
	b, ok := byNumber[to]
	if !ok {
		return out, fmt.Errorf("revision %d of %s/%s is no longer kept by the cluster", to, strings.ToLower(kind), name)
	}
	red := &redactor{seen: map[string]string{}}
	fromTpl := red.template(a.template, false)
	toTpl := red.template(b.template, true)
	if out.From, err = revisionYAML(a, fromTpl); err != nil {
		return out, err
	}
	if out.To, err = revisionYAML(b, toTpl); err != nil {
		return out, err
	}
	out.Redacted = red.count
	return out, nil
}

func deploymentRevisionsFromCluster(ctx context.Context, cs kubernetes.Interface, out model.WorkloadChanges) (model.WorkloadChanges, []revision, error) {
	d, err := cs.AppsV1().Deployments(out.Namespace).Get(ctx, out.Name, metav1.GetOptions{})
	if err != nil {
		out, err = deniedOr(out, err, "deployments get")
		return out, nil, err
	}
	sel, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return out, nil, err
	}
	list, err := cs.AppsV1().ReplicaSets(out.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		out, err = deniedOr(out, err, "replicasets list")
		return out, nil, err
	}
	return out, deploymentRevisions(d, list.Items), nil
}

// revisionYAML renders a pod template without the fields every revision
// differs in for no reason: the template hash labels and an empty
// creationTimestamp.
func revisionYAML(r revision, tpl *corev1.PodTemplateSpec) (model.RevisionYAML, error) {
	raw, err := json.Marshal(tpl)
	if err != nil {
		return model.RevisionYAML{}, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return model.RevisionYAML{}, err
	}
	if meta, ok := doc["metadata"].(map[string]any); ok {
		delete(meta, "creationTimestamp")
		if lbl, ok := meta["labels"].(map[string]any); ok {
			delete(lbl, podTemplateHashLabel)
			delete(lbl, controllerRevLabel)
			if len(lbl) == 0 {
				delete(meta, "labels")
			}
		}
		if len(meta) == 0 {
			delete(doc, "metadata")
		}
	}
	y, err := yaml.Marshal(doc)
	if err != nil {
		return model.RevisionYAML{}, err
	}
	return model.RevisionYAML{Revision: r.number, ObjectName: r.name, Time: model.TimestampFrom(r.time), YAML: string(y)}, nil
}

// redactor hides secret-like values in a copy of a pod template. It is run
// on the from side first, remembering each value it hid by position, so the
// to side can mark a hidden value that changed.
type redactor struct {
	seen  map[string]string
	count int
}

func (r *redactor) template(in *corev1.PodTemplateSpec, toSide bool) *corev1.PodTemplateSpec {
	t := in.DeepCopy()
	for k, v := range t.Annotations {
		t.Annotations[k] = r.value("annotation/"+k, k, v, toSide)
	}
	for i := range t.Spec.InitContainers {
		r.container(&t.Spec.InitContainers[i], "init/", toSide)
	}
	for i := range t.Spec.Containers {
		r.container(&t.Spec.Containers[i], "", toSide)
	}
	return t
}

func (r *redactor) container(c *corev1.Container, prefix string, toSide bool) {
	base := prefix + c.Name + "/"
	for i := range c.Env {
		e := &c.Env[i]
		if e.ValueFrom == nil {
			e.Value = r.value(base+"env/"+e.Name, e.Name, e.Value, toSide)
		}
	}
	for i, a := range c.Command {
		c.Command[i] = r.arg(base+"command/"+strconv.Itoa(i), a, toSide)
	}
	for i, a := range c.Args {
		c.Args[i] = r.arg(base+"args/"+strconv.Itoa(i), a, toSide)
	}
	for _, p := range []*corev1.Probe{c.LivenessProbe, c.ReadinessProbe, c.StartupProbe} {
		if p == nil || p.HTTPGet == nil {
			continue
		}
		for i := range p.HTTPGet.HTTPHeaders {
			h := &p.HTTPGet.HTTPHeaders[i]
			h.Value = r.value(base+"header/"+h.Name, h.Name, h.Value, toSide)
		}
	}
}

// arg redacts "--password=…" by its flag name, and any argument that looks
// like a credential on its own.
func (r *redactor) arg(pos, a string, toSide bool) string {
	if k, v, ok := strings.Cut(a, "="); ok && v != "" {
		if RedactValue(strings.TrimLeft(k, "-"), v) == Redacted {
			return k + "=" + r.value(pos, strings.TrimLeft(k, "-"), v, toSide)
		}
	}
	return r.value(pos, "", a, toSide)
}

func (r *redactor) value(pos, key, v string, toSide bool) string {
	shown := RedactValue(key, v)
	if shown != Redacted {
		return v
	}
	r.count++
	if !toSide {
		r.seen[pos] = v
		return shown
	}
	if prev, ok := r.seen[pos]; ok && prev != v {
		return Redacted + " (changed)"
	}
	return shown
}
