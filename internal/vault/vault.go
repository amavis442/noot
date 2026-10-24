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
	gi := filepath.Join(v.Root, ".gitignore")
	b, err := os.ReadFile(gi)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if !strings.Contains(string(b), MetaDir+"/") {
		f, err := os.OpenFile(gi, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if len(b) > 0 && !strings.HasSuffix(string(b), "\n") {
			fmt.Fprintln(f)
		}
		fmt.Fprintf(f, "# noot index (generated)\n%s/\n", MetaDir)
	}
	return nil
}

// Walk returns every note file with its state, keyed by vault-relative path.
func (v *Vault) Walk() (map[string]domain.FileState, error) {
	out := map[string]domain.FileState{}
	err := filepath.WalkDir(v.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != v.Root && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
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
	return true
}

// NewNote creates dir/<slug>.md with frontmatter and returns its relative path.
func (v *Vault) NewNote(title, dir string, tags []string, now time.Time) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", errors.New("title is required")
	}
	slug := Slugify(title)
	if slug == "" {
		slug = now.Format("20060102150405")
	}
	rel := path.Join(filepath.ToSlash(dir), slug+".md")
	for i := 2; exists(v.Abs(rel)); i++ {
		rel = path.Join(filepath.ToSlash(dir), fmt.Sprintf("%s-%d.md", slug, i))
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", now.Format("20060102150405"))
	fmt.Fprintf(&b, "title: %s\n", yamlString(title))
	fmt.Fprintf(&b, "created: %s\n", now.Format("2006-01-02"))
	fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(tags, ", "))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)
	return rel, v.write(rel, b.String())
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
