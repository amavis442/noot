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

type fakeJournal struct {
	day              time.Time
	content, version string
	save             error
}

func (f *fakeJournal) Journal(_ context.Context, day time.Time) (*app.JournalDay, error) {
	f.day = day
	date := day.Format("2006-01-02")
	return &app.JournalDay{
		Document: domain.Document{NoteRef: domain.NoteRef{ID: "J", Path: "journal/" + date + ".md", Title: date}, Content: "---\ntitle: x\n---\n# Dag\n\nZie [[Alpha]].\n"},
		Date:     date,
		Version:  "v1",
	}, nil
}

func (f *fakeJournal) SaveJournal(ctx context.Context, day time.Time, content, version string) (*app.JournalDay, error) {
	f.content, f.version = content, version
	if f.save != nil {
		return nil, f.save
	}
	return f.Journal(ctx, day)
}

func journalAPI(j *fakeJournal) http.Handler {
	return New(Ports{Notes: &fakeNotes{}, Journal: j, Reminders: &fakeReminders{}}, fstest.MapFS{})
}

func TestGetJournal(t *testing.T) {
	j := &fakeJournal{}
	h := journalAPI(j)

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
}

func TestPutJournal(t *testing.T) {
	j := &fakeJournal{}
	h := journalAPI(j)

	rec := do(t, h, "PUT", "/api/journal/2026-10-31", "application/json", `{"content":"# Nieuw\n","version":"v1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if j.content != "# Nieuw\n" || j.version != "v1" {
		t.Errorf("saved %q with version %q", j.content, j.version)
	}

	j.save = fmt.Errorf("journal/2026-10-31.md was %w", app.ErrConflict)
	rec = do(t, h, "PUT", "/api/journal/2026-10-31", "application/json", `{"content":"x","version":"old"}`)
	var e struct{ Error string }
	decode(t, rec, &e)
	if rec.Code != http.StatusConflict || !strings.Contains(e.Error, "changed in the meantime") {
		t.Errorf("conflict: status %d, %+v", rec.Code, e)
	}

	if rec := do(t, h, "PUT", "/api/journal/2026-10-31", "text/plain", "x"); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text body: status %d", rec.Code)
	}
	if rec := do(t, h, "PUT", "/api/journal/2026-10-31", "application/json", `{"content":"x"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("missing version: status %d", rec.Code)
	}
}
