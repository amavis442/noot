// Package reminders is the reminders tab of the terminal UI: the list of
// reminders, to add to, tick off or plan into the agenda.
package reminders

import (
	"fmt"
	"strings"
	"time"

	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/vault"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Reminders is what the tab needs from the app layer.
type Reminders interface {
	Reminders(all bool) ([]vault.Reminder, error)
	AddReminder(text string, due, now time.Time) (vault.Reminder, error)
	CompleteReminder(n int, now time.Time) (vault.Reminder, error)
	PlanReminder(n int, day time.Time) (vault.Reminder, string, error)
}

// dayHint says which days the date fields accept.
const dayHint = "vandaag, morgen, vrijdag, +3 of 2026-11-04"

// mode is what the keyboard is doing.
type mode int

const (
	browsing mode = iota
	addingText
	addingDue
	planning
)

// listMsg carries the reminders, and what was done before listing them.
type listMsg struct {
	list   []vault.Reminder
	notice string
	err    error
}

// Model is the reminders tab.
type Model struct {
	reminders Reminders
	editor    kit.Editor
	now       func() time.Time

	list   []vault.Reminder
	all    bool // done reminders are listed too
	cursor int
	mode   mode
	input  textinput.Model
	text   string // the text of the reminder being added
	notice string
	err    error
	width  int
	height int
}

// New returns the reminders tab.
func New(r Reminders, ed kit.Editor, now func() time.Time) *Model {
	in := textinput.New()
	in.Cursor.SetMode(cursor.CursorStatic)
	return &Model{reminders: r, editor: ed, now: now, input: in}
}

// Title is the tab's label.
func (m *Model) Title() string { return "Herinneringen" }

// Init loads the list.
func (m *Model) Init() tea.Cmd { return m.reload() }

// Capturing is true while a text field is open.
func (m *Model) Capturing() bool { return m.mode != browsing }

// Help lists the keys.
func (m *Model) Help() []key.Binding {
	k := kit.Keys
	if m.mode != browsing {
		return []key.Binding{k.ConfirmTyping, k.CancelTyping}
	}
	return []key.Binding{k.ListDown, k.ListUp, k.Create, k.Done, k.Plan, k.ToggleDone, k.Edit}
}

// SetSize sets the room for the panel.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.input.Width = max(width-24, 10)
}

// rows is the number of reminders the panel shows at once.
func (m *Model) rows() int {
	_, h := kit.BoxInner(m.width, m.height)
	return max(h-2, 1)
}

// Update handles keys and the reloaded list.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.mode != browsing {
			return m.fieldKey(msg)
		}
		return m.listKey(msg)
	case listMsg:
		m.list, m.notice, m.err = msg.list, msg.notice, msg.err
		m.cursor = min(m.cursor, max(len(m.list)-1, 0))
	case kit.EditedMsg:
		return m.reload()
	}
	return nil
}

func (m *Model) listKey(msg tea.KeyMsg) tea.Cmd {
	m.notice, m.err = "", nil
	k := kit.Keys
	switch {
	case key.Matches(msg, k.ListUp):
		m.cursor = max(m.cursor-1, 0)
	case key.Matches(msg, k.ListDown):
		m.cursor = min(m.cursor+1, max(len(m.list)-1, 0))
	case key.Matches(msg, k.PageUp):
		m.cursor = max(m.cursor-m.rows(), 0)
	case key.Matches(msg, k.PageDown):
		m.cursor = min(m.cursor+m.rows(), max(len(m.list)-1, 0))
	case key.Matches(msg, k.ToggleDone):
		m.all = !m.all
		return m.reload()
	case key.Matches(msg, k.Create):
		m.open(addingText, "Nieuw: ", "wat moet er gebeuren?")
	case key.Matches(msg, k.Done):
		if r, ok := m.selected(); ok {
			return m.complete(r)
		}
	case key.Matches(msg, k.Plan):
		if _, ok := m.selected(); ok {
			m.open(planning, "Plannen op: ", "leeg is vandaag; "+dayHint)
		}
	case key.Matches(msg, k.Edit):
		return m.editor.Edit(vault.RemindersFile)
	}
	return nil
}

// fieldKey handles typing; superfile's typing keys win over all others.
func (m *Model) fieldKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, kit.Keys.CancelTyping):
		m.close()
		return nil
	case key.Matches(msg, kit.Keys.ConfirmTyping):
		return m.confirm()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// confirm acts on the text field's value for the current mode.
