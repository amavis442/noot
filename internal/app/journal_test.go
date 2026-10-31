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

func TestSaveJournal(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local)
	j, err := svc.Journal(ctx, day)
	if err != nil {
		t.Fatal(err)
	}

	saved, err := svc.SaveJournal(ctx, day, j.Content+"Vandaag keyword-ibis gezien.\n", j.Version)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version == j.Version || !strings.Contains(saved.Content, "keyword-ibis") {
		t.Errorf("saved = %+v", saved)
	}
	hits, err := svc.Search(ctx, domain.SearchQuery{Text: "ibis"})
	if err != nil || len(hits) != 1 || hits[0].Path != j.Path {
		t.Errorf("search after save = %+v, %v", hits, err)
	}

	// The file changes on disk after the editor loaded it: saving with the old
	// version must not overwrite that change.
	if err := os.WriteFile(svc.Vault.Abs(j.Path), []byte(saved.Content+"edited elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = svc.SaveJournal(ctx, day, "my stale edit", saved.Version)
	if !errors.Is(err, app.ErrConflict) {
		t.Fatalf("stale save: err = %v, want ErrConflict", err)
	}
	b, _ := os.ReadFile(svc.Vault.Abs(j.Path))
	if !strings.Contains(string(b), "edited elsewhere") || strings.Contains(string(b), "my stale edit") {
		t.Errorf("file after stale save:\n%s", b)
	}
}

func TestSaveJournalOfAMissingDayIsAConflict(t *testing.T) {
	svc, _ := setup(t)
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local)
	if _, err := svc.SaveJournal(context.Background(), day, "x", "whatever"); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}
