package reminders_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/tui/kit/kittest"
	"github.com/amavis442/noot/internal/tui/reminders"
	"github.com/amavis442/noot/internal/vault"
)

var today = time.Date(2026, 10, 31, 14, 0, 0, 0, time.Local) // a Saturday

// fakeReminders keeps reminders in memory, numbered in insertion order.
type fakeReminders struct {
	list    []vault.Reminder
	listAll []bool
	planned map[int]string
}

func (f *fakeReminders) Reminders(all bool) ([]vault.Reminder, error) {
	f.listAll = append(f.listAll, all)
	var out []vault.Reminder
	for _, r := range f.list {
		if all || !r.Done {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeReminders) AddReminder(text string, due, _ time.Time) (vault.Reminder, error) {
	if strings.TrimSpace(text) == "" {
		return vault.Reminder{}, errors.New("the text is empty")
	}
	r := vault.Reminder{N: len(f.list) + 1, Text: text}
	if !due.IsZero() {
		r.Due = due.Format("2006-01-02")
	}
	f.list = append(f.list, r)
	return r, nil
}

func (f *fakeReminders) CompleteReminder(n int, now time.Time) (vault.Reminder, error) {
	f.list[n-1].Done, f.list[n-1].DoneOn = true, now.Format("2006-01-02")
	return f.list[n-1], nil
}

func (f *fakeReminders) PlanReminder(n int, day time.Time) (vault.Reminder, string, error) {
	f.list[n-1].Planned = day.Format("2006-01-02")
	return f.list[n-1], "agenda/2026-W44.md", nil
}

func newModel() (*reminders.Model, *fakeReminders, *kittest.Editor) {
	f := &fakeReminders{list: []vault.Reminder{
		{N: 1, Text: "Tandarts bellen", Due: "2026-10-29"},
		{N: 2, Text: "Brood bakken", Planned: "2026-11-02"},
		{N: 3, Text: "Oud klusje", Done: true, DoneOn: "2026-10-20"},
	}}
	ed := &kittest.Editor{}
	m := reminders.New(f, ed, func() time.Time { return today })
	m.SetSize(80, 20)
	return m, f, ed
}

func TestListsOpenRemindersWithTheirDates(t *testing.T) {
	m, _, _ := newModel()
	kittest.Run(t, m, m.Init())
	out := m.View()
	for _, want := range []string{"Tandarts bellen", "29 oktober", "te laat", "Brood bakken", "gepland 2 november"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Oud klusje") {
		t.Fatalf("done reminders are hidden:\n%s", out)
	}
	kittest.Type(t, m, ".")
	if out := m.View(); !strings.Contains(out, "Oud klusje") {
		t.Fatalf(". shows done reminders too:\n%s", out)
	}
}

func TestCompletesTheSelectedReminder(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "j", "x")
	if !f.list[1].Done || f.list[0].Done {
		t.Fatalf("x completes the selected reminder: %+v", f.list)
	}
	out := m.View()
	if !strings.Contains(out, "Afgevinkt: Brood bakken") || strings.Contains(out, "  Brood bakken") {
		t.Fatalf("view after completing:\n%s", out)
	}
}

func TestAddsAReminderWithADueDate(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "a")
	if !m.Capturing() {
		t.Fatal("a opens a text field")
	}
	kittest.Type(t, m, "Kaas kopen", "enter", "monday", "enter")
	if m.Capturing() {
		t.Fatal("the field closes after adding")
	}
	got := f.list[len(f.list)-1]
	if got.Text != "Kaas kopen" || got.Due != "2026-11-02" {
		t.Fatalf("added %+v", got)
	}
	if out := m.View(); !strings.Contains(out, "Toegevoegd: Kaas kopen") {
		t.Fatalf("view after adding:\n%s", out)
	}
}

func TestAddWithoutDueDateAndCancel(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "a", "Zonder datum", "enter", "enter")
	if got := f.list[len(f.list)-1]; got.Text != "Zonder datum" || got.Due != "" {
		t.Fatalf("added %+v", got)
	}
	n := len(f.list)
	kittest.Type(t, m, "a", "Toch niet", "esc")
	if len(f.list) != n || m.Capturing() {
		t.Fatal("esc cancels adding")
	}
}

func TestABadDateKeepsTheFieldOpen(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	n := len(f.list)
	kittest.Type(t, m, "a", "Iets", "enter", "ooit", "enter")
	if len(f.list) != n || !m.Capturing() {
		t.Fatal("a bad date adds nothing and keeps the field open")
	}
	if out := m.View(); !strings.Contains(out, "ooit") {
		t.Fatalf("the error names the date:\n%s", out)
	}
}

func TestPlansTheSelectedReminder(t *testing.T) {
	m, f, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "p", "enter")
	if f.list[0].Planned != "2026-10-31" {
		t.Fatalf("an empty day plans for today: %+v", f.list[0])
	}
	kittest.Type(t, m, "p", "tomorrow", "enter")
	if f.list[0].Planned != "2026-11-01" {
		t.Fatalf("planned %+v", f.list[0])
	}
	if out := m.View(); !strings.Contains(out, "Gepland op zondag 1 november 2026") {
		t.Fatalf("view after planning:\n%s", out)
	}
}

func TestEditOpensTheRemindersFile(t *testing.T) {
	m, _, ed := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "e")
	if !slices.Equal(ed.Opened, []string{vault.RemindersFile}) {
		t.Fatalf("opened %v", ed.Opened)
	}
}

func TestEmptyList(t *testing.T) {
	m := reminders.New(&fakeReminders{}, &kittest.Editor{}, func() time.Time { return today })
	m.SetSize(80, 20)
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "x", "p", "enter")
	if out := m.View(); !strings.Contains(out, "Geen open herinneringen") {
		t.Fatalf("view:\n%s", out)
	}
}
