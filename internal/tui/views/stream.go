package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

const (
	streamTimeW = 8
	streamTypeW = 5
	streamObjW  = 18
)

// StreamInnerRows is scrollable content height inside the stream panel border.
func StreamInnerRows(panelHeight int) int {
	rows := panelHeight - 2
	if rows < 1 {
		rows = 1
	}
	return rows
}

// StreamLineCount returns rendered line count for the live stream body.
func StreamLineCount(st model.InvestigationState, mode model.StreamViewMode, search string, width int) int {
	return len(streamBodyLines(st, mode, search, width))
}

// ClampStreamScroll bounds the stream scroll offset to valid content.
func ClampStreamScroll(scroll, panelHeight, width int, st model.InvestigationState, mode model.StreamViewMode, search string) int {
	return ClampScroll(scroll, StreamLineCount(st, mode, search, width), StreamInnerRows(panelHeight))
}

const liveEvidencePanelTitle = "Kubernetes Live Evidence Window"

// LiveStream renders the persistent bottom evidence panel in a fixed-height viewport.
func LiveStream(st model.InvestigationState, mode model.StreamViewMode, search string, scroll, height, width int, focused, follow bool) string {
	rows := StreamInnerRows(height)
	lines := streamBodyLines(st, mode, search, width)
	if len(lines) == 0 {
		lines = append(lines, dimStyle.Render("waiting for live evidence…"))
	}
	scroll = ClampScroll(scroll, len(lines), rows)
	body := paginate(strings.Join(lines, "\n"), scroll, rows)

	q := strings.ToLower(strings.TrimSpace(search))
	filter := "all"
	if q != "" {
		filter = strconvQuote(q)
	}
	prefix := " "
	if focused {
		prefix = "▸ "
	}
	title := fmt.Sprintf("%s◈ %s · %s · filter=%s", prefix, liveEvidencePanelTitle, streamModeLabel(mode), filter)
	if follow {
		title += " · LIVE"
	} else {
		title += " · history"
	}
	if st.DroppedEvidence > 0 {
		title += fmt.Sprintf(" · dropped=%d", st.DroppedEvidence)
	}
	if focused {
		title += " · j/k scroll"
	}
	return PanelH(title, width, rows, body)
}

func streamModeLabel(mode model.StreamViewMode) string {
	switch mode {
	case model.StreamRawLogs:
		return "raw"
	case model.StreamK8sEvents:
		return "events"
	case model.StreamErrorsOnly:
		return "errors"
	default:
		return "ranked"
	}
}

func streamBodyLines(st model.InvestigationState, mode model.StreamViewMode, search string, width int) []string {
	evs := filterStream(st.LiveEvidence, mode)
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].Timestamp.After(evs[j].Timestamp) })
	q := strings.ToLower(strings.TrimSpace(search))
	runs := foldStreamRuns(evs)
	return buildGroupedStreamLines(runs, width, st, q)
}

func strconvQuote(s string) string {
	if s == "" {
		return `""`
	}
	return `"` + s + `"`
}

type streamGroup struct {
	key  string
	kind string
	rows []string
}

func buildGroupedStreamLines(runs []streamRun, width int, st model.InvestigationState, q string) []string {
	var groups []streamGroup
	var cur *streamGroup

	for _, r := range runs {
		e := r.ev
		typ := streamType(e)
		obj := streamObject(e)
		msg := streamMessage(e)
		if q != "" {
			hay := strings.ToLower(typ + " " + obj + " " + msg)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		count := r.count
		if e.Count > count {
			count = e.Count
		}
		if count > 1 && e.SourceType != model.SourceLog {
			msg += fmt.Sprintf(" ×%d", count)
		}

		groupKey := streamGroupKey(e, obj)
		if cur == nil || cur.key != groupKey {
			if cur != nil {
				groups = append(groups, *cur)
			}
			cur = &streamGroup{key: groupKey, kind: typ, rows: []string{renderStreamRow(e, typ, obj, msg, width, st)}}
			continue
		}
		cur.rows = append(cur.rows, renderStreamRow(e, typ, obj, msg, width, st))
	}
	if cur != nil {
		groups = append(groups, *cur)
	}

	var lines []string
	for i, g := range groups {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, streamSourceHeader(g.key, g.kind, width))
		lines = append(lines, g.rows...)
	}
	return lines
}

func streamGroupKey(e model.EvidenceEvent, obj string) string {
	if isHypothesisStreamEvent(e) {
		return "klew/reasoning"
	}
	if e.SourceType == model.SourceLog && obj != "" {
		return obj
	}
	if obj != "" {
		return streamTypePlain(e) + "/" + obj
	}
	return streamTypePlain(e) + "/system"
}

type streamRun struct {
	ev    model.EvidenceEvent
	count int
	last  time.Time
}

