package app_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
)

func TestRemindersFlow(t *testing.T) {
	svc, _ := setup(t)
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, time.Local) // a Wednesday
	friday := time.Date(2026, 10, 30, 0, 0, 0, 0, time.Local)
	monday := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)

	for _, add := range []struct {
		text string
		due  time.Time
	}{{"Geen datum", time.Time{}}, {"Tandarts bellen", friday}, {"Melk", time.Time{}}} {
		if _, err := svc.AddReminder(add.text, add.due, now); err != nil {
			t.Fatal(err)
		}
	}

	r, agenda, err := svc.PlanReminder(3, monday)
	if err != nil {
		t.Fatal(err)
	}
	if r.Planned != "2026-11-02" || agenda != "agenda/2026-W45.md" {
		t.Errorf("plan = %+v, %s", r, agenda)
	}
	if _, err := svc.CompleteReminder(1, now); err != nil {
		t.Fatal(err)
	}

	open, err := svc.Reminders(false)
	if err != nil {
		t.Fatal(err)
	}
	// Earliest date (due or planned) first, undated last; done ones left out.
	if len(open) != 2 || open[0].Text != "Tandarts bellen" || open[1].Text != "Melk" {
		t.Errorf("open = %+v", open)
	}
	all, err := svc.Reminders(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[2].Text != "Geen datum" || !all[2].Done {
		t.Errorf("all = %+v", all)
	}
}

func TestRemindersWithoutFileIsEmptyNotNil(t *testing.T) {
	svc, _ := setup(t)
	rs, err := svc.Reminders(false)
	if err != nil || rs == nil || len(rs) != 0 {
		t.Fatalf("reminders = %#v, %v", rs, err)
	}
}

func TestReminderErrors(t *testing.T) {
	svc, _ := setup(t)
	now := time.Now()

	if _, err := svc.AddReminder("   ", time.Time{}, now); !errors.Is(err, app.ErrInvalidReminder) {
		t.Errorf("empty text: %v", err)
	}
	if _, err := svc.CompleteReminder(1, now); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("done without reminders: %v", err)
	}
	if _, err := svc.AddReminder("Iets", time.Time{}, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PlanReminder(2, now); !errors.Is(err, app.ErrNotFound) || !strings.Contains(err.Error(), "no reminder 2") {
		t.Errorf("plan unknown: %v", err)
	}
	if _, err := svc.CompleteReminder(1, now); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteReminder(1, now); !errors.Is(err, app.ErrInvalidReminder) {
		t.Errorf("done twice: %v", err)
	}
	if _, _, err := svc.PlanReminder(1, now); !errors.Is(err, app.ErrInvalidReminder) {
		t.Errorf("plan done: %v", err)
	}
	if _, err := os.Stat(svc.Vault.Abs("agenda")); err == nil {
		entries, _ := os.ReadDir(svc.Vault.Abs("agenda"))
		if len(entries) != 0 {
			t.Errorf("a failed plan created an agenda: %v", entries)
		}
	}
}
