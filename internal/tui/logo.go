package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const logoTagline = "follow the thread"

var (
	logoBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	logoAccentStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("219"))
	logoWordStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	logoTagStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
)

// RenderLaunchBanner returns the large KLEW logo for startup / demo intro.
func RenderLaunchBanner(width int) string {
	lines := []string{
		padCenter(logoAccentStyle.Render("◈"), width),
		padCenter(logoBorderStyle.Render("╭────┴────╮"), width),
		padCenter(logoBorderStyle.Render("│")+logoWordStyle.Render(" K L E W ")+logoBorderStyle.Render("│"), width),
		padCenter(logoBorderStyle.Render("╰─────────╯"), width),
		padCenter(logoTagStyle.Render(logoTagline), width),
	}
	return strings.Join(lines, "\n")
}

// RenderLogoBadge is the compact inline mark used in the header.
func RenderLogoBadge() string {
	return logoAccentStyle.Render("◈") + " " + TitleStyle.Render("KLEW")
}

// RenderFooterLogo is a small corner mark for the bottom-right of the TUI.
func RenderFooterLogo() string {
	return logoBorderStyle.Render("╭") + logoAccentStyle.Render("◈") + logoBorderStyle.Render("╯") +
		Muted.Render(" KLEW")
}

// WriteLaunchHeader writes the large logo for first-run screens.
func WriteLaunchHeader(b *strings.Builder, ctxLine string, width int) {
	b.WriteString(RenderLaunchBanner(width))
	b.WriteString("\n")
	if ctxLine != "" {
		b.WriteString(StatusStyle.Render(ctxLine))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// WriteScreenHeader writes a compact logo badge (subsequent startup steps).
func WriteScreenHeader(b *strings.Builder, ctxLine string) {
	b.WriteString(RenderLogoBadge())
	if ctxLine != "" {
		b.WriteString("  " + StatusStyle.Render(ctxLine))
	}
	b.WriteString("\n\n")
}

// FooterWithLogo right-aligns the corner logo on the help line when there is room.
func FooterWithLogo(help string, width int) string {
	mark := RenderFooterLogo()
	if width <= 0 {
		return help + "  " + mark
	}
	gap := width - lipgloss.Width(help) - lipgloss.Width(mark)
	if gap < 2 {
		return help + "  " + mark
	}
	return help + strings.Repeat(" ", gap) + mark
}

func padCenter(s string, width int) string {
	if width <= 0 {
		return s
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	pad := (width - w) / 2
	return strings.Repeat(" ", pad) + s
}
