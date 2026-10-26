package vault

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestAddAndListReminders(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local)

	if rs, err := v.Reminders(); err != nil || rs != nil {
		t.Fatalf("no reminders file: %v, %v", rs, err)
	}
	r, err := v.AddReminder("Call the dentist", time.Date(2026, 10, 30, 0, 0, 0, 0, time.Local), now)
	if err != nil || r.N != 1 || r.Due != "2026-10-30" {
		t.Fatalf("AddReminder = %+v, %v", r, err)
	}
	if r, err := v.AddReminder("  Buy milk \n", time.Time{}, now); err != nil || r.N != 2 {
		t.Fatalf("second AddReminder = %+v, %v", r, err)
	}
	if _, err := v.AddReminder("  ", time.Time{}, now); err == nil {
		t.Error("empty reminder should fail")
	}

	b, _ := os.ReadFile(v.Abs(RemindersFile))
	want := "---\ntitle: Reminders\ncreated: 2026-10-26\ntags: [reminders]\n---\n\n# Reminders\n\n" +
		"- [ ] Call the dentist · due 2026-10-30\n" +
		"- [ ] Buy milk\n"
	if string(b) != want {
		t.Errorf("reminders.md =\n%q\nwant\n%q", b, want)
	}

	rs, err := v.Reminders()
	if err != nil || len(rs) != 2 {
		t.Fatalf("Reminders = %+v, %v", rs, err)
	}
	if rs[0] != (Reminder{N: 1, Text: "Call the dentist", Due: "2026-10-30"}) || rs[1] != (Reminder{N: 2, Text: "Buy milk"}) {
		t.Errorf("Reminders = %+v", rs)
	}
}

func TestParseReminderLine(t *testing.T) {
	cases := map[string]Reminder{
		"- [ ] plain": {Text: "plain"},
		"- [x] tea · due 2026-10-30 · done 2026-10-26": {Text: "tea", Done: true, Due: "2026-10-30", DoneOn: "2026-10-26"},
		"- [X] shouting":                   {Text: "shouting", Done: true},
		"- [ ] a · b · planned 2026-10-28": {Text: "a · b", Planned: "2026-10-28"},
		"- [ ] due tomorrow · due someday": {Text: "due tomorrow · due someday"},
	}
	for line, want := range cases {
		got, ok := parseReminder(line)
		if !ok || got != want {
			t.Errorf("parseReminder(%q) = %+v, %v; want %+v", line, got, ok, want)
		}
	}
	for _, line := range []string{"# Reminders", "- plain bullet", "-  [ ] two spaces", ""} {
		if _, ok := parseReminder(line); ok {
			t.Errorf("%q is not a reminder", line)
		}
	}
}

func TestCompleteReminder(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local)
	v.AddReminder("one", time.Time{}, now)
	v.AddReminder("two", time.Time{}, now)
	// Hand-written lines between reminders do not shift the numbers.
	v.Append(RemindersFile, "\nSome notes in between.\n")
	v.AddReminder("three", time.Time{}, now)

	r, err := v.CompleteReminder(2, now.AddDate(0, 0, 1))
	if err != nil || !r.Done || r.DoneOn != "2026-10-27" || r.Text != "two" {
		t.Fatalf("CompleteReminder = %+v, %v", r, err)
	}
	b, _ := os.ReadFile(v.Abs(RemindersFile))
	if s := string(b); !strings.Contains(s, "- [ ] one\n- [x] two · done 2026-10-27\n\nSome notes in between.\n- [ ] three\n") {
		t.Errorf("reminders.md:\n%s", s)
	}
	if _, err := v.CompleteReminder(2, now); err == nil {
		t.Error("completing a done reminder should fail")
	}
	if _, err := v.CompleteReminder(9, now); err == nil {
		t.Error("a missing reminder should fail")
	}
	if r, err := v.CompleteReminder(3, now); err != nil || r.Text != "three" {
		t.Errorf("third = %+v, %v", r, err)
	}
}

