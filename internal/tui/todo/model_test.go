package todo_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/tui/kit/kittest"
	"github.com/amavis442/noot/internal/tui/todo"
	"github.com/amavis442/noot/internal/vault"
)

var today = time.Date(2026, 10, 31, 14, 0, 0, 0, time.Local) // a Saturday; the week ends tomorrow

// fakeTodo serves tasks from memory, already in the order the app returns them.
type fakeTodo struct {
	tasks   []domain.Task
	scopes  []app.TodoScope
	ticked  []string
	planned map[string]string // text -> day
	fail    error             // CompleteTask fails with this
}

func (f *fakeTodo) Tasks(scope app.TodoScope, _ time.Time) ([]domain.Task, error) {
	f.scopes = append(f.scopes, scope)
	var out []domain.Task
	for _, t := range f.tasks {
		if scope == app.TodoAll || !t.Done {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeTodo) CompleteTask(t domain.Task, now time.Time) error {
	if f.fail != nil {
		return f.fail
	}
	for i := range f.tasks {
		if f.tasks[i].File == t.File && f.tasks[i].Line == t.Line {
			f.tasks[i].Done, f.tasks[i].DoneOn = true, now.Format("2006-01-02")
		}
	}
	f.ticked = append(f.ticked, t.Text)
	return nil
}

func (f *fakeTodo) PlanTask(t domain.Task, day time.Time) (string, error) {
	if f.planned == nil {
		f.planned = map[string]string{}
	}
	f.planned[t.Text] = day.Format("2006-01-02")
	return "agenda/2026-W44.md", nil
}

func newModel() (*todo.Model, *fakeTodo, *kittest.Editor) {
	f := &fakeTodo{tasks: []domain.Task{
		{Source: domain.SourceReminders, File: "reminders.md", Line: 7, N: 1, Text: "Tandarts bellen", Due: "2026-10-29"},
		{Source: domain.SourceReminders, File: "reminders.md", Line: 8, N: 2, Text: "Brood bakken", Planned: "2026-11-01"},
		{Source: domain.SourceWeek, File: "agenda/2026-W45.md", Line: 12, Text: "Post wegbrengen", Planned: "2026-11-02", Section: "Planned"},
		{Source: domain.SourceBacklog, File: "agenda/agenda.md", Line: 9, Text: "**C# API** bouwen", Section: "Planned"},
		{Source: domain.SourceReminders, File: "reminders.md", Line: 9, N: 3, Text: "Oud klusje", Done: true, DoneOn: "2026-10-20"},
	}}
	ed := &kittest.Editor{}
	m := todo.New(f, ed, func() time.Time { return today })
	m.SetSize(100, 24)
	return m, f, ed
}

func TestListsTheTasksInGroups(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	out := m.View()
	for _, want := range []string{
		"Achterstallig (1)", "Tandarts bellen", "uiterlijk 29 oktober", "te laat",
		"Deze week (1)", "Brood bakken", "gepland 1 november",
		"Later (1)", "Post wegbrengen", "gepland 2 november", "2026-W45",
		"Backlog (1)", "**C# API** bouwen",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("view lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Oud klusje") || strings.Contains(out, "Klaar (") {
		t.Fatalf("done tasks are hidden at first:\n%s", out)
	}
	if !slices.Equal(f.scopes, []app.TodoScope{app.TodoOpen}) {
		t.Errorf("scopes = %v, want one open listing", f.scopes)
	}
	if strings.Index(out, "Achterstallig") > strings.Index(out, "Deze week") || strings.Index(out, "Later") > strings.Index(out, "Backlog") {
		t.Errorf("groups are out of order:\n%s", out)
	}
}

func TestAShowsAndHidesDoneTasks(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "a")
	out := m.View()
	if !strings.Contains(out, "Klaar (1)") || !strings.Contains(out, "Oud klusje") || !strings.Contains(out, "klaar 20 oktober") {
		t.Fatalf("a shows the done ones:\n%s", out)
	}
	kittest.Type(t, m, ".")
	if out := m.View(); strings.Contains(out, "Oud klusje") {
		t.Fatalf(". toggles back:\n%s", out)
	}
	if !slices.Equal(f.scopes, []app.TodoScope{app.TodoOpen, app.TodoAll, app.TodoOpen}) {
		t.Errorf("scopes = %v", f.scopes)
	}
}

func TestTheCursorSkipsHeadings(t *testing.T) {
	m, _, _ := newModel()
	kittest.Run(t, m, m.Init())
	if out := m.View(); !strings.Contains(out, "▸ [ ] Tandarts bellen") {
		t.Fatalf("the first task is selected:\n%s", out)
	}
	kittest.Type(t, m, "j")
	if out := m.View(); !strings.Contains(out, "▸ [ ] Brood bakken") {
		t.Fatalf("j selects the next task, not a heading:\n%s", out)
	}
	kittest.Type(t, m, "j", "j", "j", "j", "k")
	if out := m.View(); !strings.Contains(out, "▸ [ ] Post wegbrengen") {
		t.Fatalf("the cursor stops at the ends (4 tasks):\n%s", out)
	}
}

func TestSpaceAndXTickTheSelectedTask(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "j", "space")
	if !slices.Equal(f.ticked, []string{"Brood bakken"}) {
		t.Fatalf("ticked %v", f.ticked)
	}
	out := m.View()
	if !strings.Contains(out, "Afgevinkt: Brood bakken") || strings.Contains(out, "] Brood bakken") {
		t.Fatalf("view after ticking:\n%s", out)
	}
	kittest.Type(t, m, "x")
	if len(f.ticked) != 2 {
		t.Fatalf("x ticks too: %v", f.ticked)
	}
}

func TestTickingADoneTaskDoesNothing(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "a", "j", "j", "j", "j", "space")
	if len(f.ticked) != 0 {
		t.Fatalf("ticked %v", f.ticked)
	}
}

