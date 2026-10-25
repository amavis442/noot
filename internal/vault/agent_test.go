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

func TestLogEntries(t *testing.T) {
	content := "---\ntitle: Log 2026-10-24\n---\n\n# Log 2026-10-24\n\n" +
		"## 13:25 · [[noot]] #go\n\nfirst\nstill first\n\n" +
		"## 14:25\n\nsecond\n"
	got := LogEntries(content)
	if len(got) != 2 {
		t.Fatalf("entries = %+v", got)
	}
	if got[0].Heading != "13:25 · [[noot]] #go" || got[0].Text != "first\nstill first" {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].Heading != "14:25" || got[1].Text != "second" {
		t.Errorf("second = %+v", got[1])
	}
	if LogEntries("# Log\n\nno entries yet\n") != nil {
		t.Error("a log without entries should give none")
	}
}

func TestLogDate(t *testing.T) {
	if d, ok := LogDate("logs/2026-10-24.md"); !ok || d.Format("2006-01-02") != "2026-10-24" {
		t.Errorf("LogDate = %v, %v", d, ok)
	}
	for _, p := range []string{"logs/notes.md", "memory/2026-10-24.md"} {
		if _, ok := LogDate(p); ok {
			t.Errorf("%s is not a daily log", p)
		}
	}
}
