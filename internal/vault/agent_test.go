package vault

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/config"
)

func TestAgentInitAndLog(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	if err := v.Init(config.KindAgent); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"AGENTS.md", "memory/index.md", "logs", "decisions", ".gitignore"} {
		if _, err := os.Stat(v.Abs(f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}

	now := time.Date(2026, 10, 24, 13, 25, 0, 0, time.Local)
	rel, err := v.AppendLog(LogEntry{Text: "first", Project: "noot", Tags: []string{"#go"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.AppendLog(LogEntry{Text: "second"}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(v.Abs(rel))
	s := string(b)
	for _, want := range []string{"title: Log 2026-10-24", "## 13:25 · [[noot]] #go\n\nfirst\n", "## 14:25\n\nsecond\n"} {
		if !strings.Contains(s, want) {
			t.Errorf("log missing %q:\n%s", want, s)
		}
	}
	if strings.Count(s, "---\n") != 2 {
		t.Errorf("frontmatter written more than once:\n%s", s)
	}
}

func TestAppendAddsNewline(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	_ = os.WriteFile(v.Abs("a.md"), []byte("no newline"), 0o644)
	if err := v.Append("a.md", "next"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(v.Abs("a.md"))
	if string(b) != "no newline\nnext\n" {
		t.Errorf("got %q", b)
	}
}
