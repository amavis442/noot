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

// LogDir holds the daily logs, one file per day: logs/YYYY-MM-DD.md.
const LogDir = "logs"

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
	rel := LogDir + "/" + date + ".md"
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

// LoggedEntry is an entry read back from a daily log.
type LoggedEntry struct {
	Heading string // the "## " line without the hashes, e.g. "13:25 · [[noot]] #go"
	Text    string // everything up to the next entry, trimmed
}

// LogEntries splits a daily log into its "## " entries. Text before the first
// entry (frontmatter, the "# Log" title) is not part of any entry.
func LogEntries(content string) []LoggedEntry {
	var out []LoggedEntry
	var text strings.Builder
	flush := func() {
		if len(out) > 0 {
			out[len(out)-1].Text = strings.TrimSpace(text.String())
		}
		text.Reset()
	}
	for _, line := range strings.SplitAfter(content, "\n") {
		if h, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "## "); ok {
			flush()
			out = append(out, LoggedEntry{Heading: strings.TrimSpace(h)})
			continue
		}
		text.WriteString(line)
	}
	flush()
	return out
}

// LogDate returns the day of a daily log from its vault-relative path.
func LogDate(rel string) (time.Time, bool) {
	name, ok := strings.CutPrefix(rel, LogDir+"/")
	if !ok || !strings.HasSuffix(name, ".md") {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation("2006-01-02", strings.TrimSuffix(name, ".md"), time.Local)
	return d, err == nil
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
