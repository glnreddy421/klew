package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/service"
)

const (
	defaultObserveSeconds = 10
	maxObserveSeconds     = 60
	// collectTimeout bounds the initial snapshot (pods, events, logs, changes).
	collectTimeout = 2 * time.Minute
	changesTimeout = 20 * time.Second
	eventsTimeout  = 30 * time.Second
)

func readOnly(title string, openWorld bool) *mcp.ToolAnnotations {
	f := false
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, DestructiveHint: &f, IdempotentHint: true, OpenWorldHint: &openWorld}
}

// schemaFor infers T's input schema, then applies per-property constraints
// that struct tags cannot express (enums, bounds, defaults).
func schemaFor[T any](props map[string]func(*jsonschema.Schema)) *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	for name, f := range props {
		p, ok := s.Properties[name]
		if !ok {
			panic("schemaFor: unknown property " + name)
		}
		f(p)
	}
	return s
}

func intRange(min, max, def int) func(*jsonschema.Schema) {
	return func(p *jsonschema.Schema) {
		lo, hi := float64(min), float64(max)
		p.Minimum, p.Maximum = &lo, &hi
		p.Default = []byte(fmt.Sprint(def))
	}
}

func enum(values ...string) func(*jsonschema.Schema) {
	return func(p *jsonschema.Schema) {
		for _, v := range values {
			p.Enum = append(p.Enum, v)
		}
	}
}

func clamp(v, min, max, def int) int {
	if v <= 0 {
		v = def
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v
}

func (s *Server) registerTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "klew_list_contexts",
		Description: "List kube contexts from the local kubeconfig (name, cluster, default namespace, which is current). Reads the kubeconfig file only; makes no cluster calls.",
		Annotations: readOnly("List kube contexts", false),
	}, s.listContexts)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_investigate",
		Description: "Investigate a Kubernetes namespace or workload and return a verdict: status, leading signal, likely trigger, confidence, causal chain, " +
			"failing pods (exit codes, OOMKilled, limits), top log and event patterns, warning-event incidents, a related workload change, and next checks. " +
			"Collects a snapshot (pods, events, recent logs, revisions), then watches live for observe_seconds. " +
			"Returns an investigation_id for klew_investigation_details. For raw event lists use klew_event_history; for revision diffs use klew_workload_changes.",
		Annotations: readOnly("Investigate namespace or workload", true),
		InputSchema: schemaFor[investigateIn](map[string]func(*jsonschema.Schema){
			"observe_seconds": intRange(0, maxObserveSeconds, defaultObserveSeconds),
		}),
	}, s.investigate)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_investigation_details",
		Description: "Page through one section of an earlier klew_investigate result without collecting again. Sections: " +
			"pods (all pods, worst first), signals (every scored signal), log_patterns and event_patterns (mined templates with samples), " +
			"evidence_board (log templates linked to infrastructure events by time overlap), timeline (correlated incident timeline), " +
			"events (Kubernetes events in scope, newest first), live_evidence (evidence observed during the watch), matched_objects (objects the workload search matched). " +
			"Investigations expire after 30 minutes; run klew_investigate again after that.",
		Annotations: readOnly("Investigation details", false),
		InputSchema: schemaFor[detailsIn](map[string]func(*jsonschema.Schema){
			"section": enum(detailSections...),
			"limit":   intRange(1, 200, 25),
			"offset":  intRange(0, 100000, 0),
		}),
	}, s.investigationDetails)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_event_history",
		Description: "List Kubernetes events for a namespace or one object, newest first. Merges live events from the API with events the Klew desktop app " +
			"recorded locally, so it can include events Kubernetes already expired (the API keeps them about an hour). " +
			"Reports recording coverage and gaps. Without the desktop app's history only live events are returned.",
		Annotations: readOnly("Event history", true),
		InputSchema: schemaFor[eventHistoryIn](map[string]func(*jsonschema.Schema){
			"since_minutes": intRange(1, 43200, 1440),
			"limit":         intRange(1, 500, 100),
		}),
	}, s.eventHistory)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_workload_changes",
		Description: "Revision history of a Deployment, StatefulSet, DaemonSet or Helm release, newest first: time, source, actor when known, " +
			"summary, and field-level diffs (image, env, resources, probes, …). Use it to answer what changed before a failure.",
		Annotations: readOnly("Workload changes", true),
		InputSchema: schemaFor[workloadChangesIn](map[string]func(*jsonschema.Schema){
			"kind":          enum("Deployment", "StatefulSet", "DaemonSet", "HelmRelease"),
			"max_revisions": intRange(1, 50, 5),
		}),
	}, s.workloadChanges)
}

