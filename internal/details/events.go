package details

import (
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func eventsSection(req *Request) Section {
	rows := collectEventRows(req)
	if len(rows) == 0 {
		return Section{}
	}
	return sectionTable("events", "Events", GroupEvents,
		[]string{"Time", "Type", "Reason", "Message", "Count"}, rows)
}

func collectEventRows(req *Request) [][]string {
	if req == nil {
		return nil
	}
	kind := req.Ref.Kind
	name := req.Ref.Name
	var rows [][]string
	seen := map[string]bool{}

	push := func(ts time.Time, typ, reason, msg string, count int32) {
		key := typ + "|" + reason + "|" + msg
		if seen[key] {
			return
		}
		seen[key] = true
		t := "—"
		if !ts.IsZero() {
			t = ts.UTC().Format("15:04:05")
		}
		c := ""
		if count > 1 {
			c = fmtInt32(count)
		}
		rows = append(rows, []string{t, typ, reason, truncate(msg, 160), c})
	}

	for _, e := range req.Snapshot.Events {
		obj := e.InvolvedObject
		if !objectTouches(obj.Kind, obj.Name, kind, name) {
			continue
		}
		push(e.Timestamp.Time(), e.Type, e.Reason, e.Message, e.Count)
	}
	for _, e := range req.State.LiveEvidence {
		t := strings.ToLower(string(e.SourceType))
		if t != "k8s_event" && t != "object_change" {
			continue
		}
		if !evidenceTouches(e, kind, name) {
			continue
		}
		push(e.Timestamp.Time(), string(e.SourceType), e.Reason, e.Message, int32(e.Count))
	}
	if len(rows) > 40 {
		rows = rows[:40]
	}
	return rows
}

func objectTouches(objKind, objName, kind, name string) bool {
	if objName == "" {
		return false
	}
	if objName == name {
		if objKind == "" || objKind == kind {
			return true
		}
		return kindAliases(objKind, kind)
	}
	// Workload → pod prefix
	if isWorkload(kind) && (objKind == "Pod" || objKind == "") && strings.HasPrefix(objName, name+"-") {
		return true
	}
	return false
}

func evidenceTouches(e model.EvidenceEvent, kind, name string) bool {
	if e.SourceKind != "" && e.SourceName != "" && objectTouches(e.SourceKind, e.SourceName, kind, name) {
		return true
	}
	if kind == "Pod" && (e.Pod == name || e.SourceName == name) {
		return true
	}
	if isWorkload(kind) && e.Pod != "" && strings.HasPrefix(e.Pod, name+"-") {
		return true
	}
	for _, ref := range e.RelatedObjectRefs {
		if objectTouches(ref.Kind, ref.Name, kind, name) {
			return true
		}
	}
	return false
}

func isWorkload(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
		return true
	default:
		return false
	}
}

func kindAliases(a, b string) bool {
	norm := func(k string) string {
		switch strings.ToLower(k) {
		case "pvc", "persistentvolumeclaim":
			return "PersistentVolumeClaim"
		case "pv", "persistentvolume":
			return "PersistentVolume"
		case "hpa", "horizontalpodautoscaler":
			return "HorizontalPodAutoscaler"
		case "sc", "storageclass":
			return "StorageClass"
		default:
			return k
		}
	}
	return norm(a) == norm(b)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
