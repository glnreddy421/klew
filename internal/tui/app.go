package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	invmodel "github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/tui/views"
)

type appModel struct {
	ctx     context.Context
	st      invmodel.InvestigationState
	ui      uiState
	session liveSession // nil in bundle/demo mode
}

func newLiveModel(ctx context.Context, session liveSession, mode invmodel.Mode) appModel {
	st := session.State()
	st.Mode = mode
	return appModel{
		ctx:     ctx,
		st:      st,
		ui:      defaultUIState(),
		session: session,
	}
}

func newStaticModel(st invmodel.InvestigationState) appModel {
	if st.Mode == "" {
		st.Mode = invmodel.ModeBundle
	}
	return appModel{st: st, ui: defaultUIState()}
}

func (m appModel) Init() tea.Cmd {
	if m.session != nil {
		return tickCmd()
	}
	return nil
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ui.width = msg.Width
		m.ui.height = msg.Height
		return m, m.nextTick()
	case tickMsg:
		if m.session != nil {
			newSt := m.session.State()
			if !m.ui.streamFollow {
				m.preserveStreamScrollAnchor(newSt)
			}
			m.st = newSt
		}
		if m.ui.streamFollow {
			m.ui.streamScroll = 0
		}
		return m, m.nextTick()
	case tea.KeyMsg:
		if m.ui.searching {
			return m.updateSearch(msg)
		}
		if m.ui.tlSearching {
			return m.updateTimelineSearch(msg)
		}
		// Timeline tab owns a set of context keys (category filters + search +
		// fold toggle) that only affect the top panel, never the live stream.
		if m.ui.tab == TabTimeline {
			if handled, nm := m.timelineKey(msg.String()); handled {
				return nm, nil
			}
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.ui.showHelp = !m.ui.showHelp
		case "S":
			m.ui.showSettings = !m.ui.showSettings
		case "/":
			if m.ui.tab == TabEvidence {
				break
			}
			m.ui.searching = true
			m.ui.status = "search (enter to apply, esc to clear)"
		case "e":
			if m.ui.tab == TabEvidence {
				break
			}
			m.ui.streamMode = m.ui.streamMode.Next()
			m.ui.status = "stream: " + m.ui.streamMode.String()
		case "p":
			m.ui.paused = true
			if m.session != nil {
				m.session.Pause(true)
			}
			m.ui.status = "paused"
		case "r":
			m.ui.paused = false
			if m.session != nil {
				m.session.Pause(false)
			}
			m.ui.status = "live"
			return m, m.nextTick()
		case "a":
			if m.session != nil {
				m.session.SetAutoRefresh(!m.session.AutoRefresh())
				if m.session.AutoRefresh() {
					m.ui.status = fmt.Sprintf("auto-refresh on (%s)", m.session.PollInterval())
				} else {
					m.ui.status = "auto-refresh off"
				}
			}
		case "esc":
			if m.ui.showSettings {
				m.ui.showSettings = false
				break
			}
			m.ui.filter = ""
			m.ui.scroll = 0
			m.ui.streamScroll = 0
			m.ui.streamFollow = true
		case "f":
			if m.ui.tab != TabEvidence && !m.ui.showSettings {
				m.ui.streamFocused = !m.ui.streamFocused
				if m.ui.streamFocused {
					m.ui.status = "stream focused (j/k scroll · f back to tabs)"
				} else {
					m.ui.status = "tab focused"
				}
			}
		case "j", "down":
			if m.streamScrollActive() {
				m.ui.streamScroll++
				m.ui.streamFollow = false
				m.clampStreamScroll()
			} else {
				m.ui.scroll++
				m.clampTabScroll()
			}
		case "k", "up":
			if m.streamScrollActive() {
				if m.ui.streamScroll > 0 {
					m.ui.streamScroll--
				}
				if m.ui.streamScroll == 0 {
					m.ui.streamFollow = true
				}
				m.clampStreamScroll()
			} else if m.ui.scroll > 0 {
				m.ui.scroll--
			}
		case "J":
			if m.streamScrollActive() {
				m.ui.streamScroll += 5
				m.ui.streamFollow = false
				m.clampStreamScroll()
			}
		case "K":
			if m.streamScrollActive() {
				m.ui.streamScroll -= 5
				if m.ui.streamScroll < 0 {
					m.ui.streamScroll = 0
				}
				if m.ui.streamScroll == 0 {
					m.ui.streamFollow = true
				}
				m.clampStreamScroll()
			}
		case "1", "2", "3", "4", "5", "6":
			if t := Tab(int(msg.String()[0] - '1')); t < tabCount {
				m.ui.tab = t
				m.ui.scroll = 0
				m.ui.streamFocused = false
				m.ui.showSettings = false
			}
		case "pgdown":
			if m.streamScrollActive() {
				m.ui.streamScroll += 10
				m.ui.streamFollow = false
				m.clampStreamScroll()
			} else {
				m.ui.scroll += 10
				m.clampTabScroll()
			}
		case "pgup":
			if m.streamScrollActive() {
				m.ui.streamScroll -= 10
				if m.ui.streamScroll < 0 {
					m.ui.streamScroll = 0
				}
				if m.ui.streamScroll == 0 {
					m.ui.streamFollow = true
				}
				m.clampStreamScroll()
			} else {
				m.ui.scroll -= 10
				if m.ui.scroll < 0 {
					m.ui.scroll = 0
				}
			}
		}
	}
	return m, nil
}

