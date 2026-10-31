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

// fakeAgenda builds a week with one line of text per day.
type fakeAgenda struct {
	asked []string // ISO weeks
	extra string   // appended to Saturday
	fail  error
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
		w.Days = append(w.Days, app.AgendaDay{Date: d.Format("2006-01-02"), Heading: d.Format("Monday 2 January"), Content: "### Planned\n\n- [ ] " + text})
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
	kittest.Type(t, m, "right", "l", "h", "t")
	if want := []string{"2026-W44", "2026-W45", "2026-W46", "2026-W45", "2026-W44"}; !slices.Equal(a.asked, want) {
		t.Fatalf("asked %v, want %v", a.asked, want)
	}
	kittest.Type(t, m, "left")
	if out := m.View(); strings.Contains(out, "vandaag") || !strings.Contains(out, "Week 43") {
		t.Fatalf("last week has no today:\n%s", out)
	}
}

func TestEditOpensTheWeekAndReloads(t *testing.T) {
	m, a, ed := newModel(80)
	kittest.Run(t, m, m.Init())
	a.extra = " en boodschappen"
	kittest.Type(t, m, "e")
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
