package agenda_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/tui/agenda"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/tui/kit/kittest"
	"github.com/amavis442/noot/internal/vault"
)

var today = time.Date(2026, 10, 31, 14, 0, 0, 0, time.Local) // a Saturday in 2026-W44

// fakeAgenda builds a week with one line of text per day and saves days.
type fakeAgenda struct {
	asked    []string // ISO weeks
	extra    string   // appended to Saturday
	fail     error
	saved    map[string]string // date -> content
	saves    []string          // "date@version" per save
	conflict bool
}

func (f *fakeAgenda) SaveAgendaDay(_ context.Context, day time.Time, content, ver string) (*app.AgendaDay, error) {
	date := day.Format("2006-01-02")
	f.saves = append(f.saves, date+"@"+ver)
	if f.conflict {
		return nil, fmt.Errorf("%s was %w", date, app.ErrConflict)
	}
	if f.saved == nil {
		f.saved = map[string]string{}
	}
	f.saved[date] = content
	return &app.AgendaDay{Date: date, Content: content, Version: "v-" + content}, nil
}

func (f *fakeAgenda) Agenda(_ context.Context, day time.Time) (*app.AgendaWeek, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	year, week := day.ISOWeek()
	name := fmt.Sprintf("%d-W%02d", year, week)
	f.asked = append(f.asked, name)
	monday := vault.MondayOf(day)
	w := &app.AgendaWeek{
		Document: domain.Document{NoteRef: domain.NoteRef{Path: "agenda/" + name + ".md"}},
		Week:     name,
		Monday:   monday.Format("2006-01-02"),
	}
	for i := range 7 {
		d := monday.AddDate(0, 0, i)
		text := "Taak op " + d.Weekday().String()
		if d.Weekday() == time.Saturday {
			text += f.extra
		}
		date := d.Format("2006-01-02")
		content := "### Planned\n\n- [ ] " + text
		if s, ok := f.saved[date]; ok {
			content = s
		}
		w.Days = append(w.Days, app.AgendaDay{Date: date, Heading: d.Format("Monday 2 January"), Content: content, Version: "v-" + content})
	}
	return w, nil
}

func newModel(height int) (*agenda.Model, *fakeAgenda, *kittest.Editor) {
	a := &fakeAgenda{}
	ed := &kittest.Editor{}
	m := agenda.New(context.Background(), a, kit.Plain{}, ed, func() time.Time { return today })
	m.SetSize(80, height)
	return m, a, ed
}

func TestShowsTheWeekWithEveryDay(t *testing.T) {
	m, _, _ := newModel(80)
	kittest.Run(t, m, m.Init())
	out := m.View()
	for _, want := range []string{"Week 44", "26 oktober – 1 november 2026", "agenda/2026-W44.md", "Monday 26 October", "Taak op Monday", "Sunday 1 November", "Saturday 31 October · vandaag"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view lacks %q:\n%s", want, out)
		}
	}
}

func TestScrollsToToday(t *testing.T) {
	m, _, _ := newModel(8)
	kittest.Run(t, m, m.Init())
	out := m.View()
	if !strings.Contains(out, "Saturday 31 October") || strings.Contains(out, "Monday 26 October") {
		t.Fatalf("today is not at the top:\n%s", out)
	}
}

func TestBrowsesWeeks(t *testing.T) {
	m, a, _ := newModel(80)
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "]", "]", "[", "t")
	if want := []string{"2026-W44", "2026-W45", "2026-W46", "2026-W45", "2026-W44"}; !slices.Equal(a.asked, want) {
		t.Fatalf("asked %v, want %v", a.asked, want)
	}
	kittest.Type(t, m, "[")
	if out := m.View(); strings.Contains(out, "vandaag") || !strings.Contains(out, "Week 43") {
		t.Fatalf("last week has no today:\n%s", out)
	}
}

