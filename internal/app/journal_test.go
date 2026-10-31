package app_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
)

func TestJournalCreatesTheDayAndIndexesIt(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local)

	j, err := svc.Journal(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	if j.Date != "2026-10-31" || j.Path != "journal/2026-10-31.md" || j.ID == "" || j.Version == "" {
		t.Fatalf("journal = %+v", j)
	}
	if !strings.Contains(j.Content, "# Saturday 31 October 2026") {
		t.Errorf("content = %q", j.Content)
	}

	again, err := svc.Journal(ctx, day)
	if err != nil || again.Version != j.Version || again.ID != j.ID {
		t.Errorf("second call = %+v, %v; want the same note", again, err)
	}
}

func TestSaveNote(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local)
	j, err := svc.Journal(ctx, day)
	if err != nil {
		t.Fatal(err)
	}

	saved, err := svc.SaveNote(ctx, j.Path, j.Content+"Vandaag keyword-ibis gezien.\n", j.Version)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version == j.Version || saved.ID != j.ID || !strings.Contains(saved.Content, "keyword-ibis") {
		t.Errorf("saved = %+v", saved)
	}
	hits, err := svc.Search(ctx, domain.SearchQuery{Text: "ibis"})
	if err != nil || len(hits) != 1 || hits[0].Path != j.Path {
		t.Errorf("search after save = %+v, %v", hits, err)
	}
	if doc, _ := svc.ReadNote(ctx, j.Path); doc.Version != saved.Version {
		t.Errorf("ReadNote version %s, saved %s", doc.Version, saved.Version)
	}

	// The file changes on disk after the editor loaded it: saving with the old
	// version must not overwrite that change.
	if err := os.WriteFile(svc.Vault.Abs(j.Path), []byte(saved.Content+"edited elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = svc.SaveNote(ctx, j.Path, "my stale edit", saved.Version)
	if !errors.Is(err, app.ErrConflict) {
		t.Fatalf("stale save: err = %v, want ErrConflict", err)
	}
	b, _ := os.ReadFile(svc.Vault.Abs(j.Path))
	if !strings.Contains(string(b), "edited elsewhere") || strings.Contains(string(b), "my stale edit") {
		t.Errorf("file after stale save:\n%s", b)
	}
}

func TestSaveNoteErrors(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	if _, err := svc.SaveNote(ctx, "nope", "x", "v"); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("unknown note: err = %v, want ErrNotFound", err)
	}

	// Removed after it was indexed: the index still knows it, the disk does not.
	ref, err := svc.CreateNote(ctx, app.NoteDraft{Title: "Weg"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.ReadNote(ctx, ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(svc.Vault.Abs(ref.Path)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveNote(ctx, ref.Path, "x", doc.Version); !errors.Is(err, app.ErrConflict) {
		t.Errorf("removed file: err = %v, want ErrConflict", err)
	}
	if _, err := os.Stat(svc.Vault.Abs(ref.Path)); !os.IsNotExist(err) {
		t.Error("saving recreated a removed file")
	}
}

func TestAgendaCreatesTheWeek(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	thursday := time.Date(2026, 11, 5, 0, 0, 0, 0, time.Local)

	a, err := svc.Agenda(ctx, thursday)
	if err != nil {
		t.Fatal(err)
	}
	if a.Path != "agenda/2026-W45.md" || a.Week != "2026-W45" || a.Monday != "2026-11-02" || a.ID == "" || a.Version == "" {
		t.Fatalf("agenda = %+v", a)
	}
	if !strings.Contains(a.Content, "## Thursday 5 November") {
		t.Errorf("content = %q", a.Content)
	}
	// The first days of January can belong to the last week of the year before.
	a, err = svc.Agenda(ctx, time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local))
	if err != nil || a.Week != "2026-W53" || a.Monday != "2026-12-28" {
		t.Errorf("1 January 2027 = %+v, %v", a, err)
	}
}
