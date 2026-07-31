package views

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/render"
)

// ── shared styles ──────────────────────────────────────────────────────────

var (
	borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))  // cyan — panel section titles only
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))            // muted field labels
	keyStyle    = labelStyle
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	critStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	headStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")) // subsection titles
	issueStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")) // primary issue line
	highStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	klewStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")) // Klew reasoning
	metricStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))             // metrics: neutral, not cyan
	markStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	plainStyle  = lipgloss.NewStyle()
	freshStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("215")) // newly arrived evidence
)

// ── status icons ─────────────────────────────────────────────────────────

// Icon returns a colored status dot for a severity.
func Icon(sev model.Severity) string {
	switch sev {
	case model.SeverityCritical:
		return "🔴"
	case model.SeverityHigh:
		return "🟠"
	case model.SeverityWarning:
		return "🟡"
	default:
		return "🟢"
	}
}

// HealthIcon returns a colored status dot for a health string.
func HealthIcon(health string) string {
	switch strings.ToLower(health) {
	case "critical":
		return "🔴"
	case "warning", "degraded":
		return "🟠"
	case "unknown":
		return "🟡"
	default:
		return "🟢"
	}
}

// healthMark returns a calm colored dot for a health string — the same visual
// language as the Timeline/Incident panels (no emoji).
func healthMark(health string) string {
	switch strings.ToLower(health) {
	case "critical":
		return critStyle.Render("●")
	case "warning", "degraded":
		return highStyle.Render("●")
	case "unknown":
		return warnStyle.Render("●")
	default:
		return okStyle.Render("●")
	}
}

// sevMark returns a calm colored dot for a severity.
func sevMark(sev model.Severity) string {
	switch sev {
	case model.SeverityCritical:
		return critStyle.Render("●")
	case model.SeverityHigh:
		return highStyle.Render("●")
	case model.SeverityWarning:
		return warnStyle.Render("●")
	default:
		return okStyle.Render("●")
	}
}

// colorRow tints a whole row by severity (shared by events/resources/stream).
func colorRow(s string, sev model.Severity) string {
	switch sev {
	case model.SeverityCritical:
		return critStyle.Render(s)
	case model.SeverityHigh, model.SeverityWarning:
		return warnStyle.Render(s)
	default:
		return s
	}
}

func sevStyle(sev model.Severity) lipgloss.Style {
	switch sev {
	case model.SeverityCritical, model.SeverityHigh:
		return critStyle
	case model.SeverityWarning:
		return warnStyle
	default:
		return okStyle
	}
}

// ── width-aware helpers (emoji count as 2 cols) ──────────────────────────────

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func visualWidth(s string) int {
	s = ansiRe.ReplaceAllString(s, "")
	w := 0
	for _, r := range s {
		w += runeCols(r)
	}
	return w
}

func runeCols(r rune) int {
	switch {
	case r >= 0x1F300, r >= 0x2600 && r <= 0x27BF:
		return 2
	default:
		return 1
	}
}

func padRight(s string, w int) string {
	d := w - visualWidth(s)
	if d <= 0 {
		return s
	}
	return s + strings.Repeat(" ", d)
}

