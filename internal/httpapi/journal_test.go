package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
)

type fakeJournal struct{ day time.Time }

func (f *fakeJournal) Journal(_ context.Context, day time.Time) (*app.JournalDay, error) {
	f.day = day
	date := day.Format("2006-01-02")
	return &app.JournalDay{
		Document: domain.Document{
			NoteRef: domain.NoteRef{ID: "J", Path: "journal/" + date + ".md", Title: date},
			Content: "---\ntitle: x\n---\n# Dag\n\nZie [[Alpha]].\n",
			Version: "v1",
		},
		Date: date,
	}, nil
}

type fakeAgenda struct {
	day time.Time
	// SaveAgendaDay
	saved struct {
		day              time.Time
		content, version string
	}
	save error
}

func (f *fakeAgenda) Agenda(_ context.Context, day time.Time) (*app.AgendaWeek, error) {
	f.day = day
	return &app.AgendaWeek{
		Document: domain.Document{
			NoteRef: domain.NoteRef{ID: "W", Path: "agenda/2026-W45.md", Title: "Agenda 2026-W45"},
			Content: "---\ntitle: Agenda\n---\n# Week 45, 2026\n\n## Monday 2 November\n\n### Planned\n\n- [ ] Melk ([[reminders]])\n",
			Version: "w1",
		},
		Week:   "2026-W45",
		Monday: "2026-11-02",
		Intro:  "---\ntitle: Agenda\n---\n# Week 45, 2026\n\n",
		Days: []app.AgendaDay{
			{Date: "2026-11-02", Heading: "Monday 2 November", Content: "### Planned\n\n- [ ] Melk ([[reminders]])", Version: "d1"},
		},
	}, nil
}

func (f *fakeAgenda) SaveAgendaDay(_ context.Context, day time.Time, content, version string) (*app.AgendaDay, error) {
	f.saved.day, f.saved.content, f.saved.version = day, content, version
	if f.save != nil {
		return nil, f.save
	}
	return &app.AgendaDay{Date: day.Format("2006-01-02"), Heading: "Monday 2 November", Content: content, Version: "d2"}, nil
}

func TestGetJournal(t *testing.T) {
	j := &fakeJournal{}
	p := testPorts()
	p.Journal = j
	h := New(p, fstest.MapFS{})

	rec := do(t, h, "GET", "/api/journal/2026-10-31", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got struct{ Date, Path, Version, Content, HTML string }
	decode(t, rec, &got)
	if got.Date != "2026-10-31" || got.Version != "v1" || !strings.Contains(got.HTML, `<a href="#/note/Alpha">Alpha</a>`) {
		t.Errorf("journal = %+v", got)
	}

	// Words work like on the command line.
	do(t, h, "GET", "/api/journal/today", "", "")
	if y, m, d := time.Now().Date(); j.day.Year() != y || j.day.Month() != m || j.day.Day() != d {
		t.Errorf("today resolved to %s", j.day)
	}
	if rec := do(t, h, "GET", "/api/journal/someday", "", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad day: status %d", rec.Code)
	}
	if rec := do(t, h, "PUT", "/api/journal/2026-10-31", "application/json", `{}`); rec.Code != http.StatusNotFound {
		t.Errorf("PUT journal: status %d; journal notes are saved through /api/notes", rec.Code)
	}
}

func TestGetAgenda(t *testing.T) {
	a := &fakeAgenda{}
	p := testPorts()
	p.Agenda = a
	h := New(p, fstest.MapFS{})

	rec := do(t, h, "GET", "/api/agenda/2026-11-05", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got struct{ Week, Monday, Path, Version, HTML string }
	decode(t, rec, &got)
	if got.Week != "2026-W45" || got.Monday != "2026-11-02" || got.Path != "agenda/2026-W45.md" || got.Version != "w1" ||
		!strings.Contains(got.HTML, `<a href="#/note/reminders">reminders</a>`) {
		t.Errorf("agenda = %+v", got)
	}
	if a.day.Format("2006-01-02") != "2026-11-05" {
		t.Errorf("day = %s", a.day)
	}
	if rec := do(t, h, "GET", "/api/agenda/ooit", "", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad day: status %d", rec.Code)
	}

	// The intro and every day come rendered on their own, for a view per day.
	var parts struct {
		IntroHTML string `json:"intro_html"`
		Days      []struct{ Date, Heading, Content, Version, HTML string }
	}
	decode(t, do(t, h, "GET", "/api/agenda/2026-11-05", "", ""), &parts)
	if !strings.Contains(parts.IntroHTML, "<h1") || strings.Contains(parts.IntroHTML, "title: Agenda") {
		t.Errorf("intro_html = %s", parts.IntroHTML)
	}
	if len(parts.Days) != 1 || parts.Days[0].Date != "2026-11-02" || parts.Days[0].Version != "d1" ||
		!strings.Contains(parts.Days[0].HTML, `<a href="#/note/reminders">reminders</a>`) || strings.Contains(parts.Days[0].HTML, "<h2") {
		t.Errorf("days = %+v", parts.Days)
	}
}

func TestSaveAgendaDay(t *testing.T) {
	a := &fakeAgenda{}
	p := testPorts()
	p.Agenda = a
	h := New(p, fstest.MapFS{})

	rec := do(t, h, "PUT", "/api/agenda/2026-11-02/day", "application/json", `{"content":"- [ ] Zie [[Thee]]","version":"d1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got struct{ Date, Content, Version, HTML string }
	decode(t, rec, &got)
	if a.saved.day.Format("2006-01-02") != "2026-11-02" || a.saved.content != "- [ ] Zie [[Thee]]" || a.saved.version != "d1" {
		t.Errorf("saved = %+v", a.saved)
	}
	if got.Version != "d2" || !strings.Contains(got.HTML, `<a href="#/note/Thee">Thee</a>`) {
		t.Errorf("answer = %+v", got)
	}

	a.save = fmt.Errorf("Monday 2 November of agenda/2026-W45.md was %w", app.ErrConflict)
	if rec := do(t, h, "PUT", "/api/agenda/2026-11-02/day", "application/json", `{"content":"x","version":"old"}`); rec.Code != http.StatusConflict {
		t.Errorf("conflict: status %d", rec.Code)
	}
	a.save = fmt.Errorf("%w: heading of another day", app.ErrInvalidNote)
	if rec := do(t, h, "PUT", "/api/agenda/2026-11-02/day", "application/json", `{"content":"x","version":"d1"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid: status %d", rec.Code)
	}
	if rec := do(t, h, "PUT", "/api/agenda/2026-11-02/day", "application/json", `{"content":"x"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("missing version: status %d", rec.Code)
	}
	if rec := do(t, h, "PUT", "/api/agenda/ooit/day", "application/json", `{"content":"x","version":"d1"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad day: status %d", rec.Code)
	}
}
