package cli

import (
	"bytes"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/vault"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// envs returns an env writing plain text and one writing colours.
func envs() (plain, styled *env, plainOut, styledOut *bytes.Buffer) {
	plainOut, styledOut = &bytes.Buffer{}, &bytes.Buffer{}
	r := lipgloss.NewRenderer(styledOut)
	r.SetColorProfile(termenv.ANSI256)
	r.SetHasDarkBackground(true)
	plain = &env{out: plainOut, paint: newPainter(lipgloss.NewRenderer(plainOut), false)}
	styled = &env{out: styledOut, paint: newPainter(r, true)}
	return plain, styled, plainOut, styledOut
}

// sameText checks that the styled output has colours and is the plain
// output once they are stripped.
func sameText(t *testing.T, plain, styled string) {
	t.Helper()
	if !strings.Contains(styled, "\x1b[") {
		t.Fatalf("no colours in:\n%s", styled)
	}
	if got := ansi.Strip(styled); got != plain {
		t.Fatalf("styled text differs from plain.\nplain:\n%q\nstyled, stripped:\n%q", plain, got)
	}
}

func TestPlainPainterChangesNothing(t *testing.T) {
	p := newPainter(lipgloss.NewRenderer(&bytes.Buffer{}), false)
	for _, s := range []string{p.path("a.md"), p.title("A"), p.muted("x"), p.danger("y"), p.success("z"), p.warning("w"), p.snippet("met «rogge» en")} {
		if strings.Contains(s, "\x1b") {
			t.Fatalf("plain painter added escapes: %q", s)
		}
	}
	if got := p.snippet("met «rogge» en"); got != "met «rogge» en" {
		t.Fatalf("snippet = %q", got)
	}
}

func TestSearchHitsStyled(t *testing.T) {
	hits := []domain.SearchHit{
		{NoteRef: domain.NoteRef{Path: "zettels/brood.md", Title: "Brood"}, Snippet: "…met «rogge» en «spelt»…"},
		{NoteRef: domain.NoteRef{Path: "koffie.md", Title: "Koffie"}},
	}
	for _, vaultName := range []string{"", "patrick"} {
		plain, styled, po, so := envs()
		printHits(plain, vaultName, hits)
		printHits(styled, vaultName, hits)
		sameText(t, po.String(), so.String())
	}
	plain, styled, po, so := envs()
	printHits(plain, "", nil)
	printHits(styled, "", nil)
	sameText(t, po.String(), so.String())
}

func TestStatusStyled(t *testing.T) {
	cases := []app.StatusResult{
		{Changes: []app.FileChange{{Status: "A", Path: "nieuw.md"}, {Status: "M", Path: "oud.md"}, {Status: "D", Path: "weg.md"}}, Ahead: 2, Upstream: true, Remote: true},
		{Upstream: true, Remote: true},
		{},
		{Remote: true},
	}
	for _, st := range cases {
		plain, styled, po, so := envs()
		printStatus(plain, st)
		printStatus(styled, st)
		sameText(t, po.String(), so.String())
	}
}

func TestRemindersStyled(t *testing.T) {
	rs := []vault.Reminder{
		{N: 1, Text: "Tandarts bellen", Due: "2026-10-29"},
		{N: 2, Text: "Café reserveren", Planned: "2026-11-02"},
		{N: 3, Text: "Zonder datum"},
		{N: 12, Text: "Oud klusje", Done: true, Due: "2026-10-01", DoneOn: "2026-10-20"},
	}
	plain, styled, po, so := envs()
	if err := printReminders(plain, rs, "2026-10-31"); err != nil {
		t.Fatal(err)
	}
	if err := printReminders(styled, rs, "2026-10-31"); err != nil {
		t.Fatal(err)
	}
	sameText(t, po.String(), so.String())
	if !strings.Contains(po.String(), "(overdue)") {
		t.Fatalf("overdue missing:\n%s", po.String())
	}
}

// TestRemindersKeepTheirTabwriterLayout: the plain list is laid out as it
// was with text/tabwriter, which colours would throw off.
func TestRemindersKeepTheirTabwriterLayout(t *testing.T) {
	rs := []vault.Reminder{
		{N: 1, Text: "Tandarts bellen", Due: "2026-10-29"},
		{N: 2, Text: "Café", Planned: "2026-11-02"},
		{N: 3, Text: "Zonder datum"},
		{N: 104, Text: "Oud klusje", Done: true, DoneOn: "2026-10-20"},
	}
	// What the list wrote through text/tabwriter before it had colours.
	var want bytes.Buffer
	tw := tabwriter.NewWriter(&want, 0, 4, 2, ' ', 0)
	tw.Write([]byte("  1\t[ ] Tandarts bellen\tdue 2026-10-29 (overdue)\n" +
		"  2\t[ ] Café\tplanned 2026-11-02\n" +
		"  3\t[ ] Zonder datum\t\n" +
		"104\t[x] Oud klusje\tdone 2026-10-20\n"))
	tw.Flush()
	plain, _, po, _ := envs()
	if err := printReminders(plain, rs, "2026-10-31"); err != nil {
		t.Fatal(err)
	}
	if po.String() != want.String() {
		t.Fatalf("layout differs.\nwant:\n%q\ngot:\n%q", want.String(), po.String())
	}
}
