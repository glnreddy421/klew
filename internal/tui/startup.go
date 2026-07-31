package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
	invmodel "github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/service"
	"github.com/glnreddy421/klew/internal/tui/views"
)

const (
	startupPhaseNamespace = iota
	startupPhaseSearching
	startupPhaseResourcePick
	startupPhaseBuilding
)

// LiveStartupOptions configures the investigation startup flow.
type LiveStartupOptions struct {
	Client           *kube.Client
	Query            string
	Namespace        string // set when -n passed; empty triggers picker
	ContextNamespace string // kubeconfig hint for default cursor
	AllNamespaces    bool
	Window           time.Duration
	Tail             int
	PollEvery        time.Duration
	AutoRefresh      bool
}

type liveHolder struct {
	session liveSession
	err     error
}

type startupTickMsg struct{}

type namespacesLoadedMsg struct {
	names []string
	err   error
}

type discoveryDoneMsg struct {
	matches []invmodel.MatchedObject
	err     error
}

type resourcesLoadedMsg struct {
	groups []kube.ResourceGroup
	err    error
}

// liveStartup drives namespace selection → workload search → resource picker → investigation.
type liveStartup struct {
	ctx    context.Context
	client *kube.Client
	opts   LiveStartupOptions
	holder *liveHolder

	phase int

	allNamespaces []string
	filteredNS    []string
	nsFilter      string
	nsCursor      int
	selectedNS    string

	searchRevealed int
	matches        []invmodel.MatchedObject
	searchErr      error
	noMatches      bool

	resourceGroups []kube.ResourceGroup
	flatResources  []kube.NamespaceResource
	resCursor      int
	searchQuery    string
	targetKind     string
	targetName     string

	build  int
	width  int
	height int
}

// RunLiveStartup runs namespace selection and workload discovery before launching the live TUI.
func RunLiveStartup(ctx context.Context, opts LiveStartupOptions) error {
	h := &liveHolder{}
	m := liveStartup{
		ctx:         ctx,
		client:      opts.Client,
		opts:        opts,
		holder:      h,
		searchQuery: opts.Query,
		width:       100,
		height:      32,
	}
	if opts.AllNamespaces {
		m.phase = startupPhaseSearching
		m.selectedNS = "*"
	} else if opts.Namespace != "" {
		m.phase = startupPhaseSearching
		m.selectedNS = opts.Namespace
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	if err != nil {
		return err
	}
	if h.err != nil {
		return h.err
	}
	if h.session != nil {
		h.session.Stop()
	}
	return nil
}

func startupTickCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return startupTickMsg{} })
}

func (m liveStartup) Init() tea.Cmd {
	if m.phase == startupPhaseNamespace {
		return tea.Batch(loadNamespacesCmd(m.ctx, m.client), startupTickCmd())
	}
	if m.phase == startupPhaseSearching {
		return tea.Batch(m.discoverCmd(), startupTickCmd())
	}
	return startupTickCmd()
}

func loadNamespacesCmd(ctx context.Context, client *kube.Client) tea.Cmd {
	return func() tea.Msg {
		names, err := kube.ListNamespaces(ctx, client)
		if err != nil {
			return namespacesLoadedMsg{err: err}
		}
		return namespacesLoadedMsg{names: kube.SortByFuzzy(names, "")}
	}
}

func (m liveStartup) discoverCmd() tea.Cmd {
	ctx, client, ns, query, allNS := m.ctx, m.client, m.selectedNS, m.searchQuery, m.opts.AllNamespaces
	return func() tea.Msg {
		if allNS {
			return discoveryDoneMsg{matches: nil, err: nil}
		}
		matches, err := kube.DiscoverMatches(ctx, client, ns, query)
		return discoveryDoneMsg{matches: matches, err: err}
	}
}

func (m liveStartup) loadResourcesCmd() tea.Cmd {
	ctx, client, ns, query := m.ctx, m.client, m.selectedNS, m.searchQuery
	return func() tea.Msg {
		groups, err := kube.ListNamespaceResources(ctx, client, ns, query)
		return resourcesLoadedMsg{groups: groups, err: err}
	}
}

