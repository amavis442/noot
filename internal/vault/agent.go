package vault

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed all:templates/agent
var agentTemplates embed.FS

// writeTemplates copies templates/agent/* into the vault without overwriting.
func (v *Vault) writeTemplates() error {
	const base = "templates/agent"
	return fs.WalkDir(agentTemplates, base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, base+"/")
		if exists(v.Abs(rel)) {
			return nil
		}
		b, err := agentTemplates.ReadFile(p)
		if err != nil {
			return err
		}
		return v.write(rel, strings.ReplaceAll(string(b), "{{date}}", time.Now().Format("2006-01-02")))
	})
}

// LogEntry is one entry for the daily log.
type LogEntry struct {
	Text    string
	Tags    []string // rendered as #tags in the entry heading
	Project string   // rendered as a [[wikilink]] so the project note gets a backlink
}

// AppendLog appends an entry to logs/YYYY-MM-DD.md (created when missing).
func (v *Vault) AppendLog(e LogEntry, now time.Time) (string, error) {
	text := strings.TrimSpace(e.Text)
	if text == "" {
		return "", fmt.Errorf("log entry is empty")
	}
	date := now.Format("2006-01-02")
	rel := "logs/" + date + ".md"
	if !exists(v.Abs(rel)) {
		head := fmt.Sprintf("---\ntitle: Log %s\ncreated: %s\ntags: [log]\n---\n\n# Log %s\n", date, date, date)
		if err := v.write(rel, head); err != nil {
			return "", err
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## %s", now.Format("15:04"))
	if e.Project != "" {
		fmt.Fprintf(&b, " · [[%s]]", e.Project)
	}
	for _, t := range e.Tags {
		fmt.Fprintf(&b, " #%s", strings.TrimPrefix(t, "#"))
	}
	fmt.Fprintf(&b, "\n\n%s\n", text)
	return rel, v.Append(rel, b.String())
}

// Append adds text to the end of a note, making sure it starts on a new line.
func (v *Vault) Append(rel, text string) error {
	abs := v.Abs(rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	cur, err := os.ReadFile(abs)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(cur) > 0 && !strings.HasSuffix(string(cur), "\n") {
		text = "\n" + text
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	_, err = f.WriteString(text)
	return err
}
