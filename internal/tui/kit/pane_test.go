package kit_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/tui/kit/kittest"
	"github.com/charmbracelet/lipgloss"
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

func TestPaneScrollsOnListAndPageKeysOnly(t *testing.T) {
	p := kit.NewPane(kit.Plain{})
	p.SetSize(60, 5)
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("regel %d", i))
	}
	p.SetMarkdown(strings.Join(lines, "\n"))
	if !p.Scroll(kittest.Key("j")) || !shows(p, "regel 2") || shows(p, "regel 1") {
		t.Fatalf("j scrolls a line:\n%s", p.View())
	}
	if !p.Scroll(kittest.Key("pgdown")) || shows(p, "regel 2") || !shows(p, "regel 5") {
		t.Fatalf("pgdown scrolls a page:\n%s", p.View())
	}
	for _, k := range []string{"h", "l", "f"} {
		if p.Scroll(kittest.Key(k)) {
			t.Fatalf("%s is not a scroll key", k)
		}
	}
}

// shows reports whether one of the pane's lines is exactly line.
func shows(p *kit.Pane, line string) bool {
	for _, l := range strings.Split(p.View(), "\n") {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}

func TestBoxFitsTheGivenSize(t *testing.T) {
	out := kit.Box("inhoud", 30, 6, true)
	if lines := strings.Split(out, "\n"); len(lines) != 6 || lipgloss.Width(lines[0]) != 30 {
		t.Fatalf("box is %d lines of %d cells:\n%s", len(lines), lipgloss.Width(lines[0]), out)
	}
	if w, h := kit.BoxInner(30, 6); w != 26 || h != 4 {
		t.Fatalf("inner size %dx%d", w, h)
	}
}