// ---- klew_list_contexts

type listContextsIn struct{}

type contextItem struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	User      string `json:"user,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Current   bool   `json:"current"`
}

type listContextsOut struct {
	Kubeconfig     string        `json:"kubeconfig"`
	CurrentContext string        `json:"current_context,omitempty"`
	DefaultContext string        `json:"default_context"`
	Contexts       []contextItem `json:"contexts"`
	// EventHistory is true when the desktop app's recorded events are readable.
	EventHistory bool `json:"event_history"`
}

func (s *Server) listContexts(_ context.Context, _ *mcp.CallToolRequest, _ listContextsIn) (*mcp.CallToolResult, listContextsOut, error) {
	cs, err := kube.LoadKubeConfigSnapshot(s.opts.Kubeconfig)
	if err != nil {
		return nil, listContextsOut{}, err
	}
	out := listContextsOut{
		Kubeconfig: cs.KubeconfigPath, CurrentContext: cs.CurrentContext, DefaultContext: cs.SelectedContext,
		EventHistory: s.history != nil, Contexts: make([]contextItem, 0, len(cs.Contexts)),
	}
	if s.opts.Context != "" {
		out.DefaultContext = s.opts.Context
	}
	for _, c := range cs.Contexts {
		out.Contexts = append(out.Contexts, contextItem{Name: c.Name, Cluster: c.Cluster, User: c.User, Namespace: c.Namespace, Current: c.IsCurrent})
	}
	return nil, out, nil
}

// ---- klew_investigate

type investigateIn struct {
	KubeContext    string `json:"kube_context,omitempty" jsonschema:"Kube context name from klew_list_contexts. Omit to use the server default (kubeconfig current-context)."`
	Namespace      string `json:"namespace" jsonschema:"Namespace to investigate. Klew investigates inside one namespace."`
	Workload       string `json:"workload,omitempty" jsonschema:"Workload name or search text, matched against names, labels and owners. Omit to investigate the whole namespace."`
	ObserveSeconds int    `json:"observe_seconds,omitempty" jsonschema:"Seconds to watch live events after the snapshot. Longer catches more restarts and warnings."`
}

type podCounts struct {
	Ready    int `json:"ready"`
	Unready  int `json:"unready"`
	Affected int `json:"affected"`
	Restarts int `json:"restarts"`
}

type investigateOut struct {
	InvestigationID   string              `json:"investigation_id"`
	Context           string              `json:"context"`
	Namespace         string              `json:"namespace"`
	Workload          string              `json:"workload,omitempty"`
	CollectedAt       model.Timestamp     `json:"collected_at"`
	Status            model.VerdictStatus `json:"status"`
	Confidence        float64             `json:"confidence"`
	ConfidenceLabel   string              `json:"confidence_label"`
	LeadingSignal     string              `json:"leading_signal,omitempty"`
	LikelyTrigger     string              `json:"likely_trigger,omitempty"`
	Summary           string              `json:"summary,omitempty"`
	Hypothesis        string              `json:"hypothesis,omitempty"`
	HypothesisReasons []string            `json:"hypothesis_reasons,omitempty"`
	Alternatives      []model.Hypothesis  `json:"alternatives,omitempty"`
	CausalChain       []string            `json:"causal_chain,omitempty"`
	NextChecks        []string            `json:"next_checks,omitempty"`
	SuggestedActions  []string            `json:"suggested_actions,omitempty"`
	Pods              podCounts           `json:"pods"`
	EndpointsReady    int                 `json:"endpoints_ready"`
	EndpointsTotal    int                 `json:"endpoints_total"`
	MatchedObjects    []string            `json:"matched_objects,omitempty"`
	FailingPods       []podBrief          `json:"failing_pods"`
	FailingPodsTotal  int                 `json:"failing_pods_total"`
	Signals           []signalBrief       `json:"signals"`
	SignalsTotal      int                 `json:"signals_total"`
	Incidents         []model.Incident    `json:"incidents,omitempty"`
	PinnedChange      *model.PinnedChange `json:"pinned_change,omitempty"`
	RecentChange      *model.RecentChange `json:"recent_change,omitempty"`
	LogPatterns       []patternBrief      `json:"log_patterns,omitempty"`
	EventPatterns     []patternBrief      `json:"event_patterns,omitempty"`
	EventHistory      *coverageBrief      `json:"event_history,omitempty"`
	Warnings          []string            `json:"warnings,omitempty"`
}

func (s *Server) investigate(ctx context.Context, _ *mcp.CallToolRequest, in investigateIn) (*mcp.CallToolResult, investigateOut, error) {
	ns := strings.TrimSpace(in.Namespace)
	if ns == "" {
		return nil, investigateOut{}, fmt.Errorf("namespace is required")
	}
	client, err := s.newClient(in.KubeContext, ns)
	if err != nil {
		return nil, investigateOut{}, err
	}
	observe := time.Duration(clamp(in.ObserveSeconds, 1, maxObserveSeconds, defaultObserveSeconds)) * time.Second
	ctx, cancel := context.WithTimeout(ctx, collectTimeout+observe)
	defer cancel()

	st, err := s.runFor(ctx, client, engine.LiveOptions{
		Query: strings.TrimSpace(in.Workload), Namespace: ns, Tail: 200, AutoRefresh: false,
	}, observe)
	if err != nil && st.CollectedAt.IsZero() {
		return nil, investigateOut{}, fmt.Errorf("investigate %s: %w", ns, err)
	}
	return nil, summarize(s.cache.put(st), st), nil
}

func summarize(id string, st model.InvestigationState) investigateOut {
	sum := engine.BuildIncidentSummary(st)
	v := st.Verdict
	out := investigateOut{
		InvestigationID: id, Context: sum.Context, Namespace: sum.Namespace, Workload: st.Query,
		CollectedAt: st.CollectedAt, Status: v.Status, Confidence: v.Confidence,
		ConfidenceLabel: engine.ConfidenceLabel(v.Confidence), LeadingSignal: v.LeadingSignal,
		LikelyTrigger: v.LikelyTrigger, Summary: v.Summary, Hypothesis: st.HypothesisLabel,
		HypothesisReasons: st.HypothesisReasons, Alternatives: st.HypothesisAlts, CausalChain: st.CausalChain,
		NextChecks: st.NextChecks, SuggestedActions: st.FixActions,
		Pods:           podCounts{Ready: sum.ReadyPods, Unready: sum.UnreadyPods, Affected: sum.AffectedPods, Restarts: sum.Restarts},
		EndpointsReady: sum.EndpointsReady, EndpointsTotal: sum.EndpointsTotal,
		Incidents: capSlice(st.Incidents, 5), PinnedChange: st.PinnedChange, RecentChange: st.RecentChange,
		EventHistory: briefCoverage(st.Snapshot.EventHistory),
	}
	if len(out.NextChecks) == 0 {
		out.NextChecks = v.RecommendedNextChecks
	}
	for _, m := range capSlice(st.MatchedObjects, 20) {
		out.MatchedObjects = append(out.MatchedObjects, refString(m.Ref))
	}
	failing := failingPods(st.Snapshot.Pods)
	out.FailingPodsTotal = len(failing)
	out.FailingPods = mapSlice(capSlice(failing, 10), briefPod)
	sigs := allSignals(v)
	out.SignalsTotal = len(sigs)
	out.Signals = mapSlice(capSlice(sigs, 8), briefSignal)
	if lp := st.LogPatterns; lp != nil {
		out.LogPatterns = mapSlice(capSlice(lp.Templates, 5), func(t model.LogTemplate) patternBrief { return briefPattern(t, 1) })
		out.EventPatterns = mapSlice(capSlice(lp.EventTemplates, 5), func(t model.LogTemplate) patternBrief { return briefPattern(t, 1) })
	}
	out.Warnings = append(append([]string{}, v.MissingDataWarnings...), st.Warnings...)
	if st.Query != "" && len(st.MatchedObjects) == 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("No objects matched %q in %s; the verdict covers the namespace.", st.Query, sum.Namespace))
	}
	return out
}

// ---- klew_investigation_details

var detailSections = []string{"pods", "signals", "log_patterns", "event_patterns", "evidence_board", "timeline", "events", "live_evidence", "matched_objects"}

type detailsIn struct {
	InvestigationID string `json:"investigation_id" jsonschema:"investigation_id returned by klew_investigate."`
	Section         string `json:"section" jsonschema:"Which section to return."`
	Limit           int    `json:"limit,omitempty" jsonschema:"Max items to return."`
	Offset          int    `json:"offset,omitempty" jsonschema:"Items to skip, for paging."`
}

type detailsOut struct {
	InvestigationID string `json:"investigation_id"`
	Section         string `json:"section"`
	Total           int    `json:"total"`
	Offset          int    `json:"offset"`
	Items           []any  `json:"items"`
	// More is true when items remain after this page.
	More bool `json:"more"`
}

func (s *Server) investigationDetails(_ context.Context, _ *mcp.CallToolRequest, in detailsIn) (*mcp.CallToolResult, detailsOut, error) {
	st, ok := s.cache.get(in.InvestigationID)
	if !ok {
		return nil, detailsOut{}, fmt.Errorf("investigation %q not found or expired; run klew_investigate again", in.InvestigationID)
	}
	items, err := detailItems(st, in.Section)
	if err != nil {
		return nil, detailsOut{}, err
	}
	limit := clamp(in.Limit, 1, 200, 25)
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}
	out := detailsOut{InvestigationID: in.InvestigationID, Section: in.Section, Total: len(items), Offset: offset, Items: []any{}}
	if offset < len(items) {
		end := min(offset+limit, len(items))
		out.Items = items[offset:end]
		out.More = end < len(items)
	}
	return nil, out, nil
}

func asAny[T any](in []T) []any {
	return mapSlice(in, func(v T) any { return v })
}

func detailItems(st model.InvestigationState, section string) ([]any, error) {
	lp := st.LogPatterns
	if lp == nil {
		lp = &model.LogPatterns{}
	}
	switch section {
	case "pods":
		return asAny(mapSlice(rankPods(st.Snapshot.Pods), briefPod)), nil
	case "signals":
		return asAny(mapSlice(allSignals(st.Verdict), briefSignal)), nil
	case "log_patterns":
		return asAny(mapSlice(lp.Templates, func(t model.LogTemplate) patternBrief { return briefPattern(t, 3) })), nil
	case "event_patterns":
		return asAny(mapSlice(lp.EventTemplates, func(t model.LogTemplate) patternBrief { return briefPattern(t, 3) })), nil
	case "evidence_board":
		if lp.EvidenceBoard == nil {
			return []any{}, nil
		}
		return asAny(lp.EvidenceBoard.Cards), nil
	case "timeline":
		return asAny(st.Timeline), nil
	case "events":
		evs := append([]model.EventRecord(nil), st.Snapshot.Events...)
		newestEventsFirst(evs)
		return asAny(mapSlice(evs, briefEvent)), nil
	case "live_evidence":
		return asAny(mapSlice(st.LiveEvidence, briefEvidence)), nil
	case "matched_objects":
		return asAny(st.MatchedObjects), nil
	}
	return nil, fmt.Errorf("unknown section %q; use one of: %s", section, strings.Join(detailSections, ", "))
}

// ---- klew_event_history

type eventHistoryIn struct {
	KubeContext  string `json:"kube_context,omitempty" jsonschema:"Kube context name from klew_list_contexts. Omit to use the server default (kubeconfig current-context)."`
	Namespace    string `json:"namespace" jsonschema:"Namespace whose events to list."`
	ObjectName   string `json:"object_name,omitempty" jsonschema:"Limit to events about this object. A workload name also matches its pods and ReplicaSets (<name>-*) unless exact_name is true."`
	ExactName    bool   `json:"exact_name,omitempty" jsonschema:"Match object_name exactly instead of also matching <name>-* children."`
	SinceMinutes int    `json:"since_minutes,omitempty" jsonschema:"How far back to look, in minutes."`
	WarningsOnly bool   `json:"warnings_only,omitempty" jsonschema:"Return only Warning events."`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max events to return."`
}