// updateSearch captures keystrokes into the stream filter without pausing
// background analysis (the store keeps processing evidence regardless).
func (m appModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.ui.searching = false
		m.ui.status = "filter=" + strconv.Quote(m.ui.filter)
	case tea.KeyEsc:
		m.ui.searching = false
		m.ui.filter = ""
		m.ui.status = "filter cleared"
	case tea.KeyBackspace:
		if n := len(m.ui.filter); n > 0 {
			r := []rune(m.ui.filter)
			m.ui.filter = string(r[:len(r)-1])
		}
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyRunes, tea.KeySpace:
		m.ui.filter += string(msg.Runes)
	}
	return m, nil
}

// timelineKey handles Timeline-tab-only keys. Returns handled=true when the key
// was consumed so it does not fall through to the global shortcuts (e.g. `e`
// which globally cycles the stream mode).
func (m appModel) timelineKey(key string) (bool, tea.Model) {
	switch key {
	case "l":
		m.ui.tlCat = "logs"
	case "e":
		m.ui.tlCat = "events"
	case "m":
		m.ui.tlCat = "metrics"
	case "o":
		m.ui.tlCat = "objects"
	case "s":
		m.ui.tlCat = "klew"
	case "a":
		m.ui.tlCat = "all"
	case "enter":
		m.ui.tlExpand = !m.ui.tlExpand
		return true, m
	case "/":
		m.ui.tlSearching = true
		m.ui.status = "timeline search (enter to apply, esc to clear)"
		return true, m
	case "esc":
		m.ui.tlCat = "all"
		m.ui.tlSearch = ""
		m.ui.scroll = 0
		m.ui.status = "timeline filters cleared"
		return true, m
	default:
		return false, m
	}
	m.ui.scroll = 0
	m.ui.status = "timeline: " + m.ui.tlCat
	return true, m
}

// updateTimelineSearch captures the timeline-only search query. Analysis and the
// live stream keep running underneath — this only narrows the top panel.
func (m appModel) updateTimelineSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.ui.tlSearching = false
		m.ui.status = "timeline search=" + strconv.Quote(m.ui.tlSearch)
	case tea.KeyEsc:
		m.ui.tlSearching = false
		m.ui.tlSearch = ""
		m.ui.status = "timeline search cleared"
	case tea.KeyBackspace:
		if n := len(m.ui.tlSearch); n > 0 {
			r := []rune(m.ui.tlSearch)
			m.ui.tlSearch = string(r[:len(r)-1])
		}
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyRunes, tea.KeySpace:
		m.ui.tlSearch += string(msg.Runes)
	}
	return m, nil
}

func (m appModel) nextTick() tea.Cmd {
	if m.session == nil || m.ui.paused {
		return nil
	}
	return tickCmd()
}

