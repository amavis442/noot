// Package todo is the todo tab of the terminal UI: every task of the vault in
// one list, grouped by when it is due, to tick off from here. The tab only
// reads the vault and ticks lines; there is no list of its own to keep up to date.
package todo

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/vault"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Todo is what the tab needs from the app layer.
type Todo interface {
	Tasks(scope app.TodoScope, now time.Time) ([]domain.Task, error)
	CompleteTask(t domain.Task, now time.Time) error
	PlanTask(t domain.Task, day time.Time) (string, error)
}

// dayHint says which days the planning field accepts.
const dayHint = "vandaag, morgen, vrijdag, +3 of 2026-11-04"

// The tab's own keys; the rest come from kit.Keys.
var (
	tick     = key.NewBinding(key.WithKeys(" ", "x"), key.WithHelp("spatie/x", "afvinken"))
	showDone = key.NewBinding(key.WithKeys("a", "."), key.WithHelp("a", "klaar aan/uit"))
)

// errChanged is shown when a tick found another line than the one listed.
var errChanged = errors.New("de taak is intussen veranderd in het bestand; de lijst is opnieuw geladen")

// groupLabels are the headings of the groups.
var groupLabels = map[domain.TaskGroup]string{
	domain.GroupOverdue:  "Achterstallig",
	domain.GroupToday:    "Vandaag",
	domain.GroupThisWeek: "Deze week",
	domain.GroupLater:    "Later",
	domain.GroupBacklog:  "Backlog",
	domain.GroupDone:     "Klaar",
}

// listMsg carries the tasks, and what was done before listing them.
type listMsg struct {
	list   []domain.Task
	notice string
	err    error
}

// Model is the todo tab.
type Model struct {
	todo   Todo
	editor kit.Editor
	now    func() time.Time

	list     []domain.Task // in the order they are shown
	all      bool          // done tasks are listed too
	cursor   int           // index in list
	planning bool
	input    textinput.Model
	notice   string
	err      error
	width    int
	height   int
}

// New returns the todo tab.
func New(t Todo, ed kit.Editor, now func() time.Time) *Model {
	in := textinput.New()
	in.Cursor.SetMode(cursor.CursorStatic)
	in.Prompt = "Plannen op: "
	in.Placeholder = "leeg is vandaag; " + dayHint
	return &Model{todo: t, editor: ed, now: now, input: in}
}

// Title is the tab's label.
func (m *Model) Title() string { return "Todo" }

// Init loads the list.
func (m *Model) Init() tea.Cmd { return m.reload() }

// Capturing is true while the planning field is open.
func (m *Model) Capturing() bool { return m.planning }

// Help lists the keys.
func (m *Model) Help() []key.Binding {
	k := kit.Keys
	if m.planning {
		return []key.Binding{k.ConfirmTyping, k.CancelTyping}
	}
	return []key.Binding{k.ListDown, k.ListUp, tick, k.Plan, showDone, k.Confirm}
}

// SetSize sets the room for the panel.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.input.Width = max(width-24, 10)
}

// rows is the number of lines the panel shows at once.
func (m *Model) rows() int {
	_, h := kit.BoxInner(m.width, m.height)
	return max(h-2, 1)
}

// Update handles keys and the reloaded list.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.planning {
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
	case key.Matches(msg, showDone):
		m.all = !m.all
		return m.reload()
	case key.Matches(msg, tick):
		if t, ok := m.selected(); ok && !t.Done {
			return m.complete(t)
		}
	case key.Matches(msg, k.Plan):
		t, ok := m.selected()
		switch {
		case !ok || t.Done:
		case t.Source != domain.SourceReminders:
			m.err = errors.New("Alleen herinneringen kun je hier plannen; dit item staat al in " + t.File)
		default:
			m.planning = true
			m.input.SetValue("")
			m.input.Focus()
		}
	case key.Matches(msg, k.Confirm), key.Matches(msg, k.Edit):
		if t, ok := m.selected(); ok {
			return m.editor.Edit(t.File)
		}
	}
	return nil
}

// fieldKey handles typing in the planning field; superfile's typing keys win.
func (m *Model) fieldKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, kit.Keys.CancelTyping):
		m.planning = false
		m.input.Blur()
		m.err = nil
		return nil
	case key.Matches(msg, kit.Keys.ConfirmTyping):
		return m.confirmPlan()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) confirmPlan() tea.Cmd {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		value = "today"
	}
	day, err := vault.ParseDay(value, m.now())
	if err != nil {
		m.err = err
		return nil
	}
	m.err = nil
	m.planning = false
	m.input.Blur()
	if t, ok := m.selected(); ok {
		return m.plan(t, day)
	}
	return nil
}