func foldStreamRuns(evs []model.EvidenceEvent) []streamRun {
	var runs []streamRun
	for _, e := range evs {
		// Log lines are never folded — each line must remain scrollable in the tail view.
		if len(runs) > 0 && !isHypothesisStreamEvent(e) && e.SourceType != model.SourceLog {
			last := &runs[len(runs)-1]
			if streamFoldKey(last.ev) == streamFoldKey(e) {
				last.count++
				if e.Timestamp.After(last.last) {
					last.last = e.Timestamp
				}
				continue
			}
		}
		runs = append(runs, streamRun{ev: e, count: 1, last: e.Timestamp})
	}
	return runs
}

func streamFoldKey(e model.EvidenceEvent) string {
	msg := strings.TrimSpace(firstNonEmpty(e.Reason+" "+plainMsg(e), plainMsg(e)))
	msg = strings.Join(strings.Fields(msg), " ")
	return strings.ToLower(streamTypePlain(e) + "|" + streamObject(e) + "|" + msg)
}

func renderStreamRow(e model.EvidenceEvent, typ, obj, msg string, width int, st model.InvestigationState) string {
	ts := e.Timestamp.Format("15:04:05")
	if isFreshEvidence(e, st) {
		ts = freshStyle.Render(ts)
	} else {
		ts = dimStyle.Render(ts)
	}

	marker := sevMark(e.Severity)
	if isHypothesisStreamEvent(e) {
		marker = klewStyle.Render("★")
		typ = "KLEW"
	}

	msgW := maxInt(10, width-streamTimeW-streamTypeW-streamObjW-8)
	row := fmt.Sprintf("%s  %s  %s  %s %s",
		padRight(ts, streamTimeW),
		padRight(typ, streamTypeW),
		padRight(truncVisual(obj, streamObjW), streamObjW),
		marker,
		truncVisual(msg, msgW))

	if isHypothesisStreamEvent(e) {
		return klewStyle.Render(row)
	}
	if isFreshEvidence(e, st) {
		return freshStyle.Render(row)
	}
	return colorRow(row, e.Severity)
}

func isFreshEvidence(e model.EvidenceEvent, st model.InvestigationState) bool {
	ref := st.LastUpdatedAt
	if !st.Counters.LastEventAt.IsZero() {
		ref = st.Counters.LastEventAt
	}
	if ref.IsZero() {
		return false
	}
	age := ref.Sub(e.Timestamp)
	return age >= 0 && age <= 12*time.Second
}

func isHypothesisStreamEvent(e model.EvidenceEvent) bool {
	if e.SourceType != model.SourceSystem {
		return false
	}
	s := strings.ToLower(e.Reason + " " + e.Message)
	return strings.Contains(s, "hypothesis")
}

func streamMessage(e model.EvidenceEvent) string {
	if isHypothesisStreamEvent(e) {
		if _, _, cf, ct, ok := parseHypoChange(e.Message); ok && cf != "" {
			return strings.TrimSpace(firstNonEmpty(e.Reason, "Hypothesis changed") + " · confidence " + cf + " → " + ct)
		}
	}
	return strings.TrimSpace(firstNonEmpty(e.Reason+" "+plainMsg(e), plainMsg(e)))
}

func filterStream(events []model.EvidenceEvent, mode model.StreamViewMode) []model.EvidenceEvent {
	var out []model.EvidenceEvent
	for _, e := range events {
		switch mode {
		case model.StreamRawLogs:
			if e.SourceType != model.SourceLog {
				continue
			}
		case model.StreamK8sEvents:
			if e.SourceType != model.SourceK8sEvent {
				continue
			}
		case model.StreamErrorsOnly:
			if e.Severity != model.SeverityHigh && e.Severity != model.SeverityCritical {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func streamType(e model.EvidenceEvent) string {
	return streamTypePlain(e)
}

func streamTypePlain(e model.EvidenceEvent) string {
	if isHypothesisStreamEvent(e) {
		return "KLEW"
	}
	switch e.SourceType {
	case model.SourceK8sEvent:
		return "EVENT"
	case model.SourceLog:
		return "LOG"
	case model.SourceObjectChange:
		return "OBJ"
	case model.SourceMetric:
		return "METRIC"
	default:
		return "SYS"
	}
}

func streamObject(e model.EvidenceEvent) string {
	if e.SourceType == model.SourceLog && e.Pod != "" {
		if e.Container != "" {
			return e.Pod + "/" + e.Container
		}
		return e.Pod
	}
	kind := kindAbbrev(e.SourceKind)
	name := firstNonEmpty(e.SourceName, e.Pod)
	if kind == "" {
		return name
	}
	return kind + "/" + name
}

func kindAbbrev(kind string) string {
	switch kind {
	case "Service":
		return "svc"
	case "Deployment":
		return "deploy"
	case "ReplicaSet":
		return "rs"
	case "Pod":
		return "pod"
	case "Ingress":
		return "ing"
	case "":
		return ""
	default:
		return strings.ToLower(kind)
	}
}

func plainMsg(e model.EvidenceEvent) string {
	m := e.Message
	if e.Pod != "" {
		m = strings.TrimPrefix(m, e.Pod+"/"+e.Container+": ")
		m = strings.TrimPrefix(m, e.Pod+": ")
	}
	return m
}
