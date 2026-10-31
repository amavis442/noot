package kit

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// View is one tab of the terminal UI. The root model sends key messages to
// the active view only and every other message to all views, so a view picks
// out the results of its own commands.
type View interface {
	// Title is the tab's label.
	Title() string
	// Init loads the view's data; the root calls it again whenever the tab is
	// opened, so what was changed elsewhere shows up.
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	View() string
	// SetSize gives the view the room below the tab bar and above the help line.
	SetSize(width, height int)
	// Capturing is true while a text field has the keyboard, so keys such as
	// q, esc and 1-4 are typed instead of handled by the root, and ctrl+c
	// cancels typing instead of quitting.
	Capturing() bool
	// Help lists the view's keys that work right now, for the help line and
	// the ? overlay.
	Help() []key.Binding
}
