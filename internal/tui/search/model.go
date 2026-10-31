// Package search is the search tab of the terminal UI: results update while
// typing, with the selected note rendered next to them.
package search

import (
	"context"
	"strings"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Notes is what the search tab needs from the app layer.
type Notes interface {
	Search(ctx context.Context, q domain.SearchQuery) ([]domain.SearchHit, error)
	ReadNote(ctx context.Context, ref string) (*domain.Document, error)
	Sync(ctx context.Context, full bool) (app.SyncStats, error)
}

// limit is the number of results asked for.
const limit = 50

// hitsMsg answers search number seq.
type hitsMsg struct {
	seq  int
	hits []domain.SearchHit
	err  error
}

// noteMsg carries the note to preview.
type noteMsg struct {
	path string
	doc  *domain.Document
	err  error
}

// syncedMsg follows the index update after editing.
type syncedMsg struct{ err error }

// Model is the search tab.
type Model struct {
	ctx    context.Context
	notes  Notes
	editor kit.Editor

	input   textinput.Model
	inInput bool // the search field has the keyboard, else the results
	seq     int  // number of the latest search; older answers are dropped
	hits    []domain.SearchHit
	cursor  int
	err     error
	preview *kit.Pane
	width   int
	height  int
}

// New returns the search tab.
func New(ctx context.Context, notes Notes, md kit.Markdown, ed kit.Editor) *Model {
	in := textinput.New()
	in.Prompt = "› "
	in.Placeholder = "zoek in je notities, #tag filtert"
	in.Cursor.SetMode(cursor.CursorStatic)
	in.Focus()
	return &Model{ctx: ctx, notes: notes, editor: ed, input: in, inInput: true, preview: kit.NewPane(md)}
}

// Title is the tab's label.
func (m *Model) Title() string { return "Zoeken" }

// Init runs the current query again, so the results are fresh.
func (m *Model) Init() tea.Cmd {
	if strings.TrimSpace(m.input.Value()) == "" {
		return nil
	}
	return m.search()
}

// Capturing is true while the search field has the keyboard.
func (m *Model) Capturing() bool { return m.inInput }

// Help lists the keys.
func (m *Model) Help() string {
	if m.inInput {
		return "↑/↓ kies · enter naar resultaten · esc naar resultaten"
	}
	return "j/k kies · e bewerk · pgup/pgdn scroll · / zoek"
}

// SetSize divides the room between the results and the preview.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.input.Width = max(width-4, 10)
	m.preview.SetSize(m.previewWidth(), max(height-2, 1))
}

func (m *Model) listWidth() int { return min(max(m.width*2/5, 24), 50) }

func (m *Model) previewWidth() int { return max(m.width-m.listWidth()-3, 10) }

// Update handles keys and the answers of searches, note reads and syncs.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inInput {
			return m.inputKey(msg)
		}
		return m.listKey(msg)
	case hitsMsg:
		if msg.seq != m.seq {
			return nil
		}
		// A refresh keeps the selected note selected when it is still found.
		selected := m.selectedPath()
		m.hits, m.err, m.cursor = msg.hits, msg.err, 0
		for i, h := range m.hits {
			if h.Path == selected {
				m.cursor = i
			}
		}
		return m.loadPreview()
	case noteMsg:
		if msg.path != m.selectedPath() {
			return nil
		}
		m.showPreview(msg.doc, msg.err)
	case kit.EditedMsg:
		return m.sync()
	case syncedMsg:
		if msg.err != nil {
			m.err = msg.err
			return nil
		}
		return m.Init()
	}
	return nil
}

func (m *Model) inputKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "ctrl+p":
		return m.move(-1)
	case "down", "ctrl+n":
		return m.move(1)
	case "enter", "esc":
		m.inInput = false
		m.input.Blur()
		return nil
	}
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() == before {
		return cmd
	}
	if strings.TrimSpace(m.input.Value()) == "" {
		m.seq++
		m.hits, m.err = nil, nil
		m.showPreview(nil, nil)
		return cmd
	}
	return tea.Batch(cmd, m.search())
}