func TestEditOpensTheWeekAndReloads(t *testing.T) {
	m, a, ed := newModel(80)
	kittest.Run(t, m, m.Init())
	a.extra = " en boodschappen"
	kittest.Type(t, m, "E")
	if !slices.Equal(ed.Opened, []string{"agenda/2026-W44.md"}) {
		t.Fatalf("opened %v", ed.Opened)
	}
	if out := m.View(); !strings.Contains(out, "en boodschappen") {
		t.Fatalf("reloaded after editing:\n%s", out)
	}
}

func TestShowsErrors(t *testing.T) {
	m, a, _ := newModel(20)
	a.fail = errors.New("kapot")
	kittest.Run(t, m, m.Init())
	if out := m.View(); !strings.Contains(out, "kapot") {
		t.Fatalf("error missing:\n%s", out)
	}
}

func TestTodayIsSelectedAndArrowsSelectDays(t *testing.T) {
	m, _, _ := newModel(80)
	kittest.Run(t, m, m.Init())
	if out := m.View(); !strings.Contains(out, "▸ Saturday 31 October · vandaag") {
		t.Fatalf("today is selected:\n%s", out)
	}
	kittest.Type(t, m, "j")
	if out := m.View(); !strings.Contains(out, "▸ Sunday 1 November") || strings.Contains(out, "▸ Saturday") {
		t.Fatalf("j selects the next day:\n%s", out)
	}
	kittest.Type(t, m, "j", "up", "k")
	if out := m.View(); !strings.Contains(out, "▸ Friday 30 October") {
		t.Fatalf("j stops at Sunday, up and k go back:\n%s", out)
	}
	kittest.Type(t, m, "[")
	if out := m.View(); !strings.Contains(out, "▸ Monday 19 October") {
		t.Fatalf("another week starts at its Monday:\n%s", out)
	}
}

func TestEEditsOnlyTheSelectedDay(t *testing.T) {
	m, a, ed := newModel(80)
	ed.Change = func(_, content string) string { return content + "\n- [ ] Brood halen" }
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "k", "e")
	if !slices.Equal(ed.Texts, []string{"2026-10-30"}) || len(ed.Opened) != 0 {
		t.Fatalf("texts %v, files %v", ed.Texts, ed.Opened)
	}
	want := "### Planned\n\n- [ ] Taak op Friday"
	if !slices.Equal(a.saves, []string{"2026-10-30@v-" + want}) || a.saved["2026-10-30"] != want+"\n- [ ] Brood halen" {
		t.Fatalf("saves %v, saved %q", a.saves, a.saved["2026-10-30"])
	}
	out := m.View()
	for _, w := range []string{"Brood halen", "Opgeslagen: vrijdag 30 oktober", "▸ Friday 30 October"} {
		if !strings.Contains(out, w) {
			t.Fatalf("view lacks %q after saving:\n%s", w, out)
		}
	}
}

func TestAnUnchangedDayIsNotSaved(t *testing.T) {
	m, a, ed := newModel(80)
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "e")
	if len(ed.Texts) != 1 || len(a.saves) != 0 {
		t.Fatalf("texts %v, saves %v", ed.Texts, a.saves)
	}
}

func TestAConflictKeepsTheEditedText(t *testing.T) {
	m, a, ed := newModel(80)
	var given []string
	ed.Change = func(_, content string) string {
		given = append(given, content)
		return "mijn versie"
	}
	kittest.Run(t, m, m.Init())
	a.conflict = true
	kittest.Type(t, m, "e")
	if out := m.View(); !strings.Contains(out, "intussen gewijzigd") {
		t.Fatalf("the conflict is shown:\n%s", out)
	}
	a.conflict = false
	kittest.Type(t, m, "e")
	if len(given) != 2 || given[1] != "mijn versie" {
		t.Fatalf("e opens the kept text again, editor got %q", given)
	}
	if a.saved["2026-10-31"] != "mijn versie" {
		t.Fatalf("saved %q", a.saved["2026-10-31"])
	}
}