func (m *Model) selected() (domain.Task, bool) {
	if m.cursor < len(m.list) {
		return m.list[m.cursor], true
	}
	return domain.Task{}, false
}

func (m *Model) complete(t domain.Task) tea.Cmd {
	return func() tea.Msg {
		err := m.todo.CompleteTask(t, m.now())
		if errors.Is(err, vault.ErrTaskChanged) {
			err = errChanged
		}
		return m.listed("Afgevinkt: "+t.Text, err)
	}
}

func (m *Model) plan(t domain.Task, day time.Time) tea.Cmd {
	return func() tea.Msg {
		_, err := m.todo.PlanTask(t, day)
		if errors.Is(err, vault.ErrTaskChanged) {
			err = errChanged
		}
		return m.listed("Gepland op "+kit.DutchDate(day)+": "+t.Text, err)
	}
}

// reload lists the tasks again.
func (m *Model) reload() tea.Cmd {
	return func() tea.Msg { return m.listed("", nil) }
}

// listed lists the tasks after an action that said notice or failed with err.
func (m *Model) listed(notice string, err error) listMsg {
	if err != nil {
		notice = ""
	}
	scope := app.TodoOpen
	if m.all {
		scope = app.TodoAll
	}
	list, lerr := m.todo.Tasks(scope, m.now())
	if err == nil {
		err = lerr
	}
	return listMsg{list: list, notice: notice, err: err}
}

// row is one line of the panel: a group heading, or the task at index.
type row struct {
	heading string
	group   domain.TaskGroup
	index   int // into list; -1 for a heading
}

// layout returns the lines of the panel: a heading before the first task of
// each group.
func (m *Model) layout() []row {
	now := m.now()
	var rows []row
	var last domain.TaskGroup
	counts := map[domain.TaskGroup]int{}
	for _, t := range m.list {
		counts[t.Group(now)]++
	}
	for i, t := range m.list {
		g := t.Group(now)
		if g != last {
			rows = append(rows, row{heading: fmt.Sprintf("%s (%d)", groupLabels[g], counts[g]), group: g, index: -1})
			last = g
		}
		rows = append(rows, row{index: i})
	}
	return rows
}

// View draws the list with the planning field or the last notice under it.
func (m *Model) View() string {
	rows := m.layout()
	var lines []string
	if len(m.list) == 0 && m.err == nil {
		lines = append(lines, kit.Muted.Render("Niets te doen."))
	}
	at := 0 // the row of the cursor
	for i, r := range rows {
		if r.index == m.cursor {
			at = i
		}
	}
	height := m.rows()
	first := max(at-height+1, 0)
	today := kit.Midnight(m.now())
	for i := first; i < len(rows) && i < first+height; i++ {
		r := rows[i]
		switch {
		case r.index >= 0:
			lines = append(lines, m.line(m.list[r.index], r.index == m.cursor, today))
		case r.group == domain.GroupOverdue:
			lines = append(lines, kit.Error.Render(r.heading))
		default:
			lines = append(lines, kit.Heading.Render(r.heading))
		}
	}
	var foot string
	switch {
	case m.planning:
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

// line draws one task: box, text and its dates.
func (m *Model) line(t domain.Task, selected bool, today time.Time) string {
	box := "[ ]"
	if t.Done {
		box = "[x]"
	}
	text := box + " " + t.Text
	overdue := t.Group(today) == domain.GroupOverdue
	var meta []string
	add := func(label, date string) {
		s := label + " " + dayMonth(date)
		if overdue && date == t.Date() {
			meta = append(meta, kit.Error.Render(s+" · te laat"))
			return
		}
		meta = append(meta, s)
	}
	if t.Due != "" {
		add("uiterlijk", t.Due)
	}
	if t.Planned != "" {
		add("gepland", t.Planned)
	}
	if t.DoneOn != "" {
		meta = append(meta, "klaar "+dayMonth(t.DoneOn))
	}
	if t.Source == domain.SourceWeek {
		meta = append(meta, "agenda "+strings.TrimSuffix(path.Base(t.File), ".md"))
	}
	switch {
	case selected:
		text = kit.Selected.Render("▸ " + text)
	case t.Done:
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