func truncVisual(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if visualWidth(s) <= w {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range ansiRe.ReplaceAllString(s, "") {
		c := runeCols(r)
		if used+c > w-1 {
			break
		}
		b.WriteRune(r)
		used += c
	}
	return b.String() + "…"
}

// wrapPanelLine splits an over-wide panel line without truncating; prefers breaking
// at spaces so values like "192Mi req · 384Mi limit" stay readable.
func wrapPanelLine(s string, w int) []string {
	if w <= 0 || visualWidth(s) <= w {
		return []string{s}
	}
	plain := ansiRe.ReplaceAllString(s, "")
	var lines []string
	for len(plain) > w {
		cut := w
		if cut > len(plain) {
			cut = len(plain)
		}
		if sp := strings.LastIndex(plain[:cut], " "); sp > w/3 {
			cut = sp
		}
		lines = append(lines, strings.TrimRight(plain[:cut], " "))
		plain = strings.TrimLeft(plain[cut:], " ")
	}
	if plain != "" {
		lines = append(lines, plain)
	}
	if len(lines) == 0 {
		return []string{s}
	}
	return lines
}

// ── panel box with embedded title ────────────────────────────────────────────

// PaneDivider renders a fixed boundary between the tab content and live stream.
func PaneDivider(width int) string {
	if width < 8 {
		width = 8
	}
	return borderStyle.Render(strings.Repeat("─", width))
}

// streamSourceHeader labels a distinct log/evidence source block.
func streamSourceHeader(source, kind string, width int) string {
	label := firstNonEmpty(source, "unknown")
	tag := strings.ToUpper(kind)
	if tag == "" {
		tag = "SRC"
	}
	prefix := fmt.Sprintf("  ┌─ %s · %s ", tag, label)
	fill := width - visualWidth(prefix) - 2
	if fill < 0 {
		fill = 0
	}
	return dimStyle.Render(prefix) + headStyle.Render(strings.Repeat("─", fill)) + dimStyle.Render("─┐")
}

// Panel renders a bordered box with the title embedded in the top border.
func Panel(title string, width int, body string) string {
	if width < 14 {
		width = 14
	}
	cw := width - 4
	t := truncVisual(title, width-6)
	fill := width - 5 - visualWidth(t)
	if fill < 0 {
		fill = 0
	}
	var b strings.Builder
	b.WriteString(borderStyle.Render("┌─ ") + titleStyle.Render(t) + " " + borderStyle.Render(strings.Repeat("─", fill)+"┐"))
	b.WriteString("\n")
	for _, line := range strings.Split(body, "\n") {
		for _, chunk := range wrapPanelLine(line, cw) {
			b.WriteString(borderStyle.Render("│") + " " + padRight(chunk, cw) + " " + borderStyle.Render("│"))
			b.WriteString("\n")
		}
	}
	b.WriteString(borderStyle.Render("└" + strings.Repeat("─", width-2) + "┘"))
	return b.String()
}

// PanelH renders a panel padded to a fixed inner height (rows of content).
func PanelH(title string, width, rows int, body string) string {
	lines := strings.Split(body, "\n")
	for len(lines) < rows {
		lines = append(lines, "")
	}
	if len(lines) > rows {
		lines = lines[:rows]
	}
	return Panel(title, width, strings.Join(lines, "\n"))
}

// TwoCol joins two rendered blocks side by side.
func TwoCol(left, right string) string {
	l := strings.Split(left, "\n")
	r := strings.Split(right, "\n")
	lw := 0
	for _, ln := range l {
		if w := visualWidth(ln); w > lw {
			lw = w
		}
	}
	n := len(l)
	if len(r) > n {
		n = len(r)
	}
	var out []string
	for i := 0; i < n; i++ {
		ls, rs := "", ""
		if i < len(l) {
			ls = l[i]
		}
		if i < len(r) {
			rs = r[i]
		}
		out = append(out, padRight(ls, lw)+" "+rs)
	}
	return strings.Join(out, "\n")
}

// ── bars ─────────────────────────────────────────────────────────────────────

func signalBar(value, max, width int) string {
	if max <= 0 {
		max = 1
	}
	n := value * width / max
	if value > 0 && n == 0 {
		n = 1
	}
	if n > width {
		n = width
	}
	return render.BarFill.Render(strings.Repeat("█", n)) + render.BarEmpty.Render(strings.Repeat("░", width-n))
}

func meter(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	n := int(frac * float64(width))
	col := render.BarFill
	switch {
	case frac >= 0.8:
		col = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	case frac >= 0.5:
		col = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
	default:
		col = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	}
	return col.Render(strings.Repeat("█", n)) + render.BarEmpty.Render(strings.Repeat("░", width-n))
}

// ── list / pagination ─────────────────────────────────────────────────────────

func paginate(body string, scroll, height int) string {
	lines := strings.Split(body, "\n")
	if height <= 0 {
		height = 20
	}
	scroll = ClampScroll(scroll, len(lines), height)
	end := scroll + height
	if end > len(lines) {
		end = len(lines)
	}
	if scroll >= len(lines) {
		return ""
	}
	return strings.Join(lines[scroll:end], "\n")
}

// ClampScroll limits scroll offset for a viewport of visible lines.
func ClampScroll(scroll, lineCount, visible int) int {
	if scroll < 0 {
		return 0
	}
	maxScroll := maxInt(0, lineCount-visible)
	if scroll > maxScroll {
		return maxScroll
	}
	return scroll
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