func TestAChangedLineIsReportedAndTheListReloaded(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	f.fail = vault.ErrTaskChanged
	n := len(f.scopes)
	kittest.Type(t, m, "space")
	out := m.View()
	if !strings.Contains(out, "veranderd") || strings.Contains(out, "Afgevinkt") {
		t.Fatalf("the changed line is reported:\n%s", out)
	}
	if len(f.scopes) != n+1 {
		t.Errorf("the list is loaded again: %d -> %d", n, len(f.scopes))
	}
}

func TestEnterAndEOpenTheSourceFile(t *testing.T) {
	m, _, ed := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "enter", "j", "j", "e")
	if !slices.Equal(ed.Opened, []string{"reminders.md", "agenda/2026-W45.md"}) {
		t.Fatalf("opened %v", ed.Opened)
	}
}

func TestPlanARemindersDay(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "p")
	if !m.Capturing() {
		t.Fatal("p opens a text field for a reminder")
	}
	kittest.Type(t, m, "tomorrow", "enter")
	if m.Capturing() || f.planned["Tandarts bellen"] != "2026-11-01" {
		t.Fatalf("planned %v", f.planned)
	}
	if out := m.View(); !strings.Contains(out, "Gepland op zondag 1 november 2026: Tandarts bellen") {
		t.Fatalf("view after planning:\n%s", out)
	}
	kittest.Type(t, m, "p", "enter")
	if f.planned["Tandarts bellen"] != "2026-10-31" {
		t.Fatalf("an empty day plans today: %v", f.planned)
	}
}

func TestOnlyRemindersCanBePlannedHere(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "j", "j", "j", "p")
	if m.Capturing() || len(f.planned) != 0 {
		t.Fatal("no field for a backlog item")
	}
	if out := m.View(); !strings.Contains(out, "Alleen herinneringen") {
		t.Fatalf("the reason is shown:\n%s", out)
	}
}

func TestABadDayKeepsTheFieldOpenAndEscCancels(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "p", "ooit", "enter")
	if !m.Capturing() || len(f.planned) != 0 {
		t.Fatal("a bad day plans nothing and keeps the field open")
	}
	if out := m.View(); !strings.Contains(out, "ooit") {
		t.Fatalf("the error names the day:\n%s", out)
	}
	kittest.Type(t, m, "esc")
	if m.Capturing() {
		t.Fatal("esc cancels")
	}
}

func TestEmptyList(t *testing.T) {
	m := todo.New(&fakeTodo{}, &kittest.Editor{}, func() time.Time { return today })
	m.SetSize(80, 20)
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "space", "p", "enter", "j")
	if out := m.View(); !strings.Contains(out, "Niets te doen") {
		t.Fatalf("view:\n%s", out)
	}
}

func TestAnErrorFromTheListIsShown(t *testing.T) {
	var f errTodo
	m := todo.New(f, &kittest.Editor{}, func() time.Time { return today })
	m.SetSize(80, 20)
	kittest.Run(t, m, m.Init())
	if out := m.View(); !strings.Contains(out, "disk on fire") {
		t.Fatalf("view:\n%s", out)
	}
}

type errTodo struct{}

func (errTodo) Tasks(app.TodoScope, time.Time) ([]domain.Task, error) {
	return nil, errors.New("disk on fire")
}
func (errTodo) CompleteTask(domain.Task, time.Time) error { return nil }
func (errTodo) PlanTask(domain.Task, time.Time) (string, error) {
	return "", nil
}
