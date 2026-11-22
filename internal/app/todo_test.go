package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/vault"
)

// todoVault fills a vault with one task per group; today is Wednesday 18 November 2026.
func todoVault(t *testing.T) (*app.Service, string, time.Time) {
	t.Helper()
	svc, root := setup(t)
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.Local)
	write(t, root, "reminders.md", `# Reminders

- [ ] Late one · due 2026-11-10
- [ ] Today one · due 2026-11-18
- [ ] This week · planned 2026-11-20
- [ ] Far away · due 2027-01-05
- [x] Finished · done 2026-11-01
- [ ] Bake bread · planned 2026-11-19
`)
	write(t, root, "agenda/agenda.md", "# Agenda\n\n## Planned\n\n- [ ] **Backlog item**\n\n## Done\n\n")
	write(t, root, "agenda/2026-W47.md", "# Week 47, 2026\n\n## Thursday 19 November\n\n### Planned\n- [ ] Bake bread ([[reminders]])\n- [ ] Call mum\n")
	return svc, root, now
}

func texts(ts []domain.Task) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Text)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTasksAreOrderedByGroupThenDate(t *testing.T) {
	svc, _, now := todoVault(t)
	got, err := svc.Tasks(app.TodoOpen, now)
	if err != nil {
		t.Fatal(err)
	}
	// overdue, today, this week (by date), later, backlog; the planned
	// reminder is listed once, not again from the week file.
	want := []string{"Late one", "Today one", "Bake bread", "Call mum", "This week", "Far away", "**Backlog item**"}
	if !equal(texts(got), want) {
		t.Errorf("open = %v\nwant   %v", texts(got), want)
	}
}

func TestTasksScopes(t *testing.T) {
	svc, _, now := todoVault(t)
	cases := []struct {
		scope app.TodoScope
		want  []string
	}{
		{app.TodoToday, []string{"Late one", "Today one"}},
		{app.TodoWeek, []string{"Late one", "Today one", "Bake bread", "Call mum", "This week"}},
		{app.TodoAll, []string{"Late one", "Today one", "Bake bread", "Call mum", "This week", "Far away", "**Backlog item**", "Finished"}},
	}
	for _, c := range cases {
		got, err := svc.Tasks(c.scope, now)
		if err != nil {
			t.Fatal(err)
		}
		if !equal(texts(got), c.want) {
			t.Errorf("scope %d = %v\nwant     %v", c.scope, texts(got), c.want)
		}
	}
}

func TestTasksNeverNil(t *testing.T) {
	svc, _ := setup(t)
	got, err := svc.Tasks(app.TodoOpen, time.Now())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("Tasks = %#v, %v", got, err)
	}
}

func TestCompleteTaskFlow(t *testing.T) {
	svc, root, now := todoVault(t)
	open, _ := svc.Tasks(app.TodoOpen, now)
	for _, task := range open {
		if task.Text == "Late one" || task.Text == "**Backlog item**" {
			if err := svc.CompleteTask(task, now); err != nil {
				t.Fatalf("complete %q: %v", task.Text, err)
			}
		}
	}
	b, _ := os.ReadFile(filepath.Join(root, "agenda/agenda.md"))
	if string(b) != "# Agenda\n\n## Planned\n\n\n## Done\n\n- [x] **Backlog item** · done 2026-11-18\n\n" {
		t.Errorf("agenda.md = %q", b)
	}
	left, _ := svc.Tasks(app.TodoOpen, now)
	for _, task := range left {
		if task.Text == "Late one" || task.Text == "**Backlog item**" {
			t.Errorf("%q is still open", task.Text)
		}
	}

	done := domain.Task{Source: domain.SourceReminders, File: "reminders.md", Line: 7, N: 5, Text: "Finished", Done: true}
	if err := svc.CompleteTask(done, now); !errors.Is(err, app.ErrInvalidTask) {
		t.Errorf("ticking a done task = %v, want ErrInvalidTask", err)
	}
	stale := open[0]
	stale.Text = "Something else"
	if err := svc.CompleteTask(stale, now); !errors.Is(err, vault.ErrTaskChanged) {
		t.Errorf("stale task = %v, want ErrTaskChanged", err)
	}
}

func TestPlanTaskOnlyPlansReminders(t *testing.T) {
	svc, root, now := todoVault(t)
	open, _ := svc.Tasks(app.TodoOpen, now)
	var rem, backlog domain.Task
	for _, task := range open {
		switch task.Text {
		case "Late one":
			rem = task
		case "**Backlog item**":
			backlog = task
		}
	}
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, time.Local)
	if _, err := svc.PlanTask(backlog, monday); !errors.Is(err, app.ErrInvalidTask) {
		t.Errorf("planning a backlog item = %v, want ErrInvalidTask", err)
	}
	rel, err := svc.PlanTask(rem, monday)
	if err != nil || rel != "agenda/2026-W48.md" {
		t.Fatalf("PlanTask = %q, %v", rel, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "reminders.md"))
	if want := "- [ ] Late one · due 2026-11-10 · planned 2026-11-23\n"; !strings.Contains(string(b), want) {
		t.Errorf("reminders.md lacks %q:\n%s", want, b)
	}
	rem.Text = "Stale"
	if _, err := svc.PlanTask(rem, monday); !errors.Is(err, vault.ErrTaskChanged) {
		t.Errorf("stale = %v", err)
	}
}
