// Package agenda is the agenda tab of the terminal UI: the weekly agenda,
// day by day, opened at today, with one day or the whole week to edit.
package agenda

import (
	"context"
	"errors"
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
	SaveAgendaDay(ctx context.Context, day time.Time, content, version string) (*app.AgendaDay, error)
}

// todayMark follows today's heading.
const todayMark = " · vandaag"

// savedMsg answers the save of the day on date.
type savedMsg struct {
	date    string
	content string
	err     error
}

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
	editor kit.Editing
	now    func() time.Time

	monday   time.Time // the week shown
	week     *app.AgendaWeek
	selected string            // date of the selected day
	pending  map[string]string // edited text per date that could not be saved
	notice   string
	err      error
	pane     *kit.Pane
	width    int
	height   int
}

// New returns the agenda tab, starting at this week.
func New(ctx context.Context, a Agenda, md kit.Markdown, ed kit.Editing, now func() time.Time) *Model {
	return &Model{ctx: ctx, agenda: a, editor: ed, now: now, monday: vault.MondayOf(kit.Midnight(now())), pending: map[string]string{}, pane: kit.NewPane(md)}
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
	return []key.Binding{kit.WithDesc(k.ListDown, "volgende dag"), kit.WithDesc(k.ListUp, "vorige dag"), kit.WithDesc(k.Edit, "bewerk dag"), kit.WithDesc(k.EditAll, "bewerk week"),
		kit.WithDesc(k.Previous, "vorige week"), kit.WithDesc(k.Next, "volgende week"), kit.WithDesc(k.Today, "deze week"), k.PageDown, k.PageUp}
}

// SetSize keeps the room for the panel and gives the pane what is inside it,
// less the notice line.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	w, h := kit.BoxInner(width, height)
	m.pane.SetSize(w, max(h-1, 1))
}

// Update handles keys, loaded weeks, edited days and saves.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.key(msg)
	case weekMsg:
		if !msg.monday.Equal(m.monday) {
			return nil
		}
		m.show(msg.week, msg.err)
	case kit.TextEditedMsg:
		return m.save(msg)
	case savedMsg:
		if msg.err != nil {
			m.pending[msg.date] = msg.content
			m.notice, m.err = "", msg.err
			if errors.Is(msg.err, app.ErrConflict) {
				m.err = fmt.Errorf("%s is intussen gewijzigd; e opent jouw versie opnieuw", m.dayName(msg.date))
			}
			return m.load(m.monday)
		}
		delete(m.pending, msg.date)
		m.notice, m.err = "Opgeslagen: "+m.dayName(msg.date), nil
		return m.load(m.monday)
	case kit.EditedMsg:
		return m.load(m.monday)
	}
	return nil
}

func (m *Model) key(msg tea.KeyMsg) tea.Cmd {
	k := kit.Keys
	if !key.Matches(msg, k.PageDown, k.PageUp) {
		m.notice, m.err = "", nil
	}
	switch {
	case key.Matches(msg, k.Previous):
		return m.load(m.monday.AddDate(0, 0, -7))
	case key.Matches(msg, k.Next):
		return m.load(m.monday.AddDate(0, 0, 7))
	case key.Matches(msg, k.Today):
		return m.load(vault.MondayOf(kit.Midnight(m.now())))
	case key.Matches(msg, k.ListDown):
		m.selectDay(1)
	case key.Matches(msg, k.ListUp):
		m.selectDay(-1)
	case key.Matches(msg, k.Edit):
		if d, ok := m.day(); ok {
			text, kept := m.pending[d.Date]
			if !kept {
				text = d.Content
			}
			return m.editor.EditText(d.Date, text)
		}
	case key.Matches(msg, k.EditAll):
		if m.week != nil {
			return m.editor.Edit(m.week.Path)
		}
	default:
		m.pane.Scroll(msg)
	}
	return nil
}

