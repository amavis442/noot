// Package journal is the "today" tab of the terminal UI: the journal note of
// a day, one day at a time.
package journal

import (
	"context"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/tui/kit"
	tea "github.com/charmbracelet/bubbletea"
)

// Journal is what the tab needs from the app layer.
type Journal interface {
	Journal(ctx context.Context, day time.Time) (*app.JournalDay, error)
}

// dayMsg carries the journal note of day.
type dayMsg struct {
	day  time.Time
	note *app.JournalDay
	err  error
}

// Model is the journal tab.
type Model struct {
	ctx     context.Context
	journal Journal
	editor  kit.Editor
	now     func() time.Time

	day  time.Time // the day shown, at midnight
	note *app.JournalDay
	pane *kit.Pane
}

// New returns the journal tab, starting at today.
func New(ctx context.Context, j Journal, md kit.Markdown, ed kit.Editor, now func() time.Time) *Model {
	return &Model{ctx: ctx, journal: j, editor: ed, now: now, day: kit.Midnight(now()), pane: kit.NewPane(md)}
}

// Title is the tab's label.
func (m *Model) Title() string { return "Vandaag" }

// Init loads the day shown.
func (m *Model) Init() tea.Cmd { return m.load(m.day) }

// Capturing is always false: the tab has no text fields.
func (m *Model) Capturing() bool { return false }

// Help lists the keys.
func (m *Model) Help() string { return "←/→ dag · t vandaag · e bewerk · ↑/↓ scroll" }

// SetSize passes the room on to the pane.
func (m *Model) SetSize(width, height int) { m.pane.SetSize(width, height) }

// Update handles keys and loaded days.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			return m.load(m.day.AddDate(0, 0, -1))
		case "right", "l":
			return m.load(m.day.AddDate(0, 0, 1))
		case "t":
			return m.load(kit.Midnight(m.now()))
		case "e":
			if m.note != nil {
				return m.editor.Edit(m.note.Path)
			}
			return nil
		}
		return m.pane.Update(msg)
	case dayMsg:
		if !msg.day.Equal(m.day) {
			return nil
		}
		m.note = msg.note
		m.pane.SetError(msg.err)
		if msg.note != nil {
			m.pane.SetHeading(m.heading(), msg.note.Path)
			m.pane.SetMarkdown(msg.note.Content)
		}
	case kit.EditedMsg:
		return m.load(m.day)
	}
	return nil
}

// load shows day and fetches its note.
func (m *Model) load(day time.Time) tea.Cmd {
	m.day, m.note = day, nil
	m.pane.SetHeading(m.heading(), "")
	return func() tea.Msg {
		note, err := m.journal.Journal(m.ctx, day)
		return dayMsg{day: day, note: note, err: err}
	}
}

// View draws the pane.
func (m *Model) View() string { return m.pane.View() }

// heading names the day shown, marking today.
func (m *Model) heading() string {
	h := kit.DutchDate(m.day)
	if kit.SameDay(m.day, m.now()) {
		h += " · vandaag"
	}
	return h
}