func (m *Model) confirm() tea.Cmd {
	value := strings.TrimSpace(m.input.Value())
	m.err = nil
	switch m.mode {
	case addingText:
		if value == "" {
			m.close()
			return nil
		}
		m.text = value
		m.open(addingDue, "Uiterlijk: ", "leeg is geen datum; "+dayHint)
	case addingDue:
		var due time.Time
		if value != "" {
			d, err := vault.ParseDay(value, m.now())
			if err != nil {
				m.err = err
				return nil
			}
			due = d
		}
		m.close()
		return m.add(m.text, due)
	case planning:
		if value == "" {
			value = "today"
		}
		day, err := vault.ParseDay(value, m.now())
		if err != nil {
			m.err = err
			return nil
		}
		m.close()
		if r, ok := m.selected(); ok {
			return m.plan(r, day)
		}
	}
	return nil
}

func (m *Model) open(md mode, prompt, placeholder string) {
	m.mode = md
	m.input.Prompt, m.input.Placeholder = prompt, placeholder
	m.input.SetValue("")
	m.input.Focus()
}

func (m *Model) close() {
	m.mode = browsing
	m.input.Blur()
}

func (m *Model) selected() (vault.Reminder, bool) {
	if m.cursor < len(m.list) {
		return m.list[m.cursor], true
	}
	return vault.Reminder{}, false
}

func (m *Model) add(text string, due time.Time) tea.Cmd {
	return func() tea.Msg {
		r, err := m.reminders.AddReminder(text, due, m.now())
		return m.listed("Toegevoegd: "+r.Text, err)
	}
}

func (m *Model) complete(r vault.Reminder) tea.Cmd {
	return func() tea.Msg {
		_, err := m.reminders.CompleteReminder(r.N, m.now())
		return m.listed("Afgevinkt: "+r.Text, err)
	}
}

func (m *Model) plan(r vault.Reminder, day time.Time) tea.Cmd {
	return func() tea.Msg {
		_, _, err := m.reminders.PlanReminder(r.N, day)
		return m.listed("Gepland op "+kit.DutchDate(day)+": "+r.Text, err)
	}
}

// reload lists the reminders again.
func (m *Model) reload() tea.Cmd {
	return func() tea.Msg { return m.listed("", nil) }
}

// listed lists the reminders after an action that said notice or failed with err.
func (m *Model) listed(notice string, err error) listMsg {
	if err != nil {
		notice = ""
	}
	list, lerr := m.reminders.Reminders(m.all)
	if err == nil {
		err = lerr
	}
	return listMsg{list: list, notice: notice, err: err}
}

// View draws the list with the text field or the last notice under it.
func (m *Model) View() string {
	var lines []string
	if len(m.list) == 0 {
		lines = append(lines, kit.Muted.Render("Geen open herinneringen. Druk ctrl+n voor een nieuwe."))
	}
	rows := m.rows()
	first := max(m.cursor-rows+1, 0)
	today := kit.Midnight(m.now()).Format("2006-01-02")
	for i := first; i < len(m.list) && i < first+rows; i++ {
		lines = append(lines, m.line(m.list[i], i == m.cursor, today))
	}
	var foot string
	switch {
	case m.mode != browsing:
		foot = m.input.View()
	case m.notice != "":
		foot = kit.Success.Render(m.notice)
	}
	if m.err != nil {
		foot = lipgloss.JoinVertical(lipgloss.Left, foot, kit.Error.Render(m.err.Error()))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, strings.Join(lines, "\n"), "", foot)
	return kit.Box(content, m.width, m.height, true)
}

// line draws one reminder: box, text and its dates.
func (m *Model) line(r vault.Reminder, selected bool, today string) string {
	box := "[ ]"
	if r.Done {
		box = "[x]"
	}
	text := fmt.Sprintf("%s %s", box, r.Text)
	var meta []string
	if r.Due != "" {
		due := "uiterlijk " + dayMonth(r.Due)
		if !r.Done && r.Due < today {
			meta = append(meta, kit.Error.Render(due+" · te laat"))
		} else {
			meta = append(meta, due)
		}
	}
	if r.Planned != "" {
		meta = append(meta, "gepland "+dayMonth(r.Planned))
	}
	if r.DoneOn != "" {
		meta = append(meta, "klaar "+dayMonth(r.DoneOn))
	}
	switch {
	case selected:
		text = kit.Selected.Render("▸ " + text)
	case r.Done:
		text = kit.Muted.Render("  " + text)
	default:
		text = "  " + text
	}
	if len(meta) > 0 {
		text += kit.Muted.Render("  " + strings.Join(meta, " · "))
	}
	w, _ := kit.BoxInner(m.width, m.height)
	return kit.Truncate(text, w)
}

// dayMonth turns YYYY-MM-DD into "2 november"; other text is kept.
func dayMonth(date string) string {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return date
	}
	return kit.DutchDayMonth(t)
}
