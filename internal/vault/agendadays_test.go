package vault

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/config"
)

func newAgenda(t *testing.T, monday time.Time) (*Vault, string) {
	t.Helper()
	v := &Vault{Root: t.TempDir()}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	rel, _, err := v.Agenda(monday)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(v.Abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	return v, string(b)
}

func TestSplitAgenda(t *testing.T) {
	monday := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	_, content := newAgenda(t, monday)
	// Free text under Sunday, with its own "##" heading, belongs to Sunday.
	content += "## Notities\n\nVoor volgende week.\n"

	intro, days := SplitAgenda(content, monday)
	if !strings.HasPrefix(intro, "---\n") || !strings.HasSuffix(intro, "# Week 45, 2026\n\n") {
		t.Errorf("intro = %q", intro)
	}
	if len(days) != 7 {
		t.Fatalf("days = %d, want 7", len(days))
	}
	mon := days[0]
	if !mon.Day.Equal(monday) || mon.Heading != "Monday 2 November" || mon.Body != "### Planned\n\n### Doing\n\n### Done" {
		t.Errorf("monday = %+v", mon)
	}
	if sun := days[6]; sun.Heading != "Sunday 8 November" || !strings.HasSuffix(sun.Body, "## Notities\n\nVoor volgende week.") {
		t.Errorf("sunday = %+v", sun)
	}
}

func TestSplitAgendaSkipsMissingDays(t *testing.T) {
	monday := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	content := "# Week\n\n## Wednesday 4 November\n\nIets\n"
	intro, days := SplitAgenda(content, monday)
	if intro != "# Week\n\n" || len(days) != 1 || days[0].Heading != "Wednesday 4 November" || days[0].Body != "Iets" {
		t.Errorf("intro %q, days %+v", intro, days)
	}
}

func TestSetAgendaDay(t *testing.T) {
	monday := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	tuesday := monday.AddDate(0, 0, 1)
	_, content := newAgenda(t, monday)

	// Writing a day back unchanged leaves the file as it was.
	_, days := SplitAgenda(content, monday)
	same, err := SetAgendaDay(content, monday, tuesday, days[1].Body)
	if err != nil || same != content {
		t.Fatalf("unchanged body rewrote the file: %v\n%s", err, same)
	}

	got, err := SetAgendaDay(content, monday, tuesday, "\n### Planned\n\n- [ ] Tandarts\r\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "## Tuesday 3 November\n\n### Planned\n\n- [ ] Tandarts\n\n## Wednesday 4 November\n") {
		t.Errorf("tuesday not replaced:\n%s", got)
	}
	_, after := SplitAgenda(got, monday)
	if after[0].Body != days[0].Body || after[2].Body != days[2].Body {
		t.Error("other days changed")
	}

	// The last day ends the file the way a new agenda does.
	got, err = SetAgendaDay(content, monday, monday.AddDate(0, 0, 6), "Rust")
	if err != nil || !strings.HasSuffix(got, "## Sunday 8 November\n\nRust\n\n") {
		t.Errorf("sunday: %v\n%q", err, got[len(got)-40:])
	}

	// An empty day keeps just its heading.
	got, err = SetAgendaDay(content, monday, tuesday, "  \n")
	if err != nil || !strings.Contains(got, "## Tuesday 3 November\n\n## Wednesday 4 November\n") {
		t.Errorf("empty day: %v\n%s", err, got)
	}
}

func TestSetAgendaDayErrors(t *testing.T) {
	monday := time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	content := "# Week\n\n## Monday 2 November\n\nx\n"

	if _, err := SetAgendaDay(content, monday, monday.AddDate(0, 0, 1), "y"); err == nil || !strings.Contains(err.Error(), "Tuesday 3 November") {
		t.Errorf("missing day: %v", err)
	}
	// Another day's heading inside a day would move that day into this one.
	_, err := SetAgendaDay(content, monday, monday, "x\n\n## Tuesday 3 November\n\ny")
	if err == nil || !strings.Contains(err.Error(), "heading of another day") {
		t.Errorf("heading of another day: %v", err)
	}
	// Other "##" headings are fine.
	if _, err := SetAgendaDay(content, monday, monday, "## Notities"); err != nil {
		t.Errorf("own heading: %v", err)
	}
}