// save stores an edited day, unless nothing changed. It checks the version
// of the day as it was loaded, so a change made elsewhere is not overwritten.
func (m *Model) save(msg kit.TextEditedMsg) tea.Cmd {
	d, ok := m.dayOn(msg.Name)
	switch {
	case msg.Err != nil:
		m.err = msg.Err
		return nil
	case !ok:
		return nil
	case strings.TrimSpace(msg.Content) == strings.TrimSpace(d.Content):
		delete(m.pending, d.Date)
		return nil
	}
	day, err := time.ParseInLocation("2006-01-02", d.Date, m.monday.Location())
	if err != nil {
		m.err = err
		return nil
	}
	return func() tea.Msg {
		_, err := m.agenda.SaveAgendaDay(m.ctx, day, msg.Content, d.Version)
		return savedMsg{date: d.Date, content: msg.Content, err: err}
	}
}

// selectDay moves the selection delta days within the week.
func (m *Model) selectDay(delta int) {
	if m.week == nil || len(m.week.Days) == 0 {
		return
	}
	i := m.index()
	m.selected = m.week.Days[min(max(i+delta, 0), len(m.week.Days)-1)].Date
	m.render()
}

// index is the position of the selected day, 0 when it is not in the week.
func (m *Model) index() int {
	for i, d := range m.week.Days {
		if d.Date == m.selected {
			return i
		}
	}
	return 0
}

func (m *Model) day() (app.AgendaDay, bool) {
	if m.week == nil || len(m.week.Days) == 0 {
		return app.AgendaDay{}, false
	}
	return m.week.Days[m.index()], true
}

func (m *Model) dayOn(date string) (app.AgendaDay, bool) {
	if m.week != nil {
		for _, d := range m.week.Days {
			if d.Date == date {
				return d, true
			}
		}
	}
	return app.AgendaDay{}, false
}

// dayName is a date as "vrijdag 30 oktober".
func (m *Model) dayName(date string) string {
	t, err := time.ParseInLocation("2006-01-02", date, m.monday.Location())
	if err != nil {
		return date
	}
	full := kit.DutchDate(t)
	return strings.TrimSuffix(full, fmt.Sprintf(" %d", t.Year()))
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

// show takes a loaded week: the selection stays on its day, or goes to today
// when it is in this week, else to the first day.
func (m *Model) show(week *app.AgendaWeek, err error) {
	m.week = week
	m.pane.SetError(err)
	if week == nil {
		return
	}
	m.pane.SetHeading(m.heading(), week.Path)
	if _, ok := m.dayOn(m.selected); !ok {
		m.selected = ""
		today := kit.Midnight(m.now()).Format("2006-01-02")
		if _, ok := m.dayOn(today); ok {
			m.selected = today
		} else if len(week.Days) > 0 {
			m.selected = week.Days[0].Date
		}
	}
	m.render()
}

// render draws the week, a heading per day, scrolled to the selected day.
func (m *Model) render() {
	today := kit.Midnight(m.now()).Format("2006-01-02")
	var b strings.Builder
	selectedHeading := ""
	for _, d := range m.week.Days {
		heading := d.Heading
		if d.Date == today {
			heading += todayMark
		}
		if d.Date == m.selected {
			heading = "▸ " + heading
			selectedHeading = heading
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", heading, strings.TrimSpace(d.Content))
	}
	if len(m.week.Days) == 0 {
		b.Write(parser.Body([]byte(m.week.Content)))
	}
	m.pane.SetMarkdown(b.String())
	if selectedHeading != "" {
		m.pane.ScrollTo(selectedHeading)
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

// View draws the pane and the notice line in a focused panel.
func (m *Model) View() string {
	line := kit.Success.Render(m.notice)
	if m.err != nil {
		line = kit.Error.Render(m.err.Error())
	}
	w, _ := kit.BoxInner(m.width, m.height)
	return kit.Box(m.pane.View()+"\n"+kit.Truncate(line, w), m.width, m.height, true)
}