func (m liveStartup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case namespacesLoadedMsg:
		if msg.err != nil {
			m.holder.err = msg.err
			return m, tea.Quit
		}
		m.allNamespaces = msg.names
		m.filteredNS = m.rankNamespaces(msg.names)
		m.nsCursor = m.defaultNamespaceCursor()
		return m, nil

	case discoveryDoneMsg:
		m.searchErr = msg.err
		m.matches = msg.matches
		if m.opts.AllNamespaces {
			return m.launchAllNamespaces()
		}
		if msg.err != nil {
			m.noMatches = true
			m.phase = startupPhaseResourcePick
			m.resCursor = 0
			return m, m.loadResourcesCmd()
		}
		if target, ok := m.autoTarget(msg.matches); ok {
			m.targetKind, m.targetName = target.Kind, target.Name
			return m.beginBuild()
		}
		m.noMatches = len(msg.matches) == 0
		m.phase = startupPhaseResourcePick
		m.resCursor = 0
		if len(msg.matches) > 0 {
			m.flatResources = matchesToResources(msg.matches)
			m.resourceGroups = groupResources(m.flatResources)
			return m, nil
		}
		return m, m.loadResourcesCmd()

	case resourcesLoadedMsg:
		if msg.err != nil {
			m.holder.err = msg.err
			return m, tea.Quit
		}
		m.resourceGroups = msg.groups
		m.flatResources = kube.FlattenResourceGroups(msg.groups)
		if m.resCursor >= len(m.flatResources) {
			m.resCursor = max(0, len(m.flatResources)-1)
		}
		return m, nil

	case startupTickMsg:
		switch m.phase {
		case startupPhaseSearching:
			m.searchRevealed++
			if m.searchRevealed >= len(discoverySources)+2 && m.matches == nil && m.searchErr == nil && !m.opts.AllNamespaces {
				return m, nil
			}
			if m.phase == startupPhaseSearching && m.searchRevealed < len(discoverySources)+4 {
				return m, startupTickCmd()
			}
		case startupPhaseBuilding:
			m.build++
			if m.build >= len(scopeBuildSteps)+3 {
				return m.launch()
			}
			return m, startupTickCmd()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "up", "k":
			return m.moveCursor(-1)
		case "down", "j":
			return m.moveCursor(1)
		case "backspace":
			return m.trimFilter()
		default:
			if len(msg.Runes) > 0 && msg.Type == tea.KeyRunes {
				return m.appendFilter(string(msg.Runes))
			}
		}
	}
	return m, nil
}

func (m liveStartup) handleEnter() (tea.Model, tea.Cmd) {
	switch m.phase {
	case startupPhaseNamespace:
		if len(m.filteredNS) == 0 {
			return m, nil
		}
		m.selectedNS = m.filteredNS[m.nsCursor]
		m.phase = startupPhaseSearching
		m.searchRevealed = 0
		return m, tea.Batch(m.discoverCmd(), startupTickCmd())
	case startupPhaseSearching:
		if m.matches != nil || m.searchErr != nil || m.opts.AllNamespaces {
			return m, nil
		}
		return m, nil
	case startupPhaseResourcePick:
		if len(m.flatResources) == 0 {
			return m, nil
		}
		r := m.flatResources[m.resCursor]
		m.targetKind, m.targetName = investigationKind(r.Kind), r.Name
		return m.beginBuild()
	}
	return m, nil
}

func (m liveStartup) moveCursor(delta int) (tea.Model, tea.Cmd) {
	switch m.phase {
	case startupPhaseNamespace:
		if len(m.filteredNS) == 0 {
			return m, nil
		}
		m.nsCursor += delta
		if m.nsCursor < 0 {
			m.nsCursor = 0
		}
		if m.nsCursor >= len(m.filteredNS) {
			m.nsCursor = len(m.filteredNS) - 1
		}
	case startupPhaseResourcePick:
		if len(m.flatResources) == 0 {
			return m, nil
		}
		m.resCursor += delta
		if m.resCursor < 0 {
			m.resCursor = 0
		}
		if m.resCursor >= len(m.flatResources) {
			m.resCursor = len(m.flatResources) - 1
		}
	}
	return m, nil
}

func (m liveStartup) appendFilter(s string) (tea.Model, tea.Cmd) {
	switch m.phase {
	case startupPhaseNamespace:
		m.nsFilter += s
		m.filteredNS = m.rankNamespaces(m.allNamespaces)
		m.nsCursor = 0
	case startupPhaseResourcePick:
		m.searchQuery += s
		m.resCursor = 0
		m.matches = nil
		return m, m.loadResourcesCmd()
	}
	return m, nil
}

func (m liveStartup) trimFilter() (tea.Model, tea.Cmd) {
	switch m.phase {
	case startupPhaseNamespace:
		if len(m.nsFilter) > 0 {
			m.nsFilter = m.nsFilter[:len(m.nsFilter)-1]
			m.filteredNS = m.rankNamespaces(m.allNamespaces)
			m.nsCursor = 0
		}
	case startupPhaseResourcePick:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.resCursor = 0
			m.matches = nil
			return m, m.loadResourcesCmd()
		}
	}
	return m, nil
}

func (m liveStartup) rankNamespaces(names []string) []string {
	if m.nsFilter == "" {
		return kube.SortByFuzzy(names, m.opts.Query)
	}
	var out []string
	for _, n := range names {
		if kube.FuzzyContains(n, m.nsFilter) {
			out = append(out, n)
		}
	}
	return kube.SortByFuzzy(out, m.nsFilter)
}

