package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/investigation"
	invmodel "github.com/glnreddy421/klew/internal/model"
)

var discoverySources = []string{
	"Deployments", "ReplicaSets", "Pods", "Services", "Events",
	"Logs", "EndpointSlices", "ConfigMaps", "Secrets", "Metrics",
}

var scopeBuildSteps = []string{
	"Deployment", "ReplicaSets", "Pods", "Services", "EndpointSlices",
	"Events", "Logs", "ConfigMaps", "Secrets", "Metrics",
}

type demoHolder struct{ s liveSession }

type demoTickMsg struct{}

// demoIntro drives discovery → target picker → scope building, then hands off to
// the live investigation app.
type demoIntro struct {
	ctx      context.Context
	scenario engine.DemoScenario
	holder   *demoHolder
	phase    int // 0 = discovery, 1 = picker, 2 = building scope
	revealed int
	build    int
	cursor   int
	target   string
	scope    *investigation.InvestigationScope
	width    int
	height   int
}

// RunDemo runs the full demo: discovery → target picker → live investigation.
func RunDemo(ctx context.Context, query string) error {
	sc := engine.DiscoverDemo(query)
	h := &demoHolder{}
	m := demoIntro{ctx: ctx, scenario: sc, holder: h, width: 100, height: 32}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	if h.s != nil {
		h.s.Stop()
	}
	return err
}

func demoTickCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return demoTickMsg{} })
}

func (m demoIntro) Init() tea.Cmd { return demoTickCmd() }

func (m demoIntro) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case demoTickMsg:
		switch m.phase {
		case 0:
			m.revealed++
			if m.revealed >= len(discoverySources)+2 {
				m.phase = 1
				return m, nil
			}
			return m, demoTickCmd()
		case 2:
			m.build++
			if m.build >= m.buildTotal()+3 {
				return m.launch()
			}
			return m, demoTickCmd()
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.phase == 0 {
				m.phase = 1
				m.revealed = len(discoverySources) + 2
				return m, nil
			}
			if m.phase == 1 {
				return m.beginBuild()
			}
			return m, nil
		case "up", "k":
			if m.phase == 1 && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.phase == 1 && m.cursor < len(m.scenario.Targets)-1 {
				m.cursor++
			}
		default:
			if m.phase == 0 {
				m.phase = 1
				m.revealed = len(discoverySources) + 2
			}
		}
	}
	return m, nil
}

// beginBuild transitions from the picker into the scope-building bootstrap.
func (m demoIntro) beginBuild() (tea.Model, tea.Cmd) {
	m.target = m.scenario.Targets[m.cursor].Name
	m.scope = engine.DemoScope(m.scenario, m.target)
	m.phase = 2
	m.build = 0
	return m, demoTickCmd()
}

func (m demoIntro) buildTotal() int {
	n := len(scopeBuildSteps)
	if m.scope != nil {
		n += len(m.scope.Extensions)
	}
	return n
}

func (m demoIntro) launch() (tea.Model, tea.Cmd) {
	sess := engine.StartDemo(m.ctx, m.scenario, m.target)
	m.holder.s = sess
	app := newLiveModel(m.ctx, sess, invmodel.ModeLive)
	app.ui.width = m.width
	app.ui.height = m.height
	return app, app.Init()
}

func (m demoIntro) View() string {
	var b strings.Builder
	ctx := fmt.Sprintf("demo · ctx=%s · ns=%s · query=%s",
		m.scenario.Context, m.scenario.Namespace, m.scenario.Query)

	if m.phase == 0 {
		WriteLaunchHeader(&b, ctx, m.width)
		b.WriteString(fmt.Sprintf("Searching namespace %s…\n\n", m.scenario.Namespace))
		n := m.revealed
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
		b.WriteString("\n" + Muted.Render("enter to skip · q quit"))
		return b.String()
	}

	WriteScreenHeader(&b, ctx)
	b.WriteString(headStyleTUI.Render(fmt.Sprintf("Found %d workloads", len(m.scenario.Targets))))
	b.WriteString("  " + Muted.Render(fmt.Sprintf("(namespace %s)", m.scenario.Namespace)))
	b.WriteString("\n\n")

	// Align the name column to the widest target so the meta column lines up.
	nameW := 12
	for _, t := range m.scenario.Targets {
		if l := len(t.Name); l > nameW {
			nameW = l
		}
	}
	for i, t := range m.scenario.Targets {
		label := fmt.Sprintf("%-2d %-*s", i+1, nameW, t.Name)
		meta := Muted.Render(targetMeta(t))
		if i == m.cursor {
			b.WriteString(selArrow.Render("▸ ") + selRow.Render(label) + "  " + meta)
		} else {
			b.WriteString("  " + label + "  " + meta)
		}
		b.WriteString("\n")
	}
	if m.phase == 1 {
		b.WriteString("\n" + Muted.Render("↑/↓ move · enter investigate · q quit"))
		return b.String()
	}

	// phase 2 — building investigation scope
	b.Reset()
	ctx = fmt.Sprintf("demo · ns=%s · %s", m.scenario.Namespace, m.target)
	WriteScreenHeader(&b, ctx)
	b.WriteString(headStyleTUI.Render("Building Investigation Scope…") + "\n\n")
	for i, s := range scopeBuildSteps {
		if i < m.build {
			b.WriteString(okStyle.Render("  ✓ ") + s + "\n")
		} else {
			b.WriteString(Muted.Render("  · "+s) + "\n")
		}
	}
	if m.scope != nil && len(m.scope.Extensions) > 0 {
		b.WriteString("\n" + Muted.Render("Detected Extensions") + "\n")
		for i, ext := range m.scope.Extensions {
			if m.build > len(scopeBuildSteps)+i {
				b.WriteString(okStyle.Render("  ✓ ") + ext + "\n")
			} else {
				b.WriteString(Muted.Render("  · "+ext) + "\n")
			}
		}
	}
	if m.scope != nil && m.build >= m.buildTotal() {
		st := m.scope.Stats()
		b.WriteString("\n" + StatusStyle.Render(fmt.Sprintf(
			"Application %s · Resources %d · Relationships %d · Watchers 10 · Signals Live",
			m.target, st.Resources, st.Relationships)))
	}
	return b.String()
}

func targetMeta(t engine.DemoTarget) string {
	parts := []string{t.Kind}
	if t.Pods > 0 {
		parts = append(parts, fmt.Sprintf("%d pods", t.Pods))
	}
	if t.Extra != "" {
		parts = append(parts, t.Extra)
	}
	return strings.Join(parts, " · ")
}

var (
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	headStyleTUI = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	selArrow     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	selRow       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
)
