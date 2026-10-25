package vault

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestAgenda(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	wed := time.Date(2026, 10, 28, 15, 0, 0, 0, time.Local)

	rel, created, err := v.Agenda(wed)
	if err != nil || rel != "agenda/2026-W44.md" || !created {
		t.Fatalf("Agenda = %q, %v, %v", rel, created, err)
	}
	b, _ := os.ReadFile(v.Abs(rel))
	s := string(b)
	want := "---\ntitle: Agenda 2026-W44\ncreated: 2026-10-26\ntags: [agenda]\n---\n\n" +
		"# Week 44, 2026\n\n"
	for _, d := range []string{"Monday 26 October", "Tuesday 27 October", "Wednesday 28 October", "Thursday 29 October",
		"Friday 30 October", "Saturday 31 October", "Sunday 1 November"} {
		want += "## " + d + "\n\n### Planned\n\n### Doing\n\n### Done\n\n"
	}
	if s != want {
		t.Errorf("agenda =\n%q\nwant\n%q", s, want)
	}

	// The same week again: opened, not overwritten.
	if err := os.WriteFile(v.Abs(rel), []byte(s+"- dentist\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sunday := time.Date(2026, 11, 1, 23, 0, 0, 0, time.Local)
	if rel2, created, err := v.Agenda(sunday); err != nil || rel2 != rel || created {
		t.Errorf("same week = %q, %v, %v", rel2, created, err)
	}
	if b, _ := os.ReadFile(v.Abs(rel)); !strings.HasSuffix(string(b), "- dentist\n") {
		t.Error("existing agenda was overwritten")
	}
}

func TestAgendaUsesISOWeekYear(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	// Friday 1 January 2027 belongs to week 53 of 2026, which starts on Monday 28 December.
	rel, _, err := v.Agenda(time.Date(2027, 1, 1, 9, 0, 0, 0, time.Local))
	if err != nil || rel != "agenda/2026-W53.md" {
		t.Fatalf("Agenda = %q, %v", rel, err)
	}
	b, _ := os.ReadFile(v.Abs(rel))
	if s := string(b); !strings.Contains(s, "## Monday 28 December\n") || !strings.Contains(s, "## Sunday 3 January\n") {
		t.Errorf("agenda:\n%s", s)
	}
}
