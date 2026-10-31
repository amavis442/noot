package kit

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// KeyMap is every key of the terminal UI.
type KeyMap struct {
	// Global: superfile's "Basic Actions", "Navigation" and panel controls.
	Confirm       key.Binding
	Quit          key.Binding
	ListUp        key.Binding
	ListDown      key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	NextPanel     key.Binding
	PreviousPanel key.Binding
	TogglePreview key.Binding
	Edit          key.Binding
	Help          key.Binding
	ToggleDone    key.Binding
	Create        key.Binding

	// Typing: these win over every other key while a text field is open.
	ConfirmTyping key.Binding
	CancelTyping  key.Binding

	// Normal mode.
	Parent    key.Binding
	SearchBar key.Binding

	// noot's own, for which superfile has no counterpart.
	Tabs     key.Binding
	Previous key.Binding
	Next     key.Binding
	Today    key.Binding
	Done     key.Binding
	Plan     key.Binding
	// The preview never takes the keyboard, so the list keeps the arrows.
	PreviewDown key.Binding
	PreviewUp   key.Binding
}

// Keys follow superfile's default hotkeys
// (https://superfile.dev/list/hotkey-list), so its users feel at home.
var Keys = KeyMap{
	Confirm:       key.NewBinding(key.WithKeys("enter", "right", "l"), key.WithHelp("enter/l", "open in editor")),
	Quit:          key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "stop")),
	ListUp:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "omhoog")),
	ListDown:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "omlaag")),
	PageUp:        key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "pagina omhoog")),
	PageDown:      key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "pagina omlaag")),
	NextPanel:     key.NewBinding(key.WithKeys("tab", "L"), key.WithHelp("tab/L", "volgend paneel")),
	PreviousPanel: key.NewBinding(key.WithKeys("shift+left", "H"), key.WithHelp("H", "vorig paneel")),
	TogglePreview: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "preview aan/uit")),
	Edit:          key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "bewerk")),
	Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "alle toetsen")),
	ToggleDone:    key.NewBinding(key.WithKeys("."), key.WithHelp(".", "afgeronde aan/uit")),
	Create:        key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n", "nieuw")),

	ConfirmTyping: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "bevestig")),
	CancelTyping:  key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("esc", "annuleer")),

	Parent:    key.NewBinding(key.WithKeys("h", "left", "backspace"), key.WithHelp("h", "terug")),
	SearchBar: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "zoek")),

	Tabs:     key.NewBinding(key.WithKeys("1", "2", "3", "4"), key.WithHelp("1-4", "tab")),
	Previous: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "vorige")),
	Next:     key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "volgende")),
	Today:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "vandaag")),
	Done:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "klaar")),
	Plan:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "plan")),

	PreviewDown: key.NewBinding(key.WithKeys("J", "shift+down"), key.WithHelp("J/K", "preview scrollen")),
	PreviewUp:   key.NewBinding(key.WithKeys("K", "shift+up"), key.WithHelp("K", "preview omhoog")),
}

// Short renders bindings for the help line: "e bewerk · / zoek".
func Short(bindings ...key.Binding) string {
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		h := b.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return strings.Join(parts, " · ")
}

// WithDesc returns b with another description, for a view that gives the key
// its own meaning ("[" is the previous day in one tab, the previous week in
// another).
func WithDesc(b key.Binding, desc string) key.Binding {
	b.SetHelp(b.Help().Key, desc)
	return b
}
