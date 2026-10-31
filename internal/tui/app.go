// Package tui is the terminal UI behind `noot tui`: a Bubble Tea program with
// one tab per view (search, journal, agenda, reminders) over the app use cases.
package tui

import (
	"fmt"
	"strings"

	"github.com/amavis442/noot/internal/tui/kit"
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

func (m Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		return m.open((m.active + 1) % len(m.views))
	case "shift+tab":
		return m.open((m.active + len(m.views) - 1) % len(m.views))
	}
	if !m.views[m.active].Capturing() {
		s := msg.String()
		if s == "q" {
			return m, tea.Quit
		}
		if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
			if i := int(s[0] - '1'); i < len(m.views) {
				return m.open(i)
			}
			return m, nil
		}
	}
	return m, m.views[m.active].Update(msg)
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
	body := lipgloss.NewStyle().Height(max(m.height-chromeHeight, 1)).MaxHeight(max(m.height-chromeHeight, 1)).
		Render(m.views[m.active].View())
	keys := " · tab/1-" + fmt.Sprint(len(m.views)) + " wissel · q stop"
	if m.views[m.active].Capturing() {
		keys = " · tab wissel · ctrl+c stop"
	}
	help := kit.Help.Render(m.views[m.active].Help() + keys)
	if m.notice != nil {
		help = kit.Error.Render("editor: " + m.notice.Error())
	}
	return lipgloss.JoinVertical(lipgloss.Left, bar, divider, body, kit.Truncate(help, max(m.width, 1)))
}
