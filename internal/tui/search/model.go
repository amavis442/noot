// Package search is the search tab of the terminal UI: results update while
// typing, with the selected note rendered next to them.
package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
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

// focus is the part of the tab that has the keyboard.
type focus int

const (
	onSearchBar focus = iota
	onResults
)

// searchBarHeight is the boxed search field above the panels.
const searchBarHeight = 3

// Model is the search tab: a search bar over two panels, the results and the
// preview of the selected note. The keys follow superfile: the results keep
// the arrows, confirm opens the note, and the preview never takes the
// keyboard; it scrolls with J/K.
type Model struct {
	ctx    context.Context
	notes  Notes
	editor kit.Editor

	input       textinput.Model
	focus       focus
	showPreview bool
	seq         int // number of the latest search; older answers are dropped
	hits        []domain.SearchHit
	cursor      int
	err         error
	preview     *kit.Pane
	width       int
	height      int
}

// New returns the search tab, typing in the search bar.
func New(ctx context.Context, notes Notes, md kit.Markdown, ed kit.Editor) *Model {
	in := textinput.New()
	in.Prompt = "/ "
	in.Placeholder = "zoek in je notities, #tag filtert"
	in.Cursor.SetMode(cursor.CursorStatic)
	in.Focus()
	return &Model{ctx: ctx, notes: notes, editor: ed, input: in, focus: onSearchBar, showPreview: true, preview: kit.NewPane(md)}
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

// Capturing is true while typing in the search bar.
func (m *Model) Capturing() bool { return m.focus == onSearchBar }

// Help lists the keys of the part that has the keyboard.
func (m *Model) Help() []key.Binding {
	k := kit.Keys
	if m.focus == onSearchBar {
		return []key.Binding{k.ConfirmTyping, k.CancelTyping, tabToResults, arrows}
	}
	return []key.Binding{k.ListDown, k.ListUp, k.Confirm, k.PreviewDown, k.PreviewUp, k.PageDown, k.PageUp, k.NextPanel, k.PreviousPanel, k.SearchBar, k.TogglePreview}
}

// Keys of the search bar that only exist while typing.
var (
	tabToResults = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "naar resultaten"))
	arrows       = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "kies"))
)

// SetSize divides the room between the search bar and the panels.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.input.Width = max(width-8, 10)
	m.preview.SetSize(kit.BoxInner(m.previewWidth(), m.panelHeight()))
}

func (m *Model) panelHeight() int { return max(m.height-searchBarHeight, 3) }

// listWidth is the results panel's width; it takes all of it without preview.
func (m *Model) listWidth() int {
	if !m.showPreview {
		return m.width
	}
	return min(max(m.width*2/5, 28), 56)
}

func (m *Model) previewWidth() int { return max(m.width-m.listWidth(), 10) }

// Update handles keys and the answers of searches, note reads and syncs.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.focus == onSearchBar {
			return m.searchBarKey(msg)
		}
		return m.resultsKey(msg)
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
		m.setPreview(msg.doc, msg.err)
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

// searchBarKey handles typing. Superfile's typing keys win over all others;
// tab also leaves the bar, so the results can be picked right away.
func (m *Model) searchBarKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, kit.Keys.ConfirmTyping, kit.Keys.CancelTyping, tabToResults):
		m.setFocus(onResults)
		return nil
	case msg.Type == tea.KeyUp:
		return m.move(-1)
	case msg.Type == tea.KeyDown:
		return m.move(1)
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
		m.setPreview(nil, nil)
		return cmd
	}
	return tea.Batch(cmd, m.search())
}

