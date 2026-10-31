package kit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/tui/kit"
)

func TestGlamourRendersMarkdownWithoutFrontmatter(t *testing.T) {
	md := kit.NewGlamour("dark")
	out, err := md.Render("---\ntitle: Brood\n---\n# Brood\n\nVoeden met **rogge**.\n", 60)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "title: Brood") || strings.Contains(out, "**") {
		t.Fatalf("frontmatter or Markdown syntax left in:\n%s", out)
	}
	if !strings.Contains(out, "Brood") || !strings.Contains(out, "rogge") {
		t.Fatalf("text missing:\n%s", out)
	}
}

func TestGlamourWrapsToWidth(t *testing.T) {
	md := kit.NewGlamour("notty")
	out, err := md.Render(strings.Repeat("woord ", 40), 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out, "\n") {
		if w := len([]rune(strings.TrimRight(line, " "))); w > 30 {
			t.Fatalf("line of %d runes, want at most 30: %q", w, line)
		}
	}
}

func TestPlainKeepsMarkdownButDropsFrontmatter(t *testing.T) {
	out, err := kit.Plain{}.Render("---\ntitle: X\n---\n# X\n", 40)
	if err != nil || out != "# X\n" {
		t.Fatalf("Render = %q, %v", out, err)
	}
}

func TestDutchDate(t *testing.T) {
	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local)
	if got := kit.DutchDate(day); got != "zaterdag 31 oktober 2026" {
		t.Fatalf("DutchDate = %q", got)
	}
}

func TestSameDay(t *testing.T) {
	a := time.Date(2026, 10, 31, 8, 0, 0, 0, time.Local)
	if !kit.SameDay(a, a.Add(10*time.Hour)) || kit.SameDay(a, a.AddDate(0, 0, 1)) {
		t.Fatal("SameDay compares calendar days")
	}
}

func TestDutchDayMonth(t *testing.T) {
	if got := kit.DutchDayMonth(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)); got != "1 maart" {
		t.Fatalf("DutchDayMonth = %q", got)
	}
}