func (m *Model) listKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "/", "i":
		m.inInput = true
		m.input.Focus()
	case "up", "k":
		return m.move(-1)
	case "down", "j":
		return m.move(1)
	case "e", "enter":
		if p := m.selectedPath(); p != "" {
			return m.editor.Edit(p)
		}
	default:
		return m.preview.Update(msg)
	}
	return nil
}

// search starts a new search for the query; "#word" parts filter on tags.
func (m *Model) search() tea.Cmd {
	m.seq++
	seq := m.seq
	q := domain.SearchQuery{Limit: limit}
	var words []string
	for _, w := range strings.Fields(m.input.Value()) {
		if tag := strings.TrimPrefix(w, "#"); tag != w && tag != "" {
			q.Tags = append(q.Tags, tag)
		} else {
			words = append(words, w)
		}
	}
	q.Text = strings.Join(words, " ")
	return func() tea.Msg {
		hits, err := m.notes.Search(m.ctx, q)
		return hitsMsg{seq: seq, hits: hits, err: err}
	}
}

func (m *Model) move(delta int) tea.Cmd {
	next := min(max(m.cursor+delta, 0), len(m.hits)-1)
	if next < 0 || next == m.cursor {
		return nil
	}
	m.cursor = next
	return m.loadPreview()
}

func (m *Model) selectedPath() string {
	if m.cursor < len(m.hits) {
		return m.hits[m.cursor].Path
	}
	return ""
}

func (m *Model) loadPreview() tea.Cmd {
	path := m.selectedPath()
	if path == "" {
		m.showPreview(nil, nil)
		return nil
	}
	return func() tea.Msg {
		doc, err := m.notes.ReadNote(m.ctx, path)
		return noteMsg{path: path, doc: doc, err: err}
	}
}

func (m *Model) sync() tea.Cmd {
	return func() tea.Msg {
		_, err := m.notes.Sync(m.ctx, false)
		return syncedMsg{err: err}
	}
}

// showPreview shows doc, or err, next to the results; nil clears the preview.
func (m *Model) showPreview(doc *domain.Document, err error) {
	m.preview.SetError(err)
	if doc == nil {
		m.preview.SetHeading("", "")
		m.preview.SetMarkdown("")
		return
	}
	m.preview.SetHeading(doc.Title, doc.Path)
	m.preview.SetMarkdown(doc.Content)
}

// View draws the search field above the results and the preview.
func (m *Model) View() string {
	rows := m.height - 2
	var list string
	switch {
	case m.err != nil:
		list = kit.Error.Render(kit.Truncate(m.err.Error(), m.listWidth()))
	case strings.TrimSpace(m.input.Value()) == "":
		list = kit.Muted.Render("Typ om te zoeken.")
	case len(m.hits) == 0:
		list = kit.Muted.Render("Geen resultaten.")
	default:
		list = m.hitList(rows)
	}
	left := lipgloss.NewStyle().Width(m.listWidth()).Height(rows).MaxHeight(rows).Render(list)
	sep := kit.Divider.Render(strings.TrimSuffix(strings.Repeat("│\n", max(rows, 1)), "\n"))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", sep, " ", m.preview.View())
	return lipgloss.JoinVertical(lipgloss.Left, m.input.View(), "", body)
}

// hitList shows two lines per hit, scrolled so the selection stays visible.
func (m *Model) hitList(rows int) string {
	per := max(rows/2, 1)
	first := max(m.cursor-per+1, 0)
	w := m.listWidth()
	var b strings.Builder
	for i := first; i < len(m.hits) && i < first+per; i++ {
		h := m.hits[i]
		title := h.Title
		if title == "" {
			title = h.Path
		}
		if i == m.cursor {
			b.WriteString(kit.Selected.Render(kit.Truncate("▸ "+title, w)))
		} else {
			b.WriteString(kit.Truncate("  "+title, w))
		}
		b.WriteString("\n" + kit.Muted.Render(kit.Truncate("  "+h.Path, w)) + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