func (m *Model) resultsKey(msg tea.KeyMsg) tea.Cmd {
	k := kit.Keys
	switch {
	case key.Matches(msg, k.ListUp):
		return m.move(-1)
	case key.Matches(msg, k.ListDown):
		return m.move(1)
	case key.Matches(msg, k.PageUp):
		return m.move(-m.perPage())
	case key.Matches(msg, k.PageDown):
		return m.move(m.perPage())
	case key.Matches(msg, k.Confirm, k.Edit):
		return m.edit()
	case key.Matches(msg, k.PreviewDown):
		m.preview.ScrollLines(1)
	case key.Matches(msg, k.PreviewUp):
		m.preview.ScrollLines(-1)
	case key.Matches(msg, k.NextPanel, k.PreviousPanel, k.SearchBar):
		m.setFocus(onSearchBar)
	case key.Matches(msg, k.TogglePreview):
		m.togglePreview()
	}
	return nil
}
func (m *Model) setFocus(f focus) {
	m.focus = f
	if f == onSearchBar {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

func (m *Model) togglePreview() {
	m.showPreview = !m.showPreview
	m.SetSize(m.width, m.height)
}

func (m *Model) edit() tea.Cmd {
	if p := m.selectedPath(); p != "" {
		return m.editor.Edit(p)
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
		m.setPreview(nil, nil)
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

// setPreview shows doc, or err, next to the results; nil clears the preview.
func (m *Model) setPreview(doc *domain.Document, err error) {
	m.preview.SetError(err)
	if doc == nil {
		m.preview.SetHeading("", "")
		m.preview.SetMarkdown("")
		return
	}
	m.preview.SetHeading(doc.Title, doc.Path)
	m.preview.SetMarkdown(doc.Content)
}

// View draws the search bar above the results and the preview panels.
func (m *Model) View() string {
	bar := kit.Box(m.input.View(), m.width, searchBarHeight, m.focus == onSearchBar)
	h := m.panelHeight()
	_, rows := kit.BoxInner(m.listWidth(), h)
	results := kit.Box(m.results(rows), m.listWidth(), h, m.focus == onResults)
	if !m.showPreview {
		return lipgloss.JoinVertical(lipgloss.Left, bar, results)
	}
	preview := kit.Box(m.preview.View(), m.previewWidth(), h, false)
	return lipgloss.JoinVertical(lipgloss.Left, bar, lipgloss.JoinHorizontal(lipgloss.Top, results, preview))
}

// results is the content of the results panel.
func (m *Model) results(rows int) string {
	w, _ := kit.BoxInner(m.listWidth(), 0)
	switch {
	case m.err != nil:
		return kit.Error.Render(kit.Truncate(m.err.Error(), w))
	case strings.TrimSpace(m.input.Value()) == "":
		return kit.Muted.Render("Typ om te zoeken.")
	case len(m.hits) == 0:
		return kit.Muted.Render("Geen resultaten.")
	}
	return m.hitList(rows, w)
}

// countLines is the count line on top of the results and the blank under it.
const countLines = 2

// perPage is the number of hits the results panel shows at once.
func (m *Model) perPage() int {
	_, rows := kit.BoxInner(m.listWidth(), m.panelHeight())
	return max(rows-countLines, 1)
}

// hitList shows the number of hits, then one line per hit: the title with
// its path dimmed after it, scrolled so the selection stays visible. A title
// and path on lines of their own looked like two hits.
func (m *Model) hitList(rows, w int) string {
	count := fmt.Sprintf("%d resultaten", len(m.hits))
	if len(m.hits) == 1 {
		count = "1 resultaat"
	}
	lines := []string{kit.Muted.Render(count), ""}
	per := max(rows-countLines, 1)
	first := max(m.cursor-per+1, 0)
	for i := first; i < len(m.hits) && i < first+per; i++ {
		lines = append(lines, m.hitLine(m.hits[i], i == m.cursor, w))
	}
	return strings.Join(lines, "\n")
}

// hitLine is "▸ Title  path", cut to w cells; the path is left out when the title fills the line.
func (m *Model) hitLine(h domain.SearchHit, selected bool, w int) string {
	title := h.Title
	if title == "" {
		title = h.Path
	}
	marker, style := "  ", lipgloss.NewStyle()
	if selected {
		marker, style = "▸ ", kit.Selected
	}
	head := marker + title
	if lipgloss.Width(head) >= w || h.Path == title {
		return style.Render(kit.Truncate(head, w))
	}
	return style.Render(head) + kit.Muted.Render(kit.Truncate("  "+h.Path, w-lipgloss.Width(head)))
}