func (m appModel) View() string {
	width := m.ui.width
	if width == 0 {
		width = 100
	}
	height := m.ui.height
	if height == 0 {
		height = 32
	}

	help := 0
	if m.ui.showHelp {
		help = 1
	}
	// header + tabs + footer = 3; active tab banner = +1
	banner := 1
	if m.ui.showSettings {
		banner = 0
	}
	// Evidence and Graph tabs use the full content height — no live stream split.
	showStream := m.ui.tab != TabEvidence && m.ui.tab != TabGraph && !m.ui.showSettings
	contentH := height - 3 - help - banner
	if contentH < 6 {
		contentH = 6
	}
	upperH, streamH := splitPanelHeights(contentH, showStream)

	var b strings.Builder
	b.WriteString(m.renderHeader(width))
	b.WriteString("\n")
	b.WriteString(RenderTabs(m.ui.tab, width))
	b.WriteString("\n")
	if !m.ui.showSettings {
		b.WriteString(RenderTabBanner(m.ui.tab, width))
		b.WriteString("\n")
	}
	if m.ui.showHelp {
		helpText := "1-6 tabs · j/k scroll · f stream · a auto-refresh · e stream mode · S settings · p pause · q quit"
		if m.ui.tab == TabTimeline {
			helpText = "timeline: a/l/e/m/o/s filter · / search · ⏎ fold/expand · j/k scroll · esc clear · 1-6 tabs · q quit"
		}
		if m.ui.tab == TabGraph {
			helpText = "graph: j/k pgup/pgdn scroll full map · f stream · a auto-refresh · 1-6 tabs · q quit"
		}
		if m.ui.tab == TabEvidence {
			helpText = "evidence: live verdict report · j/k scroll · 1-6 tabs · S settings · p pause · r resume · q quit"
		}
		if m.ui.showSettings {
			helpText = "settings · esc or S to close · scope, permissions, watches & missing data"
		}
		b.WriteString(Muted.Render(helpText))
		b.WriteString("\n")
	}
	// Settings is a full-height overlay rather than a numbered tab: it replaces
	// the content region (keeping header, tab bar, and footer) until dismissed.
	if m.ui.showSettings {
		b.WriteString(fitHeight(views.SettingsView(m.st, width, m.settingsRuntime()), contentH))
		b.WriteString("\n")
		b.WriteString(FooterWithLogo(FooterHelp(), width))
		if m.ui.status != "" {
			b.WriteString("  ")
			b.WriteString(StatusStyle.Render(m.ui.status))
		}
		return b.String()
	}
	b.WriteString(fitHeight(m.renderTab(width, upperH), upperH))
	if showStream {
		b.WriteString("\n")
		b.WriteString(views.PaneDivider(width))
		b.WriteString("\n")
		b.WriteString(fitHeight(views.LiveStream(m.st, m.ui.streamMode, m.ui.filter, m.ui.streamScroll, streamH, width, m.ui.streamFocused, m.ui.streamFollow), streamH))
	}
	b.WriteString("\n")
	b.WriteString(FooterWithLogo(FooterHelp(), width))
	if m.ui.searching {
		b.WriteString("  ")
		b.WriteString(StatusStyle.Render("search: " + m.ui.filter + "▏"))
	} else if m.ui.tlSearching {
		b.WriteString("  ")
		b.WriteString(StatusStyle.Render("timeline search: " + m.ui.tlSearch + "▏"))
	} else if m.ui.status != "" {
		b.WriteString("  ")
		b.WriteString(StatusStyle.Render(m.ui.status))
	}
	return b.String()
}

func (m appModel) renderHeader(width int) string {
	ns := m.st.NamespaceScope.Primary
	if m.st.NamespaceScope.AllNamespaces {
		ns = "*"
	}
	if ns == "" {
		ns = m.st.Snapshot.Namespace
	}
	mode := string(m.st.Mode)
	if m.ui.paused {
		mode += "(paused)"
	}
	fields := fmt.Sprintf("ctx=%s  ns=%s  query=%s  mode=%s  %s",
		m.st.KubeContext.Context, ns, m.st.Query, mode, watchersLabel(m.st))
	line := RenderLogoBadge() + "  " + StatusStyle.Render(fields)
	if meta := views.HeaderMetaLine(m.st); meta != "" {
		line += "  " + Muted.Render("│  "+meta)
	}
	return line
}

// watchersLabel summarizes live watch health, e.g. "watching 8 sources · healthy"
// or "watching 7/8 · nodes unavailable".
func watchersLabel(st invmodel.InvestigationState) string {
	active := len(st.ActiveWatches)
	expected := st.ExpectedWatches
	if expected <= 0 {
		expected = active
	}
	if active >= expected && st.WatchNote == "" {
		return fmt.Sprintf("watching %d sources · healthy", active)
	}
	note := st.WatchNote
	if note == "" {
		note = "degraded"
	}
	return fmt.Sprintf("watching %d/%d · %s", active, expected, note)
}

func (m appModel) settingsRuntime() views.SettingsRuntime {
	rt := views.SettingsRuntime{}
	if m.session != nil {
		rt.AutoRefresh = m.session.AutoRefresh()
		rt.RefreshEvery = m.session.PollInterval()
	}
	return rt
}

