package kit

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Colours adapt to a light or dark terminal background.
var (
	accent  = lipgloss.AdaptiveColor{Light: "#6B4FBB", Dark: "#B8A2FF"}
	muted   = lipgloss.AdaptiveColor{Light: "#6C6C6C", Dark: "#9A9A9A"}
	danger  = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#FF7B72"}
	success = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#7EE787"}
	panel   = lipgloss.AdaptiveColor{Light: "#D6D0EE", Dark: "#3D3557"}
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
