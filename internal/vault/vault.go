// Package vault knows the on-disk layout: where the root is, which files are
// notes, and how new notes and journal entries are created.
package vault

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/amavis442/noot/internal/config"
	"github.com/amavis442/noot/internal/domain"
)

// MetaDir holds noot's own data (the index). It belongs in .gitignore.
const MetaDir = ".noot"

// idLayout names a note after its creation time when the title has no slug.
const idLayout = "20060102150405"

// DefaultDirs are created by `noot init` for a personal vault.
var DefaultDirs = []string{"journal", "zettels", "projects", "attachments"}

// AgentDirs are created by `noot init --agent`.
var AgentDirs = []string{"logs", "memory", "decisions", "projects", "scratch"}

// skipDirs are never scanned.
var skipDirs = map[string]bool{
	".git": true, MetaDir: true, ".obsidian": true, ".logseq": true,
	"node_modules": true, ".trash": true,
}

// Vault is a directory of Markdown notes.
type Vault struct {
	Root string // absolute path
}

// FindUp returns the nearest directory at or above start that contains .noot/.
func FindUp(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, MetaDir)); err == nil && fi.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// Open returns the vault rooted at root, which must be an existing directory.
func Open(root string) (*Vault, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("vault %s is not a directory", abs)
	}
	return &Vault{Root: abs}, nil
}

// IndexFile is the SQLite index location.
func (v *Vault) IndexFile() string { return filepath.Join(v.Root, MetaDir, "index.db") }

// Abs converts a vault-relative path to an absolute OS path.
func (v *Vault) Abs(rel string) string { return filepath.Join(v.Root, filepath.FromSlash(rel)) }

// Rel converts an absolute OS path to a vault-relative slash path.
func (v *Vault) Rel(abs string) (string, error) {
	r, err := filepath.Rel(v.Root, abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(r), nil
}

// Init creates the layout for kind and a .gitignore. Existing files are left alone.
func (v *Vault) Init(kind config.Kind) error {
	dirs := DefaultDirs
	if kind == config.KindAgent {
		dirs = AgentDirs
	}
	for _, d := range append([]string{MetaDir}, dirs...) {
		if err := os.MkdirAll(filepath.Join(v.Root, d), 0o755); err != nil {
			return err
		}
	}
	if kind == config.KindAgent {
		if err := v.writeTemplates(); err != nil {
			return err
		}
	}
	return v.EnsureGitignore()
}

// EnsureGitignore makes sure .noot/ (the generated index) is listed in the
// vault's .gitignore, creating the file when needed.
func (v *Vault) EnsureGitignore() error {
	gi := filepath.Join(v.Root, ".gitignore")
	b, err := os.ReadFile(gi)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if strings.Contains(string(b), MetaDir+"/") {
		return nil
	}
	f, err := os.OpenFile(gi, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(b) > 0 && !strings.HasSuffix(string(b), "\n") {
		fmt.Fprintln(f)
	}
	_, err = fmt.Fprintf(f, "# noot index (generated)\n%s/\n", MetaDir)
	return err
}

// Walk returns every note file with its state, keyed by vault-relative path.
func (v *Vault) Walk() (map[string]domain.FileState, error) {
	out := map[string]domain.FileState{}
	err := filepath.WalkDir(v.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != v.Root && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") || isVaultRoot(p)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsNote(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := v.Rel(p)
		if err != nil {
			return err
		}
		out[rel] = domain.FileState{ModTime: info.ModTime().UnixNano(), Size: info.Size()}
		return nil
	})
	return out, err
}

// IsNote reports whether a file name is a note.
func IsNote(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".md") && !strings.HasPrefix(name, ".")
}

// ShouldWatchDir reports whether a directory is part of the vault (for the watcher).
func (v *Vault) ShouldWatchDir(abs string) bool {
	rel, err := v.Rel(abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	if rel == "." {
		return true
	}
	for _, part := range strings.Split(rel, "/") {
		if skipDirs[part] || strings.HasPrefix(part, ".") {
			return false
		}
	}
	return !isVaultRoot(abs)
}

// isVaultRoot reports whether dir is itself a vault (has .noot/). Nested vaults
// are skipped so D:\Noot can hold several vaults without one indexing the others.
func isVaultRoot(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, MetaDir))
	return err == nil && fi.IsDir()
}

// NewNote creates dir/<slug>.md with frontmatter and returns its relative path.
// A body that is not blank goes below the title heading.
func (v *Vault) NewNote(title, dir string, tags []string, body string, now time.Time) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", errors.New("title is required")
	}
	slug := Slugify(title)
	if slug == "" {
		slug = now.Format(idLayout)
	}
	rel := path.Join(filepath.ToSlash(dir), slug+".md")
	for i := 2; exists(v.Abs(rel)); i++ {
		rel = path.Join(filepath.ToSlash(dir), fmt.Sprintf("%s-%d.md", slug, i))
	}
	id, err := NewID(now)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "title: %s\n", yamlString(title))
	fmt.Fprintf(&b, "created: %s\n", now.Format("2006-01-02"))
	fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(tags, ", "))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)
	if body = strings.TrimSpace(body); body != "" {
		b.WriteString(body + "\n")
	}
	return rel, v.write(rel, b.String())
}