func (m liveStartup) defaultNamespaceCursor() int {
	if m.opts.ContextNamespace == "" {
		return 0
	}
	for i, n := range m.filteredNS {
		if n == m.opts.ContextNamespace {
			return i
		}
	}
	return 0
}

func (m liveStartup) autoTarget(matches []invmodel.MatchedObject) (invmodel.ObjectRef, bool) {
	if len(matches) != 1 {
		return invmodel.ObjectRef{}, false
	}
	m0 := matches[0]
	if m0.Score < 0.8 {
		return invmodel.ObjectRef{}, false
	}
	switch m0.Ref.Kind {
	case "Deployment", "StatefulSet", "Service", "Pod":
		return m0.Ref, true
	default:
		return invmodel.ObjectRef{}, false
	}
}

func (m liveStartup) beginBuild() (tea.Model, tea.Cmd) {
	m.phase = startupPhaseBuilding
	m.build = 0
	return m, startupTickCmd()
}

func (m liveStartup) finalQuery() string {
	if m.targetName == "" {
		return m.searchQuery
	}
	switch m.targetKind {
	case "Deployment":
		return "deployment/" + m.targetName
	case "StatefulSet":
		return "statefulset/" + m.targetName
	case "Service":
		return "service/" + m.targetName
	case "Pod":
		return "pod/" + m.targetName
	default:
		return m.targetName
	}
}

func (m liveStartup) launch() (tea.Model, tea.Cmd) {
	query := m.finalQuery()
	ns := m.selectedNS
	allNS := m.opts.AllNamespaces
	m.client.Namespace = ns

	sess, err := service.Start(m.ctx, m.client, engine.LiveOptions{
		Query:        query,
		Namespace:    ns,
		AllNS:        allNS,
		Window:       m.opts.Window,
		Tail:         m.opts.Tail,
		PollEvery:    m.opts.PollEvery,
		AutoRefresh:  m.opts.AutoRefresh,
	})
	if err != nil {
		m.holder.err = err
		return m, tea.Quit
	}
	m.holder.session = sess
	app := newLiveModel(m.ctx, sess, invmodel.ModeLive)
	app.ui.width = m.width
	app.ui.height = m.height
	return app, app.Init()
}

func (m liveStartup) launchAllNamespaces() (tea.Model, tea.Cmd) {
	m.client.Namespace = m.client.ContextNamespace
	sess, err := service.Start(m.ctx, m.client, engine.LiveOptions{
		Query:        m.opts.Query,
		Namespace:    "",
		AllNS:        true,
		Window:       m.opts.Window,
		Tail:         m.opts.Tail,
		PollEvery:    m.opts.PollEvery,
		AutoRefresh:  m.opts.AutoRefresh,
	})
	if err != nil {
		m.holder.err = err
		return m, tea.Quit
	}
	m.holder.session = sess
	app := newLiveModel(m.ctx, sess, invmodel.ModeLive)
	app.ui.width = m.width
	app.ui.height = m.height
	return app, app.Init()
}

func (m liveStartup) View() string {
	switch m.phase {
	case startupPhaseNamespace:
		return m.viewNamespace()
	case startupPhaseSearching:
		return m.viewSearching()
	case startupPhaseResourcePick:
		return m.viewResourcePick()
	case startupPhaseBuilding:
		return m.viewBuilding()
	}
	return ""
}

func (m liveStartup) viewNamespace() string {
	var b strings.Builder
	ctx := fmt.Sprintf("ctx=%s · query=%s", m.client.Context, m.opts.Query)
	WriteLaunchHeader(&b, ctx, m.width)
	b.WriteString(headStyleTUI.Render("Select Investigation Namespace") + "\n\n")

	if len(m.filteredNS) == 0 {
		b.WriteString(Muted.Render("  (no namespaces match filter)\n"))
	} else {
		for i, ns := range m.filteredNS {
			if i == m.nsCursor {
				b.WriteString(selArrow.Render("❯ ") + selRow.Render(ns) + "\n")
			} else {
				b.WriteString("  " + ns + "\n")
			}
		}
	}
	b.WriteString("\n" + views.PaneDivider(m.width))
	b.WriteString("\n")
	if m.nsFilter != "" {
		b.WriteString(Muted.Render("Filter: ") + m.nsFilter + "\n")
	} else {
		b.WriteString(Muted.Render("Type to filter namespaces…"))
	}
	b.WriteString("\n" + Muted.Render("↑/↓ navigate · enter select · q quit"))
	return b.String()
}

