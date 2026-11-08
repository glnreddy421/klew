package mcpserver

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/redact"
	"github.com/glnreddy421/klew/internal/service"
)

const (
	defaultObserveSeconds = 10
	maxObserveSeconds     = 60
	// maxConcurrentInvestigations caps cluster load from parallel tool calls.
	maxConcurrentInvestigations = 3
	// progressInterval spaces progress notifications during klew_investigate.
	progressInterval = 5 * time.Second
	// maxDiffsPerRevision caps field diffs per revision in klew_workload_changes.
	maxDiffsPerRevision = 30
	// collectTimeout bounds the initial snapshot (pods, events, logs, changes).
	collectTimeout = 2 * time.Minute
	changesTimeout = 20 * time.Second
	// namespaceCheckTimeout bounds the existence check before a tool runs.
	namespaceCheckTimeout = 5 * time.Second
	eventsTimeout         = 30 * time.Second
	// maxRecordedEvents caps rows read from local history per call.
	maxRecordedEvents = 5000
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

// Kubernetes name rules: namespaces are DNS labels, workloads DNS subdomains.
const (
	namespacePattern  = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	objectNamePattern = `^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
)

var validNamespace = regexp.MustCompile(namespacePattern)

// parseNamespace validates a namespace here rather than in the input schema:
// the SDK reports schema failures as raw regex errors, which give an assistant
// that passed a list or wildcard nothing to act on.
func parseNamespace(in string) (string, error) {
	ns := strings.TrimSpace(in)
	if ns == "" {
		return "", fmt.Errorf("namespace is required")
	}
	if len(ns) > 63 || !validNamespace.MatchString(ns) {
		return "", fmt.Errorf("invalid namespace %q: pass one Kubernetes namespace name, not a list or wildcard. Tools cover one namespace per call; to check several, call once for each", clean(in))
	}
	return ns, nil
}

// namespaceMissing reports whether the API says ns does not exist. Any other
// result, such as no permission to get namespaces, is not treated as missing.
func namespaceMissing(ctx context.Context, client *kube.Client, ns string) bool {
	ctx, cancel := context.WithTimeout(ctx, namespaceCheckTimeout)
	defer cancel()
	_, err := client.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	return apierrors.IsNotFound(err)
}

func errNamespaceNotFound(ns, kubeContext string) error {
	return fmt.Errorf("namespace %q not found in context %q; check the name. Tools cover one namespace per call", ns, kubeContext)
}

func pattern(re string, maxLen int) func(*jsonschema.Schema) {
	return func(p *jsonschema.Schema) {
		p.Pattern = re
		p.MaxLength = &maxLen
	}
}

func enum(values ...string) func(*jsonschema.Schema) {
	return func(p *jsonschema.Schema) {
		for _, v := range values {
			p.Enum = append(p.Enum, v)
		}
	}
}

// clamp returns def for unset (<= 0) values and caps v at max.
func clamp(v, max, def int) int {
	if v <= 0 {
		v = def
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
			"problems (every unhealthy workload or Service with its state and key facts: exit reason, limits, probe, ports, requests, image, mounts), " +
			"failing pods, top log and event patterns, warning-event incidents, a related workload change, and next checks. " +
			"The leading signal and suggested actions follow the strongest problem only; read problems for the full list. " +
			"not_collected lists collectors that were skipped (no permission, logs off) or failed; treat what they cover as unchecked, not healthy. " +
			"Collects a snapshot (pods, events, recent logs, revisions), then watches live for observe_seconds; 0 skips the watch for a fast snapshot-only verdict. " +
			"Covers one namespace per call; to check several, call once for each. " +
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
			"investigation_id": pattern(`^inv_[0-9a-f]{12}$`, 16),
			"section":          enum(detailSections...),
			"limit":            intRange(1, 200, 25),
			"offset":           intRange(0, 100000, 0),
		}),
	}, s.investigationDetails)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "klew_event_history",
		Description: "List Kubernetes events for a namespace or one object, newest first. Merges live events from the API with events the Klew desktop app " +
			"recorded locally, so it can include events Kubernetes already expired (the API keeps them about an hour). " +
			"Filter by object_name, object_kind (all Node or PersistentVolumeClaim events), reason (BackOff, FailedScheduling, …) or warnings_only. " +
			"Reports recording coverage and gaps. Without the desktop app's history only live events are returned.",
		Annotations: readOnly("Event history", true),
		InputSchema: schemaFor[eventHistoryIn](map[string]func(*jsonschema.Schema){
			"object_name":   pattern(objectNamePattern, 253),
			"reason":        pattern(`^[A-Za-z0-9, ]*$`, 256),
			"object_kind":   pattern(`^[A-Za-z0-9]*$`, 63),
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
			"name":          pattern(objectNamePattern, 253),
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
		EventHistory: s.recordedHistory() != nil, Contexts: make([]contextItem, 0, len(cs.Contexts)),
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
	Namespace      string `json:"namespace" jsonschema:"One namespace name to investigate. Lists and wildcards are rejected; to cover several namespaces, call once for each."`
	Workload       string `json:"workload,omitempty" jsonschema:"What to investigate: Kind/name for exactly one object (Deployment/api, Service/web, ConfigMap/settings, PersistentVolumeClaim/data, or any kind a collector lists), several separated by commas, or a bare name for every object with exactly that name. Text that names no object is matched loosely against names. Omit to investigate the whole namespace."`
	ObserveSeconds *int   `json:"observe_seconds,omitempty" jsonschema:"Seconds to watch live events after the snapshot. Longer catches more restarts and warnings; 0 returns the snapshot verdict without watching."`
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
	Headline          string              `json:"headline,omitempty"`
	Problems          []problemBrief      `json:"problems,omitempty"`
	ProblemsTotal     int                 `json:"problems_total,omitempty"`
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
	// NotCollected lists collectors that did not run or failed, and why, so the
	// verdict is read knowing what was not checked.
	NotCollected []model.CollectorStatus `json:"not_collected,omitempty"`
	Warnings     []string                `json:"warnings,omitempty"`
}

// observeFor is the watch length: the default when unset, 0 for snapshot only.
func observeFor(seconds *int) time.Duration {
	if seconds == nil {
		return defaultObserveSeconds * time.Second
	}
	return time.Duration(max(min(*seconds, maxObserveSeconds), 0)) * time.Second
}

// reportProgress sends periodic progress notifications when the client asked
// for them, so hosts that reset their timeout on progress keep waiting. The
// returned func stops them; none is sent after it returns.
func (s *Server) reportProgress(ctx context.Context, req *mcp.CallToolRequest, message string) func() {
	token := req.Params.GetProgressToken()
	if token == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		start := time.Now()
		tick := time.NewTicker(s.progressEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				elapsed := time.Since(start)
				_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
					ProgressToken: token, Progress: elapsed.Seconds(),
					Message: fmt.Sprintf("%s: %s elapsed", message, elapsed.Round(time.Second)),
				})
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (s *Server) investigate(ctx context.Context, req *mcp.CallToolRequest, in investigateIn) (*mcp.CallToolResult, investigateOut, error) {
	ns, err := parseNamespace(in.Namespace)
	if err != nil {
		return nil, investigateOut{}, err
	}
	client, err := s.newClient(in.KubeContext, ns)
	if err != nil {
		return nil, investigateOut{}, err
	}
	if namespaceMissing(ctx, client, ns) {
		return nil, investigateOut{}, errNamespaceNotFound(ns, client.Context)
	}
	defer s.reportProgress(ctx, req, "Investigating "+ns)()
	select {
	case s.investigations <- struct{}{}:
		defer func() { <-s.investigations }()
	case <-ctx.Done():
		return nil, investigateOut{}, ctx.Err()
	}
	observe := observeFor(in.ObserveSeconds)
	ctx, cancel := context.WithTimeout(ctx, collectTimeout+observe)
	defer cancel()

	st, err := s.runFor(ctx, client, engine.LiveOptions{
		Query: strings.TrimSpace(in.Workload), Namespace: ns, Tail: 200, AutoRefresh: false,
	}, observe)
	if err != nil && st.CollectedAt.IsZero() {
		return nil, investigateOut{}, fmt.Errorf("investigate %s: %w", ns, err)
	}
	out := s.shape.summarize(s.cache.put(st), st)
	if err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Investigation incomplete: %s. Results may be partial.", clean(err.Error())))
	}
	return nil, out, nil
}

func (sh shaper) summarize(id string, st model.InvestigationState) investigateOut {
	sum := engine.BuildIncidentSummary(st)
	v := st.Verdict
	out := investigateOut{
		InvestigationID: id, Context: sum.Context, Namespace: sum.Namespace, Workload: st.Query,
		CollectedAt: st.CollectedAt, Status: v.Status, Confidence: v.Confidence,
		ConfidenceLabel: engine.ConfidenceLabel(v.Confidence), LeadingSignal: v.LeadingSignal,
		LikelyTrigger: clean(v.LikelyTrigger), Summary: clean(v.Summary), Headline: clean(st.Headline), Hypothesis: clean(st.HypothesisLabel),
		HypothesisReasons: redact.Strings(st.HypothesisReasons), Alternatives: st.HypothesisAlts,
		CausalChain: sh.causalChain(st.CausalChain, st.Timeline),
		NextChecks:  redact.Strings(st.NextChecks), SuggestedActions: redact.Strings(st.FixActions),
		Pods:           podCounts{Ready: sum.ReadyPods, Unready: sum.UnreadyPods, Affected: sum.AffectedPods, Restarts: sum.Restarts},
		EndpointsReady: sum.EndpointsReady, EndpointsTotal: sum.EndpointsTotal,
		Incidents: capSlice(st.Incidents, 5), PinnedChange: st.PinnedChange, RecentChange: st.RecentChange,
		EventHistory: briefCoverage(st.Snapshot.EventHistory),
	}
	if len(out.NextChecks) == 0 {
		out.NextChecks = redact.Strings(v.RecommendedNextChecks)
	}
	for _, m := range capSlice(st.MatchedObjects, 20) {
		out.MatchedObjects = append(out.MatchedObjects, refString(m.Ref))
	}
	out.ProblemsTotal = len(st.Problems)
	out.Problems = mapSlice(capSlice(st.Problems, maxProblems), briefProblem)
	failing := failingPods(st.Snapshot.Pods)
	out.FailingPodsTotal = len(failing)
	out.FailingPods = mapSlice(capSlice(failing, 10), sh.pod)
	sigs := allSignals(v)
	out.SignalsTotal = len(sigs)
	out.Signals = mapSlice(capSlice(sigs, 8), sh.signal)
	if lp := st.LogPatterns; lp != nil {
		out.LogPatterns = mapSlice(capSlice(lp.Templates, 5), func(t model.LogTemplate) patternBrief { return sh.pattern(t, 1) })
		out.EventPatterns = mapSlice(capSlice(lp.EventTemplates, 5), func(t model.LogTemplate) patternBrief { return briefPattern(t, 1) })
	}
	if pc := st.PinnedChange; pc != nil {
		cp := *pc
		cp.Text = clean(cp.Text)
		out.PinnedChange = &cp
	}
	for _, c := range st.Snapshot.Coverage {
		if c.Status != "collected" && c.Status != "not-installed" {
			c.Detail = clean(c.Detail)
			out.NotCollected = append(out.NotCollected, c)
		}
	}
	out.Warnings = briefWarnings(v.MissingDataWarnings, st.Warnings)
	if st.Query != "" && len(st.MatchedObjects) == 0 && st.Snapshot.Target == nil {
		out.Warnings = append(out.Warnings, clean(fmt.Sprintf("No objects matched %q in %s; the verdict covers the namespace.", st.Query, sum.Namespace)))
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
	items, err := s.shape.detailItems(st, in.Section)
	if err != nil {
		return nil, detailsOut{}, err
	}
	limit := clamp(in.Limit, 200, 25)
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

func (sh shaper) detailItems(st model.InvestigationState, section string) ([]any, error) {
	lp := st.LogPatterns
	if lp == nil {
		lp = &model.LogPatterns{}
	}
	switch section {
	case "pods":
		return asAny(mapSlice(rankPods(st.Snapshot.Pods), sh.pod)), nil
	case "signals":
		return asAny(mapSlice(allSignals(st.Verdict), sh.signal)), nil
	case "log_patterns":
		return asAny(mapSlice(lp.Templates, func(t model.LogTemplate) patternBrief { return sh.pattern(t, 3) })), nil
	case "event_patterns":
		return asAny(mapSlice(lp.EventTemplates, func(t model.LogTemplate) patternBrief { return briefPattern(t, 3) })), nil
	case "evidence_board":
		if lp.EvidenceBoard == nil {
			return []any{}, nil
		}
		return asAny(mapSlice(lp.EvidenceBoard.Cards, sh.card)), nil
	case "timeline":
		return asAny(mapSlice(st.Timeline, sh.timeline)), nil
	case "events":
		evs := append([]model.EventRecord(nil), st.Snapshot.Events...)
		newestEventsFirst(evs)
		return asAny(mapSlice(evs, briefEvent)), nil
	case "live_evidence":
		return asAny(mapSlice(st.LiveEvidence, sh.evidence)), nil
	case "matched_objects":
		return asAny(st.MatchedObjects), nil
	}
	return nil, fmt.Errorf("unknown section %q; use one of: %s", section, strings.Join(detailSections, ", "))
}

// ---- klew_event_history

type eventHistoryIn struct {
	KubeContext  string `json:"kube_context,omitempty" jsonschema:"Kube context name from klew_list_contexts. Omit to use the server default (kubeconfig current-context)."`
	Namespace    string `json:"namespace" jsonschema:"One namespace name whose events to list. To cover several, call once for each."`
	ObjectName   string `json:"object_name,omitempty" jsonschema:"Limit to events about this object. A workload name also matches its pods and ReplicaSets (<name>-*) unless exact_name is true."`
	ExactName    bool   `json:"exact_name,omitempty" jsonschema:"Match object_name exactly instead of also matching <name>-* children."`
	SinceMinutes int    `json:"since_minutes,omitempty" jsonschema:"How far back to look, in minutes."`
	WarningsOnly bool   `json:"warnings_only,omitempty" jsonschema:"Return only Warning events."`
	Reason       string `json:"reason,omitempty" jsonschema:"Only events with this reason, or any of several separated by commas (BackOff, FailedScheduling, OOMKilling, Unhealthy, FailedMount). Case is ignored."`
	ObjectKind   string `json:"object_kind,omitempty" jsonschema:"Only events about objects of this kind (Pod, Node, PersistentVolumeClaim, Deployment, …). Case is ignored."`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max events to return."`
}

type eventHistoryOut struct {
	Context   string       `json:"context"`
	Namespace string       `json:"namespace"`
	Since     string       `json:"since"`
	Total     int          `json:"total"`
	Events    []eventBrief `json:"events"`
	// Truncated is true when matches exist beyond Events, including when the
	// recorded-event cap was hit and Total is only a lower bound.
	Truncated bool `json:"truncated"`
	// LiveEvents and RecordedEvents count matches by origin before truncation.
	LiveEvents     int            `json:"live_events"`
	RecordedEvents int            `json:"recorded_events"`
	Coverage       *coverageBrief `json:"coverage,omitempty"`
	Note           string         `json:"note,omitempty"`
}

func (s *Server) eventHistory(ctx context.Context, _ *mcp.CallToolRequest, in eventHistoryIn) (*mcp.CallToolResult, eventHistoryOut, error) {
	ns, err := parseNamespace(in.Namespace)
	if err != nil {
		return nil, eventHistoryOut{}, err
	}
	client, err := s.newClient(in.KubeContext, ns)
	if err != nil {
		return nil, eventHistoryOut{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, eventsTimeout)
	defer cancel()

	since := time.Now().UTC().Add(-time.Duration(clamp(in.SinceMinutes, 43200, 1440)) * time.Minute)
	name := strings.TrimSpace(in.ObjectName)
	prefix := name != "" && !in.ExactName
	reasons := splitList(in.Reason)
	kind := strings.TrimSpace(in.ObjectKind)

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
	capped := false
	if client.History != nil && client.Config != nil {
		q := model.EventHistoryQuery{Namespace: ns, Since: since, RegardingName: name, NamePrefix: prefix, Reasons: reasons, RegardingKind: kind, Limit: maxRecordedEvents}
		if in.WarningsOnly {
			q.Type = "Warning"
		}
		recs, cov, herr := client.History.RecordedEvents(ctx, client.Context, client.Config.Host, q)
		if herr != nil {
			out.Note = "Recorded event history unavailable: " + herr.Error()
		} else {
			all, _ = kube.MergeRecordedEvents(live, recs)
			capped = len(recs) >= maxRecordedEvents
			switch {
			case cov.Since != "" || len(cov.Gaps) > 0:
				out.Coverage = briefCoverage(&cov)
			case cov.LastRecorded != "":
				out.Note = fmt.Sprintf("Klew's recording of this cluster ended at %s, before this window; showing live events only. Increase since_minutes to include recorded events.", cov.LastRecorded)
			default:
				out.Note = "Klew has not recorded this cluster; showing live events only (the API keeps events for about an hour)."
			}
		}
	} else {
		out.Note = "No local event history (recorded by the Klew desktop app); showing live events only (the API keeps events for about an hour)."
	}

	// Not an error: recorded events can outlive a deleted namespace.
	if namespaceMissing(ctx, client, ns) {
		out.Note = strings.TrimSpace(fmt.Sprintf("Namespace %q does not exist in this cluster, so only recorded events can appear; check the name. %s", ns, out.Note))
	}

	var matched []model.EventRecord
	for _, e := range all {
		if eventTime(e).Time().Before(since) {
			continue
		}
		if in.WarningsOnly && e.Type != "Warning" {
			continue
		}
		if name != "" && !nameMatches(e.InvolvedObject.Name, name, prefix) {
			continue
		}
		if kind != "" && !strings.EqualFold(e.InvolvedObject.Kind, kind) {
			continue
		}
		if len(reasons) > 0 && !containsFold(reasons, e.Reason) {
			continue
		}
		matched = append(matched, e)
		if e.Recorded {
			out.RecordedEvents++
		} else {
			out.LiveEvents++
		}
	}
	newestEventsFirst(matched)
	limit := clamp(in.Limit, 500, 100)
	out.Total = len(matched)
	out.Truncated = len(matched) > limit || capped
	if capped {
		out.Note = strings.TrimSpace(out.Note + fmt.Sprintf(" Read only the newest %d recorded events: total is a lower bound and older events are missing. Narrow since_minutes or object_name.", maxRecordedEvents))
	}
	out.Events = mapSlice(capSlice(matched, limit), briefEvent)
	return nil, out, nil
}

// splitList splits a comma-separated value, dropping blanks.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func containsFold(list []string, v string) bool {
	for _, s := range list {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

// nameMatches reports whether obj is name, or with prefix one of its <name>-* children.
func nameMatches(obj, name string, prefix bool) bool {
	return obj == name || (prefix && strings.HasPrefix(obj, name+"-"))
}

// ---- klew_workload_changes

type workloadChangesIn struct {
	KubeContext  string `json:"kube_context,omitempty" jsonschema:"Kube context name from klew_list_contexts. Omit to use the server default (kubeconfig current-context)."`
	Namespace    string `json:"namespace" jsonschema:"One namespace name: the workload's namespace."`
	Kind         string `json:"kind" jsonschema:"Workload kind."`
	Name         string `json:"name" jsonschema:"Workload or Helm release name."`
	MaxRevisions int    `json:"max_revisions,omitempty" jsonschema:"Max revisions to return, newest first."`
	SummaryOnly  bool   `json:"summary_only,omitempty" jsonschema:"Omit field-level diffs and return one summary line per revision."`
}

// changeBrief is a revision with diffs capped; DiffsTotal counts them all.
type changeBrief struct {
	model.WorkloadChange
	DiffsTotal int `json:"diffs_total"`
}

type workloadChangesOut struct {
	model.WorkloadChanges
	Changes        []changeBrief `json:"changes"`
	TotalRevisions int           `json:"total_revisions"`
}

func (s *Server) workloadChanges(ctx context.Context, _ *mcp.CallToolRequest, in workloadChangesIn) (*mcp.CallToolResult, workloadChangesOut, error) {
	switch in.Kind {
	case "Deployment", "StatefulSet", "DaemonSet", "HelmRelease":
	default:
		return nil, workloadChangesOut{}, fmt.Errorf("unsupported kind %q; use Deployment, StatefulSet, DaemonSet or HelmRelease", in.Kind)
	}
	ns, err := parseNamespace(in.Namespace)
	if err != nil {
		return nil, workloadChangesOut{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, workloadChangesOut{}, fmt.Errorf("name is required")
	}
	client, err := s.newClient(in.KubeContext, ns)
	if err != nil {
		return nil, workloadChangesOut{}, err
	}
	if namespaceMissing(ctx, client, ns) {
		return nil, workloadChangesOut{}, errNamespaceNotFound(ns, client.Context)
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
	return nil, shapeChanges(wc, clamp(in.MaxRevisions, 50, 5), in.SummaryOnly), nil
}

func shapeChanges(wc model.WorkloadChanges, maxRevisions int, summaryOnly bool) workloadChangesOut {
	out := workloadChangesOut{WorkloadChanges: wc, TotalRevisions: len(wc.Changes)}
	out.WorkloadChanges.Changes = nil
	out.AccessMessage, out.HelmNote = clean(wc.AccessMessage), clean(wc.HelmNote)
	out.Changes = mapSlice(capSlice(wc.Changes, maxRevisions), func(c model.WorkloadChange) changeBrief {
		b := changeBrief{WorkloadChange: c, DiffsTotal: len(c.Diffs)}
		b.Summary, b.ChangeCause, b.Actor = clean(c.Summary), clean(c.ChangeCause), clean(c.Actor)
		b.Diffs = nil
		if !summaryOnly {
			// Diff values are already masked by key and shape (changes.RedactValue);
			// clean also catches credentials embedded in longer values.
			b.Diffs = mapSlice(capSlice(c.Diffs, maxDiffsPerRevision), func(d model.FieldDiff) model.FieldDiff {
				d.Path, d.Before, d.After = clean(d.Path), clean(d.Before), clean(d.After)
				return d
			})
		}
		return b
	})
	return out
}
