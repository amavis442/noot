// Package theme is the palette shared by the terminal UI and the styled
// output of the commands, so both look the same.
package theme

import "github.com/charmbracelet/lipgloss"

// Colours adapt to a light or dark terminal background.
var (
	Accent  = lipgloss.AdaptiveColor{Light: "#6B4FBB", Dark: "#B8A2FF"}
	Muted   = lipgloss.AdaptiveColor{Light: "#6C6C6C", Dark: "#9A9A9A"}
	Danger  = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#FF7B72"}
	Success = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#7EE787"}
	Warning = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"}
	Panel   = lipgloss.AdaptiveColor{Light: "#D6D0EE", Dark: "#3D3557"}
)
