package investigation

// index is a fast in-memory view over a Dataset used by discovery + builder.
type index struct {
	ds       Dataset
	byKey    map[string]Object
	children map[string][]Object // owner key → direct children
}

func newIndex(ds Dataset) *index {
	ix := &index{ds: ds, byKey: map[string]Object{}, children: map[string][]Object{}}
	for _, o := range ds.Objects {
		ix.byKey[o.Ref.key()] = o
		if o.Owner != nil {
			ix.children[o.Owner.key()] = append(ix.children[o.Owner.key()], o)
		}
	}
	return ix
}

// descendants returns all objects transitively owned by root (excluding root).
func (ix *index) descendants(root Ref) []Object {
	var out []Object
	var walk func(Ref)
	walk = func(r Ref) {
		for _, c := range ix.children[r.key()] {
			out = append(out, c)
			walk(c.Ref)
		}
	}
	walk(root)
	return out
}

func (ix *index) podsOf(root Ref) []Ref {
	var pods []Ref
	for _, d := range ix.descendants(root) {
		if d.Ref.Kind == "Pod" {
			pods = append(pods, d.Ref)
		}
	}
	return pods
}

func (ix *index) membersOf(root Ref) []Ref {
	out := []Ref{root}
	for _, d := range ix.descendants(root) {
		out = append(out, d.Ref)
	}
	return out
}

// podLabels returns the union of labels across a root's pods (best-effort).
func (ix *index) podLabels(root Ref) map[string]string {
	merged := map[string]string{}
	for _, d := range ix.descendants(root) {
		if d.Ref.Kind != "Pod" {
			continue
		}
		for k, v := range d.Labels {
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		if o, ok := ix.byKey[root.key()]; ok {
			for k, v := range o.Labels {
				merged[k] = v
			}
		}
	}
	return merged
}

func (ix *index) serviceFor(root Ref) string {
	labels := ix.podLabels(root)
	for _, o := range ix.ds.Objects {
		if o.Ref.Kind == "Service" && selectorMatches(o.Selector, labels) {
			return o.Ref.Name
		}
	}
	return ""
}

// selectorMatches reports whether every selector key/value is present in labels.
func selectorMatches(selector, labels map[string]string) bool {
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
