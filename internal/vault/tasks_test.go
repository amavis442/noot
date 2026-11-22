package vault

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/domain"
)

// put writes a file in the vault, creating its folder.
func put(t *testing.T, v *Vault, rel, content string) {
	t.Helper()
	p := v.Abs(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const tasksReminders = `---
title: Reminders
---

# Reminders

- [ ] Call the dentist · due 2026-11-20
- [x] Buy milk · done 2026-11-10

Notes in between.
- [ ] Bake bread · planned 2026-11-18
`

const tasksBacklog = `---
title: Agenda
---

# Agenda

## Planned

- [ ] **C# API** (learning project): build it
  in C#/.NET. When: later, no date yet.
- [ ] **Fix the roof** · due 2026-12-01
- A plain bullet is no task.

## Doing

- [ ] **Write docs** · planned 2026-11-19

## Done

- [x] **Rename repo** · done 2026-11-22
`

const tasksWeek = `---
title: Agenda 2026-W47
---

# Week 47, 2026

## Monday 16 November

### Planned
- Vakantie 16-27
- [ ] Sort the mail

### Doing

### Done
- [x] Already finished

## Wednesday 18 November

### Planned
- [ ] Bake bread ([[reminders]])
- [x] Water the plants

### Doing
- [ ] Paint the fence · due 2026-11-25
`

func tasksVault(t *testing.T) *Vault {
	t.Helper()
	v := &Vault{Root: t.TempDir()}
	put(t, v, RemindersFile, tasksReminders)
	put(t, v, "agenda/agenda.md", tasksBacklog)
	put(t, v, "agenda/2026-W47.md", tasksWeek)
	return v
}

func TestTasksReadsAllThreeSources(t *testing.T) {
	v := tasksVault(t)
	got, err := v.Tasks()
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.Task{
		{Source: domain.SourceReminders, File: "reminders.md", Line: 7, N: 1, Text: "Call the dentist", Due: "2026-11-20"},
		{Source: domain.SourceReminders, File: "reminders.md", Line: 8, N: 2, Text: "Buy milk", Done: true, DoneOn: "2026-11-10"},
		{Source: domain.SourceReminders, File: "reminders.md", Line: 11, N: 3, Text: "Bake bread", Planned: "2026-11-18"},

		{Source: domain.SourceBacklog, File: "agenda/agenda.md", Line: 9, Text: "**C# API** (learning project): build it", Section: "Planned"},
		{Source: domain.SourceBacklog, File: "agenda/agenda.md", Line: 11, Text: "**Fix the roof**", Due: "2026-12-01", Section: "Planned"},
		{Source: domain.SourceBacklog, File: "agenda/agenda.md", Line: 16, Text: "**Write docs**", Planned: "2026-11-19", Section: "Doing"},
		{Source: domain.SourceBacklog, File: "agenda/agenda.md", Line: 20, Text: "**Rename repo**", Done: true, DoneOn: "2026-11-22", Section: "Done"},

		{Source: domain.SourceWeek, File: "agenda/2026-W47.md", Line: 11, Text: "Sort the mail", Planned: "2026-11-16", Section: "Planned"},
		{Source: domain.SourceWeek, File: "agenda/2026-W47.md", Line: 22, Text: "Water the plants", Done: true, Planned: "2026-11-18", Section: "Planned"},
		{Source: domain.SourceWeek, File: "agenda/2026-W47.md", Line: 25, Text: "Paint the fence", Due: "2026-11-25", Planned: "2026-11-18", Section: "Doing"},
	}
	if !reflect.DeepEqual(got, want) {
		for i := range max(len(got), len(want)) {
			var g, w any
			if i < len(got) {
				g = got[i]
			}
			if i < len(want) {
				w = want[i]
			}
			if !reflect.DeepEqual(g, w) {
				t.Errorf("task %d:\n got  %+v\n want %+v", i, g, w)
			}
		}
	}
}

func TestTasksWithoutAnyFileIsEmpty(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	got, err := v.Tasks()
	if err != nil || len(got) != 0 {
		t.Fatalf("Tasks = %+v, %v", got, err)
	}
}

func TestTasksSkipsWeekFilesThatAreNotAWeek(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	put(t, v, "agenda/notes.md", "- [ ] stray\n")
	put(t, v, "agenda/2026-W99.md", "## Monday 1 January\n\n### Planned\n- [ ] impossible week\n")
	got, err := v.Tasks()
	if err != nil || len(got) != 0 {
		t.Fatalf("Tasks = %+v, %v", got, err)
	}
}

func TestTasksCRLF(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	put(t, v, RemindersFile, "- [ ] one · due 2026-11-20\r\n- [x] two\r\n")
	got, err := v.Tasks()
	if err != nil || len(got) != 2 || got[0].Text != "one" || got[0].Due != "2026-11-20" || !got[1].Done {
		t.Fatalf("Tasks = %+v, %v", got, err)
	}
}

// taskAt returns the task of tasks standing at line of file.
func taskAt(t *testing.T, v *Vault, file string, line int) domain.Task {
	t.Helper()
	all, err := v.Tasks()
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range all {
		if task.File == file && task.Line == line {
			return task
		}
	}
	t.Fatalf("no task at %s:%d in %+v", file, line, all)
	return domain.Task{}
}

func readFile(t *testing.T, v *Vault, rel string) string {
	t.Helper()
	b, err := os.ReadFile(v.Abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var tickDay = time.Date(2026, 11, 23, 9, 0, 0, 0, time.Local)

func TestCompleteTaskTicksAReminderAndKeepsItsLine(t *testing.T) {
	v := tasksVault(t)
	if err := v.CompleteTask(taskAt(t, v, "reminders.md", 7), tickDay); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(tasksReminders, "- [ ] Call the dentist · due 2026-11-20", "- [x] Call the dentist · due 2026-11-20 · done 2026-11-23", 1)
	if got := readFile(t, v, RemindersFile); got != want {
		t.Errorf("reminders.md =\n%s\nwant\n%s", got, want)
	}
}

func TestCompleteTaskOnAPlannedReminderTicksTheAgendaToo(t *testing.T) {
	v := tasksVault(t)
	if err := v.CompleteTask(taskAt(t, v, "reminders.md", 11), tickDay); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, v, RemindersFile); !strings.Contains(got, "- [x] Bake bread · planned 2026-11-18 · done 2026-11-23\n") {
		t.Errorf("reminders.md =\n%s", got)
	}
	if got := readFile(t, v, "agenda/2026-W47.md"); !strings.Contains(got, "- [x] Bake bread ([[reminders]])\n") {
		t.Errorf("the week was not ticked:\n%s", got)
	}
}

func TestCompleteTaskMovesABacklogItemToDone(t *testing.T) {
	v := tasksVault(t)
	if err := v.CompleteTask(taskAt(t, v, BacklogFile, 9), tickDay); err != nil {
		t.Fatal(err)
	}
	want := `---
title: Agenda
---

# Agenda

## Planned

- [ ] **Fix the roof** · due 2026-12-01
- A plain bullet is no task.

## Doing

- [ ] **Write docs** · planned 2026-11-19

## Done

- [x] **Rename repo** · done 2026-11-22
- [x] **C# API** (learning project): build it · done 2026-11-23
  in C#/.NET. When: later, no date yet.
`
	// The metadata sits on the first line, the continuation moves along with it.
	if got := readFile(t, v, BacklogFile); got != want {
		t.Errorf("agenda.md =\n%s\nwant\n%s", got, want)
	}
}

func TestCompleteTaskCreatesTheDoneSectionWhenMissing(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	put(t, v, BacklogFile, "# Agenda\n\n## Planned\n\n- [ ] one\n- [ ] two\n")
	if err := v.CompleteTask(taskAt(t, v, BacklogFile, 5), tickDay); err != nil {
		t.Fatal(err)
	}
	want := "# Agenda\n\n## Planned\n\n- [ ] two\n\n## Done\n\n- [x] one · done 2026-11-23\n"
	if got := readFile(t, v, BacklogFile); got != want {
		t.Errorf("agenda.md =\n%q\nwant\n%q", got, want)
	}
}

func TestCompleteTaskTicksAWeekLineInPlace(t *testing.T) {
	v := tasksVault(t)
	if err := v.CompleteTask(taskAt(t, v, "agenda/2026-W47.md", 11), tickDay); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(tasksWeek, "- [ ] Sort the mail", "- [x] Sort the mail", 1)
	if got := readFile(t, v, "agenda/2026-W47.md"); got != want {
		t.Errorf("week =\n%s\nwant\n%s", got, want)
	}
}

func TestCompleteTaskRefusesALineThatMoved(t *testing.T) {
	v := tasksVault(t)
	task := taskAt(t, v, "reminders.md", 7)
	// Someone adds a line on top: line 7 is now another reminder.
	put(t, v, RemindersFile, strings.Replace(tasksReminders, "# Reminders\n", "# Reminders\n\n- [ ] Brand new\n", 1))
	before := readFile(t, v, RemindersFile)
	err := v.CompleteTask(task, tickDay)
	if !errors.Is(err, ErrTaskChanged) {
		t.Fatalf("err = %v, want ErrTaskChanged", err)
	}
	if got := readFile(t, v, RemindersFile); got != before {
		t.Errorf("a refused tick must not write:\n%s", got)
	}
}

func TestCompleteTaskRefusesDoneTasksAndMissingFiles(t *testing.T) {
	v := tasksVault(t)
	if err := v.CompleteTask(taskAt(t, v, "reminders.md", 8), tickDay); err == nil {
		t.Error("ticking a done task should fail")
	}
	gone := domain.Task{Source: domain.SourceBacklog, File: "agenda/agenda.md", Line: 99, Text: "x"}
	if err := v.CompleteTask(gone, tickDay); !errors.Is(err, ErrTaskChanged) {
		t.Errorf("line past the end: %v", err)
	}
	if err := v.CompleteTask(domain.Task{File: "../outside.md", Line: 1, Text: "x"}, tickDay); err == nil {
		t.Error("a path outside the agenda files should be refused")
	}
}