func TestPlanReminder(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local)
	v.AddReminder("Call the dentist", time.Time{}, now)
	v.AddReminder("Buy milk", time.Time{}, now)
	wed := time.Date(2026, 10, 28, 0, 0, 0, 0, time.Local)

	r, rel, err := v.PlanReminder(1, wed)
	if err != nil || rel != "agenda/2026-W44.md" || r.Planned != "2026-10-28" {
		t.Fatalf("PlanReminder = %+v, %q, %v", r, rel, err)
	}
	if _, _, err := v.PlanReminder(2, wed); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(v.Abs(rel))
	want := "## Wednesday 28 October\n\n### Planned\n\n" +
		"- [ ] Call the dentist ([[reminders]])\n" +
		"- [ ] Buy milk ([[reminders]])\n\n" +
		"### Doing\n\n### Done\n\n## Thursday 29 October\n"
	if s := string(b); !strings.Contains(s, want) {
		t.Errorf("agenda:\n%s\nwant to contain:\n%s", s, want)
	}
	b, _ = os.ReadFile(v.Abs(RemindersFile))
	if s := string(b); !strings.Contains(s, "- [ ] Call the dentist · planned 2026-10-28\n") {
		t.Errorf("reminders.md:\n%s", s)
	}

	// Done ticks the planned agenda line too.
	if _, err := v.CompleteReminder(1, now); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(v.Abs(rel))
	if s := string(b); !strings.Contains(s, "- [x] Call the dentist ([[reminders]])\n- [ ] Buy milk ([[reminders]])\n") {
		t.Errorf("agenda after done:\n%s", s)
	}
	if _, _, err := v.PlanReminder(1, wed); err == nil {
		t.Error("planning a done reminder should fail")
	}
}

func TestPlanReminderLastDayAndMissingSection(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local)
	v.AddReminder("weekend", time.Time{}, now)
	sun := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)

	// The owner removed the subsections of Sunday: the item goes at the end of the day.
	rel, _, _ := v.Agenda(sun)
	b, _ := os.ReadFile(v.Abs(rel))
	s := strings.Replace(string(b), "## Sunday 1 November\n\n### Planned\n\n### Doing\n\n### Done\n", "## Sunday 1 November\n\nfree day", 1)
	os.WriteFile(v.Abs(rel), []byte(s), 0o644)

	if _, _, err := v.PlanReminder(1, sun); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(v.Abs(rel))
	if s := string(b); !strings.HasSuffix(s, "## Sunday 1 November\n\nfree day\n- [ ] weekend ([[reminders]])\n") {
		t.Errorf("agenda:\n%q", s)
	}
}

func TestParseDay(t *testing.T) {
	now := time.Date(2026, 10, 28, 15, 0, 0, 0, time.Local) // a Wednesday
	cases := map[string]string{
		"2026-11-03": "2026-11-03",
		"today":      "2026-10-28",
		"Tomorrow":   "2026-10-29",
		"+3":         "2026-10-31",
		"+3d":        "2026-10-31",
		"friday":     "2026-10-30",
		"wednesday":  "2026-11-04", // the next one, not today
		"mon":        "2026-11-02",
		"vandaag":    "2026-10-28",
		"morgen":     "2026-10-29",
		"vrijdag":    "2026-10-30",
		"ma":         "2026-11-02",
		"Zondag":     "2026-11-01",
	}
	for in, want := range cases {
		got, err := ParseDay(in, now)
		if err != nil || got.Format("2006-01-02") != want {
			t.Errorf("ParseDay(%q) = %v, %v; want %s", in, got, err, want)
		}
	}
	for _, in := range []string{"", "someday", "2026-13-01", "+x", "m", "fr"} {
		if _, err := ParseDay(in, now); err == nil {
			t.Errorf("ParseDay(%q) should fail", in)
		}
	}
}
