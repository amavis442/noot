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

func TestAgendaHasItsDays(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)

	a, err := svc.Agenda(ctx, time.Date(2026, 11, 5, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(a.Intro, "# Week 45, 2026\n\n") || len(a.Days) != 7 {
		t.Fatalf("intro %q, %d days", a.Intro, len(a.Days))
	}
	thu := a.Days[3]
	if thu.Date != "2026-11-05" || thu.Heading != "Thursday 5 November" || thu.Content != "### Planned\n\n### Doing\n\n### Done" || thu.Version == "" {
		t.Errorf("thursday = %+v", thu)
	}
	if a.Days[0].Version != thu.Version {
		t.Error("days with the same text should have the same version")
	}
}

func TestSaveAgendaDay(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	monday := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	tuesday := monday.AddDate(0, 0, 1)
	a, err := svc.Agenda(ctx, monday)
	if err != nil {
		t.Fatal(err)
	}
	tue := a.Days[1]

	// Planning a reminder on Wednesday changes the file, not Tuesday.
	if _, err := svc.AddReminder("Fiets", time.Time{}, monday); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PlanReminder(1, monday.AddDate(0, 0, 2)); err != nil {
		t.Fatal(err)
	}

	day, err := svc.SaveAgendaDay(ctx, tuesday, "### Planned\n\n- [ ] keyword-redshank", tue.Version)
	if err != nil {
		t.Fatal(err)
	}
	if day.Date != "2026-11-03" || day.Content != "### Planned\n\n- [ ] keyword-redshank" || day.Version == tue.Version {
		t.Errorf("saved day = %+v", day)
	}
	b, _ := os.ReadFile(svc.Vault.Abs(a.Path))
	if !strings.Contains(string(b), "keyword-redshank") || !strings.Contains(string(b), "- [ ] Fiets ([[reminders]])") {
		t.Errorf("file keeps both edits?\n%s", b)
	}
	if hits, _ := svc.Search(ctx, domain.SearchQuery{Text: "redshank"}); len(hits) != 1 {
		t.Errorf("search after save = %+v", hits)
	}

	// Tuesday itself changed since tue was read.
	if _, err := svc.SaveAgendaDay(ctx, tuesday, "stale", tue.Version); !errors.Is(err, app.ErrConflict) {
		t.Errorf("stale day: err = %v, want ErrConflict", err)
	}
}

func TestSaveAgendaDayErrors(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	monday := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	a, err := svc.Agenda(ctx, monday)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.SaveAgendaDay(ctx, monday, "x\n\n## Tuesday 3 November", a.Days[0].Version)
	if !errors.Is(err, app.ErrInvalidNote) {
		t.Errorf("heading of another day: err = %v, want ErrInvalidNote", err)
	}

	// The day's heading was removed from the file.
	b, _ := os.ReadFile(svc.Vault.Abs(a.Path))
	if err := os.WriteFile(svc.Vault.Abs(a.Path), []byte(strings.Replace(string(b), "## Monday 2 November\n", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveAgendaDay(ctx, monday, "x", a.Days[0].Version); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("missing day: err = %v, want ErrNotFound", err)
	}
}