func (m liveStartup) viewSearching() string {
	var b strings.Builder
	ctxLine := fmt.Sprintf("ctx=%s · query=%s", m.client.Context, m.opts.Query)
	if m.opts.AllNamespaces {
		ctxLine += " · all namespaces"
	} else {
		ctxLine += fmt.Sprintf(" · ns=%s", m.selectedNS)
	}
	WriteScreenHeader(&b, ctxLine)

	if m.opts.AllNamespaces {
		b.WriteString(fmt.Sprintf("Searching %q across all namespaces…\n\n", m.opts.Query))
	} else {
		b.WriteString(fmt.Sprintf("Namespace: %s\n\n", m.selectedNS))
		b.WriteString(fmt.Sprintf("Searching %q…\n\n", m.searchQuery))
	}

	n := m.searchRevealed
	if n > len(discoverySources) {
		n = len(discoverySources)
	}
	for i, s := range discoverySources {
		if i < n {
			b.WriteString(okStyle.Render("  ✓ ") + s + "\n")
		} else {
			b.WriteString(Muted.Render("  · "+s) + "\n")
		}
	}
	b.WriteString("\n" + Muted.Render("q quit"))
	return b.String()
}

func (m liveStartup) viewResourcePick() string {
	var b strings.Builder
	ctx := fmt.Sprintf("ctx=%s · ns=%s", m.client.Context, m.selectedNS)
	WriteScreenHeader(&b, ctx)

	if m.noMatches {
		b.WriteString(Warning.Render("No matching resources found.") + "\n\n")
	} else {
		b.WriteString(headStyleTUI.Render(fmt.Sprintf("Found %d candidates", len(m.flatResources))))
		b.WriteString("\n\n")
	}

	b.WriteString("Search again:\n\n")
	b.WriteString(selRow.Render("> "+m.searchQuery) + "\n")
	b.WriteString("\n" + views.PaneDivider(m.width) + "\n\n")

	if len(m.flatResources) == 0 {
		b.WriteString(Muted.Render("  (no resources match)\n"))
	} else {
		cursor := 0
		for _, g := range m.resourceGroups {
			b.WriteString(headStyleTUI.Render(g.Kind) + "\n")
			for _, name := range g.Items {
				label := "  " + name
				if cursor == m.resCursor {
					b.WriteString(selArrow.Render("❯ ") + selRow.Render(name) + "\n")
				} else {
					b.WriteString(label + "\n")
				}
				cursor++
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(Muted.Render("type to filter · ↑/↓ navigate · enter investigate · q quit"))
	return b.String()
}

func (m liveStartup) viewBuilding() string {
	var b strings.Builder
	target := m.targetName
	if target == "" {
		target = m.searchQuery
	}
	ctx := fmt.Sprintf("ns=%s · %s", m.selectedNS, target)
	WriteScreenHeader(&b, ctx)
	b.WriteString(headStyleTUI.Render("Building Investigation Scope…") + "\n\n")
	for i, s := range scopeBuildSteps {
		if i < m.build {
			b.WriteString(okStyle.Render("  ✓ ") + s + "\n")
		} else {
			b.WriteString(Muted.Render("  · "+s) + "\n")
		}
	}
	if m.build >= len(scopeBuildSteps) {
		b.WriteString("\n" + StatusStyle.Render("Starting live investigation…"))
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func matchesToResources(matches []invmodel.MatchedObject) []kube.NamespaceResource {
	var out []kube.NamespaceResource
	seen := map[string]bool{}
	for _, m := range matches {
		key := m.Ref.Kind + "/" + m.Ref.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, kube.NamespaceResource{Kind: displayKind(m.Ref.Kind), Name: m.Ref.Name})
	}
	return out
}

func groupResources(items []kube.NamespaceResource) []kube.ResourceGroup {
	order := []string{"Deployments", "StatefulSets", "DaemonSets", "Services", "Pods"}
	byKind := map[string][]string{}
	for _, r := range items {
		byKind[r.Kind] = append(byKind[r.Kind], r.Name)
	}
	var out []kube.ResourceGroup
	for _, kind := range order {
		names := byKind[kind]
		if len(names) == 0 {
			continue
		}
		out = append(out, kube.ResourceGroup{Kind: kind, Items: names})
	}
	return out
}

func displayKind(kind string) string {
	switch kind {
	case "Deployment":
		return "Deployments"
	case "StatefulSet":
		return "StatefulSets"
	case "DaemonSet":
		return "DaemonSets"
	case "Service":
		return "Services"
	case "Pod":
		return "Pods"
	default:
		return kind
	}
}

func investigationKind(display string) string {
	switch display {
	case "Deployments":
		return "Deployment"
	case "StatefulSets":
		return "StatefulSet"
	case "DaemonSets":
		return "DaemonSet"
	case "Services":
		return "Service"
	case "Pods":
		return "Pod"
	default:
		return display
	}
}
