package details

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glnreddy421/klew/internal/model"
)

func field(k, v string) Field {
	return Field{Key: k, Value: strings.TrimSpace(v)}
}

func fields(pairs ...string) []Field {
	out := make([]Field, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		k, v := pairs[i], pairs[i+1]
		if strings.TrimSpace(v) == "" || v == "—" {
			continue
		}
		out = append(out, field(k, v))
	}
	return out
}

func kvMap(m map[string]string) []KeyValue {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]KeyValue, 0, len(keys))
	for _, k := range keys {
		out = append(out, KeyValue{Key: k, Value: m[k]})
	}
	return out
}

func sectionFields(id, title, group string, f []Field) Section {
	return Section{ID: id, Title: title, Group: group, Fields: f}
}

func sectionKV(id, title, group string, kv []KeyValue) Section {
	return Section{ID: id, Title: title, Group: group, KeyValues: kv}
}

func sectionTable(id, title, group string, cols []string, rows [][]string) Section {
	return Section{ID: id, Title: title, Group: group, Table: &Table{Columns: cols, Rows: rows}}
}

func prune(sections []Section) []Section {
	out := make([]Section, 0, len(sections))
	for _, s := range sections {
		if s.Empty() {
			continue
		}
		out = append(out, s)
	}
	return out
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func joinNonEmpty(parts ...string) string {
	var b []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			b = append(b, p)
		}
	}
	return strings.Join(b, ", ")
}

func fmtInt32(n int32) string { return fmt.Sprintf("%d", n) }

func fmtInt64(n int64) string { return fmt.Sprintf("%d", n) }

func ownerRows(refs []model.ObjectRef) [][]string {
	var rows [][]string
	for _, r := range refs {
		rows = append(rows, []string{r.Kind, r.Name, r.Namespace, r.UID})
	}
	return rows
}

func metaSections(labels, annotations map[string]string, owners []model.ObjectRef) []Section {
	var out []Section
	if kv := kvMap(labels); len(kv) > 0 {
		out = append(out, sectionKV("labels", "Labels", GroupMetadata, kv))
	}
	if kv := kvMap(annotations); len(kv) > 0 {
		out = append(out, sectionKV("annotations", "Annotations", GroupMetadata, kv))
	}
	if rows := ownerRows(owners); len(rows) > 0 {
		out = append(out, sectionTable("ownerRefs", "Owner References", GroupRelationships,
			[]string{"Kind", "Name", "Namespace", "UID"}, rows))
	}
	return out
}

func categoryFor(kind string) string {
	switch kind {
	case "Pod":
		return "runtime"
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
		return "workload"
	case "Service", "Ingress", "Endpoints", "EndpointSlice", "NetworkPolicy", "Gateway", "HTTPRoute":
		return "network"
	case "ConfigMap", "Secret":
		return "config"
	case "PersistentVolumeClaim", "PersistentVolume", "StorageClass":
		return "storage"
	case "ServiceAccount", "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		return "access"
	case "HorizontalPodAutoscaler":
		return "autoscaling"
	case "Node", "Namespace":
		return "cluster"
	case "ClusterPolicy", "Policy", "PolicyReport", "ClusterPolicyReport":
		return "policy"
	default:
		return "component"
	}
}

func statusFromPhase(phase string, ready bool) StatusBadge {
	p := strings.ToLower(phase)
	switch {
	case p == "running" && ready:
		return StatusBadge{Tone: "healthy", Label: "Ready"}
	case p == "running" && !ready:
		return StatusBadge{Tone: "warning", Label: "Not Ready"}
	case p == "pending":
		return StatusBadge{Tone: "warning", Label: "Pending"}
	case p == "failed":
		return StatusBadge{Tone: "critical", Label: "Failed"}
	case p == "succeeded":
		return StatusBadge{Tone: "healthy", Label: "Succeeded"}
	default:
		if ready {
			return StatusBadge{Tone: "healthy", Label: "Ready"}
		}
		if phase != "" {
			return StatusBadge{Tone: "unknown", Label: phase}
		}
		return StatusBadge{Tone: "unknown", Label: "Unknown"}
	}
}
