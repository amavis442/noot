package journal_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/tui/journal"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/tui/kit/kittest"
)

var today = time.Date(2026, 10, 31, 14, 0, 0, 0, time.Local)

// fakeJournal returns a note per day, with text set per date.
type fakeJournal struct {
	text  map[string]string
	asked []string
	fail  error
}

func (f *fakeJournal) Journal(_ context.Context, day time.Time) (*app.JournalDay, error) {
	date := day.Format("2006-01-02")
	f.asked = append(f.asked, date)
	if f.fail != nil {
		return nil, f.fail
	}
	rel := "journal/" + date + ".md"
	return &app.JournalDay{Document: domain.Document{NoteRef: domain.NoteRef{Path: rel}, Content: f.text[date]}, Date: date}, nil
}

func newModel() (*journal.Model, *fakeJournal, *kittest.Editor) {
	j := &fakeJournal{text: map[string]string{
		"2026-10-31": "# 2026-10-31\n\nBrood gebakken.",
		"2026-10-30": "# 2026-10-30\n\nKoffie gezet.",
	}}
	ed := &kittest.Editor{}
	m := journal.New(context.Background(), j, kit.Plain{}, ed, func() time.Time { return today })
	m.SetSize(80, 20)
	return m, j, ed
}

func TestShowsTodaysJournal(t *testing.T) {
	m, _, _ := newModel()
	kittest.Run(t, m, m.Init())
	out := m.View()
	for _, want := range []string{"zaterdag 31 oktober 2026", "vandaag", "journal/2026-10-31.md", "Brood gebakken."} {
		if !strings.Contains(out, want) {
			t.Fatalf("view lacks %q:\n%s", want, out)
		}
	}
}

func TestBrowsesDaysAndBackToToday(t *testing.T) {
	m, j, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "left")
	if out := m.View(); !strings.Contains(out, "vrijdag 30 oktober 2026") || !strings.Contains(out, "Koffie gezet.") || strings.Contains(out, "vandaag") {
		t.Fatalf("left shows yesterday:\n%s", out)
	}
	kittest.Type(t, m, "l", "l")
	if out := m.View(); !strings.Contains(out, "zondag 1 november 2026") {
		t.Fatalf("l goes a day forward:\n%s", out)
	}
	kittest.Type(t, m, "t")
	if want := []string{"2026-10-31", "2026-10-30", "2026-10-31", "2026-11-01", "2026-10-31"}; !slices.Equal(j.asked, want) {
		t.Fatalf("asked %v, want %v", j.asked, want)
	}
}

func TestInitReturnsToTheSameDay(t *testing.T) {
	m, j, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "h")
	kittest.Run(t, m, m.Init())
	if got := j.asked[len(j.asked)-1]; got != "2026-10-30" {
		t.Fatalf("reopening the tab reloads the day shown, asked %s", got)
	}
}

func TestEditOpensTheDayAndReloads(t *testing.T) {
	m, j, ed := newModel()
	kittest.Run(t, m, m.Init())
	j.text["2026-10-31"] = "# 2026-10-31\n\nBrood en soep."
	kittest.Type(t, m, "e")
	if !slices.Equal(ed.Opened, []string{"journal/2026-10-31.md"}) {
		t.Fatalf("opened %v", ed.Opened)
	}
	if out := m.View(); !strings.Contains(out, "Brood en soep.") {
		t.Fatalf("reloaded after editing:\n%s", out)
	}
}

func TestShowsErrors(t *testing.T) {
	m, j, _ := newModel()
	j.fail = errors.New("geen toegang")
	kittest.Run(t, m, m.Init())
	if out := m.View(); !strings.Contains(out, "geen toegang") {
		t.Fatalf("error missing:\n%s", out)
	}
	if m.Capturing() {
		t.Fatal("the journal tab never captures the keyboard")
	}
}