// SetTitle changes a note's title from old to title: the frontmatter `title:`
// line and the first H1 when it reads old. A note without either takes its
// title from the file name, so there is nothing to change.
func SetTitle(content []byte, old, title string) ([]byte, error) {
	lines := strings.SplitAfter(string(content), "\n")
	changed := false
	inFrontmatter, h1Done := false, false
	for i, line := range lines {
		body := strings.TrimRight(line, "\r\n")
		eol := line[len(body):]
		switch {
		case i == 0 && strings.TrimPrefix(body, "\xef\xbb\xbf") == "---":
			inFrontmatter = true
		case inFrontmatter && (body == "---" || body == "..."):
			inFrontmatter = false
		case inFrontmatter:
			if strings.HasPrefix(body, "title:") {
				lines[i] = "title: " + yamlString(title) + eol
				changed = true
			}
		case !h1Done && strings.HasPrefix(body, "# "):
			h1Done = true
			if strings.TrimSpace(body[2:]) == old {
				lines[i] = "# " + title + eol
				changed = true
			}
		}
	}
	if !changed {
		return nil, fmt.Errorf("no title line to change: the title %q comes from the file name, rename the file instead", old)
	}
	return []byte(strings.Join(lines, "")), nil
}

// Journal returns journal/YYYY-MM-DD.md, creating it when missing.
func (v *Vault) Journal(day time.Time) (rel string, created bool, err error) {
	date := day.Format("2006-01-02")
	rel = "journal/" + date + ".md"
	if exists(v.Abs(rel)) {
		return rel, false, nil
	}
	content := fmt.Sprintf("---\ntitle: %s\ncreated: %s\ntags: [journal]\n---\n\n# %s\n\n",
		date, date, day.Format("Monday 2 January 2006"))
	return rel, true, v.write(rel, content)
}

// agendaSections are the subheadings of every day in a weekly agenda.
var agendaSections = []string{"Planned", "Doing", "Done"}

// Agenda returns agenda/YYYY-Www.md for the ISO week (Monday to Sunday) that
// day falls in, creating it when missing with a heading per day and the
// agendaSections under each.
func (v *Vault) Agenda(day time.Time) (rel string, created bool, err error) {
	year, week := day.ISOWeek()
	rel = AgendaPath(day)
	if exists(v.Abs(rel)) {
		return rel, false, nil
	}
	monday := MondayOf(day)

	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: Agenda %d-W%02d\ncreated: %s\ntags: [agenda]\n---\n\n", year, week, monday.Format("2006-01-02"))
	fmt.Fprintf(&b, "# Week %d, %d\n\n", week, year)
	for i := range 7 {
		fmt.Fprintf(&b, "## %s\n\n", monday.AddDate(0, 0, i).Format("Monday 2 January"))
		for _, s := range agendaSections {
			fmt.Fprintf(&b, "### %s\n\n", s)
		}
	}
	return rel, true, v.write(rel, b.String())
}

// AgendaPath is the vault-relative path of the agenda of the ISO week day
// falls in: agenda/YYYY-Www.md.
func AgendaPath(day time.Time) string {
	year, week := day.ISOWeek()
	return fmt.Sprintf("agenda/%d-W%02d.md", year, week)
}

// MondayOf returns the Monday of the ISO week day falls in, at midnight.
func MondayOf(day time.Time) time.Time {
	y, m, d := day.Date()
	monday := time.Date(y, m, d, 0, 0, 0, 0, day.Location())
	return monday.AddDate(0, 0, -(int(monday.Weekday())+6)%7)
}

func (v *Vault) write(rel, content string) error {
	abs := v.Abs(rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(content), 0o644)
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// Slugify turns "Clean architectuur in Go!" into "clean-architectuur-in-go".
func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		r = fold(r)
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// fold strips common Latin diacritics so file names stay ASCII-friendly.
func fold(r rune) rune {
	switch r {
	case 'à', 'á', 'â', 'ã', 'ä', 'å':
		return 'a'
	case 'è', 'é', 'ê', 'ë':
		return 'e'
	case 'ì', 'í', 'î', 'ï':
		return 'i'
	case 'ò', 'ó', 'ô', 'õ', 'ö':
		return 'o'
	case 'ù', 'ú', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	case 'ñ':
		return 'n'
	case 'ÿ', 'ý':
		return 'y'
	}
	return r
}

func yamlString(s string) string {
	if strings.ContainsAny(s, ":#[]{},&*!|>'\"%@`") || strings.TrimSpace(s) != s {
		return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
	}
	return s
}