func (m appModel) renderTab(width, height int) string {
	switch m.ui.tab {
	case TabIncident:
		return views.IncidentView(m.st, width, m.ui.scroll, height)
	case TabTimeline:
		return views.TimelineView(m.st, width, m.ui.scroll, height, m.ui.tlCat, m.ui.tlSearch, m.ui.tlExpand)
	case TabGraph:
		return views.GraphView(m.st, width, m.ui.scroll, height)
	case TabFailures:
		return views.FailuresView(m.st, width, m.ui.scroll, height)
	case TabResources:
		return views.ResourcesView(m.st, width, height)
	case TabEvidence:
		return views.EvidenceView(m.st, m.ui.filter, m.ui.scroll, height, width)
	default:
		return views.IncidentView(m.st, width, m.ui.scroll, height)
	}
}

// splitPanelHeights divides content area between the tab panel and live stream.
// The stream panel keeps a stable minimum height so its header is never clipped.
func splitPanelHeights(contentH int, showStream bool) (upperH, streamH int) {
	if !showStream {
		return contentH, 0
	}
	const dividerH = 1
	if contentH < 9 {
		streamH = 4
		upperH = contentH - streamH - dividerH
		if upperH < 2 {
			upperH = 2
			streamH = contentH - upperH - dividerH
		}
		return upperH, streamH
	}
	streamH = (contentH - dividerH) * 2 / 5
	if streamH < 4 {
		streamH = 4
	}
	maxStream := contentH - dividerH - 4
	if streamH > maxStream {
		streamH = maxStream
	}
	upperH = contentH - streamH - dividerH
	return upperH, streamH
}

func (m *appModel) clampTabScroll() {
	width := m.ui.width
	if width == 0 {
		width = 100
	}
	height := m.ui.height
	if height == 0 {
		height = 32
	}
	help := 0
	if m.ui.showHelp {
		help = 1
	}
	banner := 1
	contentH := height - 3 - help - banner
	if contentH < 6 {
		contentH = 6
	}
	showStream := m.ui.tab != TabEvidence && m.ui.tab != TabGraph && !m.ui.showSettings
	upperH, _ := splitPanelHeights(contentH, showStream)
	if m.ui.tab != TabGraph {
		return
	}
	inner := upperH - 2
	if inner < 6 {
		inner = 6
	}
	total := views.GraphLineCount(m.st, width)
	m.ui.scroll = views.ClampScroll(m.ui.scroll, total, inner)
}

func (m appModel) streamScrollActive() bool {
	if m.ui.tab == TabEvidence || m.ui.tab == TabGraph || m.ui.showSettings {
		return false
	}
	if m.ui.streamFocused {
		return true
	}
	// When the tab panel isn't scrolled, j/k naturally tails through log lines.
	return m.ui.scroll == 0 && m.streamHasOverflow()
}

func (m appModel) streamHasOverflow() bool {
	streamH := m.streamPanelHeight()
	if streamH <= 0 {
		return false
	}
	width := m.ui.width
	if width == 0 {
		width = 100
	}
	lines := views.StreamLineCount(m.st, m.ui.streamMode, m.ui.filter, width)
	return lines > views.StreamInnerRows(streamH)
}

func (m *appModel) preserveStreamScrollAnchor(newSt invmodel.InvestigationState) {
	width := m.ui.width
	if width == 0 {
		width = 100
	}
	oldLines := views.StreamLineCount(m.st, m.ui.streamMode, m.ui.filter, width)
	newLines := views.StreamLineCount(newSt, m.ui.streamMode, m.ui.filter, width)
	if delta := newLines - oldLines; delta > 0 {
		m.ui.streamScroll += delta
		m.clampStreamScrollForHeight(m.streamPanelHeight())
	}
}

func (m *appModel) clampStreamScroll() {
	m.clampStreamScrollForHeight(m.streamPanelHeight())
}

func (m *appModel) clampStreamScrollForHeight(streamH int) {
	if streamH <= 0 {
		return
	}
	width := m.ui.width
	if width == 0 {
		width = 100
	}
	m.ui.streamScroll = views.ClampStreamScroll(m.ui.streamScroll, streamH, width, m.st, m.ui.streamMode, m.ui.filter)
}

func (m appModel) streamPanelHeight() int {
	height := m.ui.height
	if height == 0 {
		height = 32
	}
	help := 0
	if m.ui.showHelp {
		help = 1
	}
	banner := 1
	if m.ui.showSettings {
		banner = 0
	}
	showStream := m.ui.tab != TabEvidence && !m.ui.showSettings
	contentH := height - 3 - help - banner
	if contentH < 6 {
		contentH = 6
	}
	_, streamH := splitPanelHeights(contentH, showStream)
	return streamH
}

// fitHeight pads or truncates a block to exactly n lines so the layout fills
// the terminal without leaving gaps.
func fitHeight(body string, n int) string {
	if n <= 0 {
		return body
	}
	lines := strings.Split(body, "\n")
	for len(lines) < n {
		lines = append(lines, "")
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
