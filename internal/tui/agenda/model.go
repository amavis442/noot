// Package agenda is the agenda tab of the terminal UI: the weekly agenda,
// day by day, opened at today.
package agenda

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/parser"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/vault"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Agenda is what the tab needs from the app layer.
type Agenda interface {
	Agenda(ctx context.Context, day time.Time) (*app.AgendaWeek, error)
}

// todayMark follows today's heading.
const todayMark = " · vandaag"

// weekMsg carries the agenda of the week starting monday.
type weekMsg struct {
	monday time.Time
	week   *app.AgendaWeek
	err    error
}

// Model is the agenda tab.
type Model struct {
	ctx    context.Context
	agenda Agenda
	editor kit.Editor
	now    func() time.Time

	monday time.Time // the week shown
	week   *app.AgendaWeek
	pane   *kit.Pane
	width  int
	height int
}

// New returns the agenda tab, starting at this week.
func New(ctx context.Context, a Agenda, md kit.Markdown, ed kit.Editor, now func() time.Time) *Model {
	return &Model{ctx: ctx, agenda: a, editor: ed, now: now, monday: vault.MondayOf(kit.Midnight(now())), pane: kit.NewPane(md)}
}

// Title is the tab's label.
func (m *Model) Title() string { return "Agenda" }

// Init loads the week shown.
func (m *Model) Init() tea.Cmd { return m.load(m.monday) }

// Capturing is always false: the tab has no text fields.
func (m *Model) Capturing() bool { return false }

// Help lists the keys.
func (m *Model) Help() []key.Binding {
	k := kit.Keys
	return []key.Binding{kit.WithDesc(k.Previous, "vorige week"), kit.WithDesc(k.Next, "volgende week"), kit.WithDesc(k.Today, "deze week"), k.Edit, k.ListDown, k.ListUp, k.PageDown, k.PageUp}
}

// SetSize keeps the room for the panel and gives the pane what is inside it.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.pane.SetSize(kit.BoxInner(width, height))
}

// Update handles keys and loaded weeks.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, kit.Keys.Previous):
			return m.load(m.monday.AddDate(0, 0, -7))
		case key.Matches(msg, kit.Keys.Next):
			return m.load(m.monday.AddDate(0, 0, 7))
		case key.Matches(msg, kit.Keys.Today):
			return m.load(vault.MondayOf(kit.Midnight(m.now())))
		case key.Matches(msg, kit.Keys.Edit):
			if m.week != nil {
				return m.editor.Edit(m.week.Path)
			}
			return nil
		}
		m.pane.Scroll(msg)
		return nil
	case weekMsg:
		if !msg.monday.Equal(m.monday) {
			return nil
		}
		m.show(msg.week, msg.err)
	case kit.EditedMsg:
		return m.load(m.monday)
	}
	return nil
}

// load shows the week starting monday and fetches its agenda.
func (m *Model) load(monday time.Time) tea.Cmd {
	m.monday, m.week = monday, nil
	m.pane.SetHeading(m.heading(), "")
	return func() tea.Msg {
		week, err := m.agenda.Agenda(m.ctx, monday)
		return weekMsg{monday: monday, week: week, err: err}
	}
}

// show renders the week, a heading per day, scrolled to today.
func (m *Model) show(week *app.AgendaWeek, err error) {
	m.week = week
	m.pane.SetError(err)
	if week == nil {
		return
	}
	m.pane.SetHeading(m.heading(), week.Path)
	today := kit.Midnight(m.now()).Format("2006-01-02")
	var b strings.Builder
	todayHeading := ""
	for _, d := range week.Days {
		heading := d.Heading
		if d.Date == today {
			heading += todayMark
			todayHeading = heading
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", heading, strings.TrimSpace(d.Content))
	}
	if len(week.Days) == 0 {
		b.Write(parser.Body([]byte(week.Content)))
	}
	m.pane.SetMarkdown(b.String())
	if todayHeading != "" {
		m.pane.ScrollTo(todayHeading)
	}
}

// heading names the week shown, e.g. "Week 44 · 26 oktober – 1 november 2026".
func (m *Model) heading() string {
	_, week := m.monday.ISOWeek()
	sunday := m.monday.AddDate(0, 0, 6)
	from := kit.DutchDayMonth(m.monday)
	if m.monday.Year() != sunday.Year() {
		from += fmt.Sprintf(" %d", m.monday.Year())
	}
	to := fmt.Sprintf("%s %d", kit.DutchDayMonth(sunday), sunday.Year())
	return fmt.Sprintf("Week %d · %s – %s", week, from, to)
}

// View draws the pane in a focused panel.
func (m *Model) View() string { return kit.Box(m.pane.View(), m.width, m.height, true) }
