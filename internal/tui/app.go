// Package tui is the terminal UI behind `noot tui`: a Bubble Tea program with
// one tab per view (search, journal, agenda, reminders) over the app use cases.
package tui

import (
	"fmt"
	"strings"

	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// chromeHeight is the number of lines around the active view: the tab bar,
// the divider under it and the help line.
const chromeHeight = 3

// Model is the root model: a tab bar over the active view.
type Model struct {
	vault  string // registered vault name, "" when unregistered
	views  []kit.View
	active int
	width  int
	height int
	notice error // shown on the help line until the next key
	help   bool  // the ? overlay with every key is open
}

// New returns the root model showing views as tabs, the first one active.
func New(vault string, views ...kit.View) Model {
	return Model{vault: vault, views: views}
}

// Init loads the first tab; the others load when they are opened.
func (m Model) Init() tea.Cmd {
	return m.views[m.active].Init()
}

// Update handles the global keys and passes everything else on.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		for _, v := range m.views {
			v.SetSize(msg.Width, max(msg.Height-chromeHeight, 1))
		}
		return m, nil
	case tea.KeyMsg:
		m.notice = nil
		return m.key(msg)
	case kit.EditedMsg:
		m.notice = msg.Err
	}
	cmds := make([]tea.Cmd, 0, len(m.views))
	for _, v := range m.views {
		cmds = append(cmds, v.Update(msg))
	}
	return m, tea.Batch(cmds...)
}

// key handles the root's keys. While the view captures the keyboard, every
// key is the view's, ctrl+c included: superfile's typing keys win over all.
func (m Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	v := m.views[m.active]
	switch {
	case m.help:
		m.help = false
		return m, nil
	case v.Capturing():
		return m, v.Update(msg)
	case key.Matches(msg, kit.Keys.Quit), msg.String() == "ctrl+c":
		return m, tea.Quit
	case key.Matches(msg, kit.Keys.Help):
		m.help = true
		return m, nil
	case key.Matches(msg, kit.Keys.Tabs):
		if i := int(msg.String()[0] - '1'); i < len(m.views) {
			return m.open(i)
		}
		return m, nil
	}
	return m, v.Update(msg)
}

// globalKeys are the root's keys, for the help line and the overlay.
func globalKeys() []key.Binding {
	return []key.Binding{kit.Keys.Help, kit.Keys.Tabs, kit.Keys.Quit}
}

// open makes view i the active tab and reloads it.
func (m Model) open(i int) (tea.Model, tea.Cmd) {
	m.active = i
	return m, m.views[i].Init()
}

// View draws the tab bar, the active view and the help line.
func (m Model) View() string {
	tabs := []string{kit.Brand.Render("noot")}
	if m.vault != "" {
		tabs[0] += kit.Muted.Render(" · " + m.vault)
	}
	tabs[0] += "  "
	for i, v := range m.views {
		label := fmt.Sprintf("%d %s", i+1, v.Title())
		if i == m.active {
			tabs = append(tabs, kit.ActiveTab.Render(label))
		} else {
			tabs = append(tabs, kit.Tab.Render(label))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	divider := kit.Divider.Render(strings.Repeat("─", max(m.width, 1)))
	height := max(m.height-chromeHeight, 1)
	content := m.views[m.active].View()
	if m.help {
		content = m.helpOverlay()
	}
	body := lipgloss.NewStyle().Height(height).MaxHeight(height).Render(content)
	// The root's keys come first, so ? stays in sight on a narrow terminal.
	keys := append(globalKeys(), m.views[m.active].Help()...)
	if m.views[m.active].Capturing() {
		keys = m.views[m.active].Help()
	}
	help := kit.Help.Render(kit.Short(keys...))
	if m.notice != nil {
		help = kit.Error.Render("editor: " + m.notice.Error())
	}
	return lipgloss.JoinVertical(lipgloss.Left, bar, divider, body, kit.Truncate(help, max(m.width, 1)))
}

// helpOverlay lists every key of the active view and of the root.
func (m Model) helpOverlay() string {
	var b strings.Builder
	b.WriteString(kit.Heading.Render("Toetsen · "+m.views[m.active].Title()) + "\n\n")
	for _, k := range append(m.views[m.active].Help(), globalKeys()...) {
		h := k.Help()
		fmt.Fprintf(&b, "  %s  %s\n", kit.Selected.Render(fmt.Sprintf("%-12s", h.Key)), h.Desc)
	}
	b.WriteString("\n" + kit.Muted.Render("Toetsen volgen superfile. Een willekeurige toets sluit dit."))
	return b.String()
}
