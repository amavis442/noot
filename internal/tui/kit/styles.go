package kit

import (
	"github.com/amavis442/noot/internal/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The palette is shared with the styled output of the commands.
var (
	accent  = theme.Accent
	muted   = theme.Muted
	danger  = theme.Danger
	success = theme.Success
	panel   = theme.Panel
)

// The theme of the terminal UI.
var (
	Brand     = lipgloss.NewStyle().Bold(true).Foreground(accent)
	Tab       = lipgloss.NewStyle().Padding(0, 1).Foreground(muted)
	ActiveTab = lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(accent)
	Heading   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	Muted     = lipgloss.NewStyle().Foreground(muted)
	Selected  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	Error     = lipgloss.NewStyle().Foreground(danger)
	Success   = lipgloss.NewStyle().Foreground(success)
	Help      = lipgloss.NewStyle().Foreground(muted)
	Divider   = lipgloss.NewStyle().Foreground(panel)
)

// Truncate cuts s to width cells, ending in "…" when it was longer.
func Truncate(s string, width int) string {
	return ansi.Truncate(s, width, "…")
}

// boxBorder and boxPadding are the room a Box takes around its content.
const (
	boxBorder  = 1
	boxPadding = 1
)

// BoxInner returns the room for content inside a Box of width x height.
func BoxInner(width, height int) (int, int) {
	return max(width-2*(boxBorder+boxPadding), 1), max(height-2*boxBorder, 1)
}

// Box draws content in a rounded panel of exactly width x height cells. The
// focused panel's border has the accent colour, as in superfile.
func Box(content string, width, height int, focused bool) string {
	border := panel
	if focused {
		border = accent
	}
	w, h := BoxInner(width, height)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, boxPadding).
		Width(w + 2*boxPadding).Height(h).MaxHeight(h + 2*boxBorder).
		Render(lipgloss.NewStyle().MaxWidth(w).MaxHeight(h).Render(content))
}
