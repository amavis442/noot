package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// fakeView records what the root sends it.
type fakeView struct {
	title     string
	capturing bool
	inits     int
	msgs      []tea.Msg
	w, h      int
}

type initMsg struct{ title string }

func (f *fakeView) Title() string { return f.title }
func (f *fakeView) Init() tea.Cmd {
	f.inits++
	return func() tea.Msg { return initMsg{f.title} }
}
func (f *fakeView) Update(msg tea.Msg) tea.Cmd { f.msgs = append(f.msgs, msg); return nil }
func (f *fakeView) View() string               { return "body of " + f.title }
func (f *fakeView) SetSize(w, h int)           { f.w, f.h = w, h }
func (f *fakeView) Capturing() bool            { return f.capturing }
func (f *fakeView) Help() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "keys of "+f.title))}
}

func newTestApp() (Model, []*fakeView) {
	views := []*fakeView{{title: "Zoeken"}, {title: "Vandaag"}, {title: "Agenda"}}
	kv := make([]kit.View, len(views))
	for i, v := range views {
		kv[i] = v
	}
	return New("persoonlijk", kv...), views
}

func press(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestInitLoadsTheFirstTabOnly(t *testing.T) {
	m, views := newTestApp()
	if cmd := m.Init(); cmd == nil || cmd() != (initMsg{"Zoeken"}) {
		t.Fatal("Init should load the first tab")
	}
	if views[1].inits+views[2].inits != 0 {
		t.Fatal("other tabs load when opened")
	}
}

func TestWindowSizeGoesToEveryViewMinusChrome(t *testing.T) {
	m, views := newTestApp()
	update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	for _, v := range views {
		if v.w != 100 || v.h != 30-chromeHeight {
			t.Fatalf("%s got %dx%d", v.title, v.w, v.h)
		}
	}
}

func TestNumberKeysSwitchViewsAndReload(t *testing.T) {
	m, views := newTestApp()
	m, cmd := update(t, m, press("3"))
	if m.active != 2 || views[2].inits != 1 || cmd == nil {
		t.Fatalf("3: active %d, inits %d", m.active, views[2].inits)
	}
	if m, _ = update(t, m, press("9")); m.active != 2 {
		t.Fatal("a number without a tab changes nothing")
	}
}

func TestTabBelongsToTheView(t *testing.T) {
	m, views := newTestApp()
	m, _ = update(t, m, press("tab"))
	m, _ = update(t, m, press("shift+tab"))
	if m.active != 0 || len(views[0].msgs) != 2 {
		t.Fatalf("tab and shift+tab switch panels inside the view: active %d, msgs %v", m.active, views[0].msgs)
	}
}
func TestKeysGoToTheActiveViewOnly(t *testing.T) {
	m, views := newTestApp()
	update(t, m, press("j"))
	if len(views[0].msgs) != 1 || len(views[1].msgs) != 0 {
		t.Fatalf("msgs: %v / %v", views[0].msgs, views[1].msgs)
	}
}

func TestOtherMessagesGoToEveryView(t *testing.T) {
	m, views := newTestApp()
	update(t, m, kit.EditedMsg{Rel: "a.md"})
	for _, v := range views {
		if len(v.msgs) != 1 {
			t.Fatalf("%s got %v", v.title, v.msgs)
		}
	}
}

func TestEditorErrorsShowUntilTheNextKey(t *testing.T) {
	m, _ := newTestApp()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = update(t, m, kit.EditedMsg{Rel: "a.md", Err: errors.New(`exec: "vim": not found`)})
	if out := m.View(); !strings.Contains(out, `"vim": not found`) {
		t.Fatalf("editor error missing:\n%s", out)
	}
	m, _ = update(t, m, press("j"))
	if out := m.View(); strings.Contains(out, "not found") {
		t.Fatalf("error still shown after a key:\n%s", out)
	}
}

func TestQuitUnlessAViewCaptures(t *testing.T) {
	m, views := newTestApp()
	for _, k := range []string{"q", "esc", "ctrl+c"} {
		if _, cmd := update(t, m, press(k)); !isQuit(cmd) {
			t.Fatalf("%s quits", k)
		}
	}
	views[0].capturing = true
	for _, k := range []string{"q", "esc", "ctrl+c", "2", "?"} {
		before := len(views[0].msgs)
		m2, cmd := update(t, m, press(k))
		if isQuit(cmd) || m2.active != 0 || m2.help || len(views[0].msgs) != before+1 {
			t.Fatalf("%s goes to the view while it captures", k)
		}
	}
}

func TestHelpOverlayListsAllKeys(t *testing.T) {
	m, views := newTestApp()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = update(t, m, press("?"))
	out := m.View()
	for _, want := range []string{"keys of Zoeken", "1-4", "q/esc", "?"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "body of Zoeken") {
		t.Fatalf("help replaces the view:\n%s", out)
	}
	m, cmd := update(t, m, press("esc"))
	if isQuit(cmd) || m.help || len(views[0].msgs) != 0 {
		t.Fatal("any key closes the help, without reaching the view or quitting")
	}
}
func TestViewShowsTabsActiveBodyAndHelp(t *testing.T) {
	m, _ := newTestApp()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, _ = update(t, m, press("2"))
	out := m.View()
	for _, want := range []string{"noot", "persoonlijk", "1 Zoeken", "2 Vandaag", "3 Agenda", "body of Vandaag", "keys of Vandaag", "? alle toetsen"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "body of Zoeken") {
		t.Fatal("only the active view is shown")
	}
}