type eventHistoryOut struct {
	Context   string       `json:"context"`
	Namespace string       `json:"namespace"`
	Since     string       `json:"since"`
	Total     int          `json:"total"`
	Events    []eventBrief `json:"events"`
	Truncated bool         `json:"truncated"`
	// LiveEvents and RecordedEvents count matches by origin before truncation.
	LiveEvents     int            `json:"live_events"`
	RecordedEvents int            `json:"recorded_events"`
	Coverage       *coverageBrief `json:"coverage,omitempty"`
	Note           string         `json:"note,omitempty"`
}

func (s *Server) eventHistory(ctx context.Context, _ *mcp.CallToolRequest, in eventHistoryIn) (*mcp.CallToolResult, eventHistoryOut, error) {
	ns := strings.TrimSpace(in.Namespace)
	if ns == "" {
		return nil, eventHistoryOut{}, fmt.Errorf("namespace is required")
	}
	client, err := s.newClient(in.KubeContext, ns)
	if err != nil {
		return nil, eventHistoryOut{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, eventsTimeout)
	defer cancel()

	since := time.Now().UTC().Add(-time.Duration(clamp(in.SinceMinutes, 1, 43200, 1440)) * time.Minute)
	name := strings.TrimSpace(in.ObjectName)
	prefix := name != "" && !in.ExactName

	live, err := kube.ListEvents(ctx, client, ns)
	if err != nil {
		// Recorded events are withheld too: history must not show more than
		// the current credentials can see.
		if apierrors.IsForbidden(err) {
			return nil, eventHistoryOut{}, fmt.Errorf("no permission to list events in %s: %w", ns, err)
		}
		return nil, eventHistoryOut{}, err
	}

	out := eventHistoryOut{Context: client.Context, Namespace: ns, Since: since.Format(time.RFC3339), Events: []eventBrief{}}
	all := live
	if client.History != nil && client.Config != nil {
		recs, cov, herr := client.History.RecordedEvents(ctx, client.Context, client.Config.Host, model.EventHistoryQuery{
			Namespace: ns, Since: since, RegardingName: name, NamePrefix: prefix, Limit: 5000,
		})
		if herr != nil {
			out.Note = "Recorded event history unavailable: " + herr.Error()
		} else {
			all, _ = kube.MergeRecordedEvents(live, recs)
			if cov.Since != "" || len(cov.Gaps) > 0 {
				out.Coverage = briefCoverage(&cov)
			} else {
				out.Note = "Klew has not recorded this cluster; showing live events only (the API keeps events for about an hour)."
			}
		}
	} else {
		out.Note = "No local event history (recorded by the Klew desktop app); showing live events only (the API keeps events for about an hour)."
	}

	var matched []model.EventRecord
	for _, e := range all {
		if eventTime(e).Time().Before(since) {
			continue
		}
		if in.WarningsOnly && e.Type != "Warning" {
			continue
		}
		if name != "" {
			n := e.InvolvedObject.Name
			if n != name && !(prefix && strings.HasPrefix(n, name+"-")) {
				continue
			}
		}
		matched = append(matched, e)
		if e.Recorded {
			out.RecordedEvents++
		} else {
			out.LiveEvents++
		}
	}
	newestEventsFirst(matched)
	limit := clamp(in.Limit, 1, 500, 100)
	out.Total = len(matched)
	out.Truncated = len(matched) > limit
	out.Events = mapSlice(capSlice(matched, limit), briefEvent)
	return nil, out, nil
}

// ---- klew_workload_changes

type workloadChangesIn struct {
	KubeContext  string `json:"kube_context,omitempty" jsonschema:"Kube context name from klew_list_contexts. Omit to use the server default (kubeconfig current-context)."`
	Namespace    string `json:"namespace" jsonschema:"Namespace of the workload."`
	Kind         string `json:"kind" jsonschema:"Workload kind."`
	Name         string `json:"name" jsonschema:"Workload or Helm release name."`
	MaxRevisions int    `json:"max_revisions,omitempty" jsonschema:"Max revisions to return, newest first."`
	SummaryOnly  bool   `json:"summary_only,omitempty" jsonschema:"Omit field-level diffs and return one summary line per revision."`
}

type workloadChangesOut struct {
	model.WorkloadChanges
	TotalRevisions int `json:"total_revisions"`
}

func (s *Server) workloadChanges(ctx context.Context, _ *mcp.CallToolRequest, in workloadChangesIn) (*mcp.CallToolResult, workloadChangesOut, error) {
	switch in.Kind {
	case "Deployment", "StatefulSet", "DaemonSet", "HelmRelease":
	default:
		return nil, workloadChangesOut{}, fmt.Errorf("unsupported kind %q; use Deployment, StatefulSet, DaemonSet or HelmRelease", in.Kind)
	}
	ns, name := strings.TrimSpace(in.Namespace), strings.TrimSpace(in.Name)
	if ns == "" || name == "" {
		return nil, workloadChangesOut{}, fmt.Errorf("namespace and name are required")
	}
	client, err := s.newClient(in.KubeContext, ns)
	if err != nil {
		return nil, workloadChangesOut{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, changesTimeout)
	defer cancel()

	wc, err := service.WorkloadChanges(ctx, client, in.Kind, ns, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, workloadChangesOut{}, fmt.Errorf("%s %s/%s not found; check the name with klew_investigate matched_objects", in.Kind, ns, name)
		}
		return nil, workloadChangesOut{}, err
	}
	out := workloadChangesOut{WorkloadChanges: wc, TotalRevisions: len(wc.Changes)}
	out.Changes = append([]model.WorkloadChange(nil), capSlice(wc.Changes, clamp(in.MaxRevisions, 1, 50, 5))...)
	if in.SummaryOnly {
		for i := range out.Changes {
			out.Changes[i].Diffs = nil
		}
	}
	return nil, out, nil
}
