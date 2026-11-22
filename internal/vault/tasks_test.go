package vault

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

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
