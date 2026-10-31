package kit_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/amavis442/noot/internal/tui/kit"
)

func TestPaneShowsHeadingAndRenderedMarkdown(t *testing.T) {
	p := kit.NewPane(kit.Plain{})
	p.SetSize(60, 10)
	p.SetHeading("zaterdag 31 oktober 2026", "journal/2026-10-31.md")
	p.SetMarkdown("---\ntitle: x\n---\nVandaag gebakken.")
	out := p.View()
	for _, want := range []string{"zaterdag 31 oktober 2026", "journal/2026-10-31.md", "Vandaag gebakken."} {
		if !strings.Contains(out, want) {
			t.Fatalf("pane lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "title: x") {
		t.Fatalf("frontmatter shown:\n%s", out)
	}
}

func TestPaneShowsErrorsInsteadOfContent(t *testing.T) {
	p := kit.NewPane(kit.Plain{})
	p.SetSize(60, 10)
	p.SetMarkdown("tekst")
	p.SetError(errors.New("schijf vol"))
	if out := p.View(); !strings.Contains(out, "schijf vol") || strings.Contains(out, "tekst") {
		t.Fatalf("view:\n%s", out)
	}
	p.SetError(nil)
	if out := p.View(); !strings.Contains(out, "tekst") {
		t.Fatalf("content back after the error cleared:\n%s", out)
	}
}

func TestPaneScrollsToText(t *testing.T) {
	p := kit.NewPane(kit.Plain{})
	p.SetSize(60, 5)
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("regel %d", i))
	}
	p.SetMarkdown(strings.Join(lines, "\n"))
	p.ScrollTo("regel 20")
	out := p.View()
	if !strings.Contains(out, "regel 20") || strings.Contains(out, "regel 1\n") {
		t.Fatalf("not scrolled to regel 20:\n%s", out)
	}
}
