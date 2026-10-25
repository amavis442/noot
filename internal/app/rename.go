package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/parser"
	"github.com/amavis442/noot/internal/vault"
)

// RenameOptions says where a note goes and what it is called.
type RenameOptions struct {
	To     string // vault-relative path, ".md" optional; a bare name stays in the same folder; "" keeps the path
	Title  string // "" keeps the title
	DryRun bool   // report what would change without writing
}

// RenameResult reports what a rename did (or would do, for a dry run).
type RenameResult struct {
	From    string              `json:"from"`
	To      string              `json:"to"`
	Changed []string            `json:"changed"` // files whose content changed, the note under its new path last
	Links   int                 `json:"links"`   // links rewritten
	Broken  []domain.BrokenLink `json:"broken"`  // links that no longer resolve after the rename
	DryRun  bool                `json:"dry_run"`
}

// Rename moves a note and/or changes its title, then rewrites every link that
// pointed to it so it still does. The note's own relative Markdown links are
// adjusted to its new folder. The index must be up to date.
func (s *Service) Rename(ctx context.Context, ref string, opt RenameOptions) (*RenameResult, error) {
	found, err := s.MustResolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(s.Vault.Abs(found.Path))
	if err != nil {
		return nil, err
	}
	note, err := parser.Parse(found.Path, content)
	if err != nil {
		return nil, err
	}
	to, err := renameTarget(found.Path, opt.To)
	if err != nil {
		return nil, err
	}
	title := opt.Title
	if title == note.Title {
		title = ""
	}
	if to == note.Path && title == "" {
		return nil, errors.New("nothing to rename: give a new path or a new title")
	}
	if to != note.Path && !strings.EqualFold(to, note.Path) {
		if _, err := os.Stat(s.Vault.Abs(to)); err == nil {
			return nil, fmt.Errorf("%s already exists", to)
		}
	}

	r := &renamer{svc: s, ctx: ctx, note: note, to: to, title: title}
	res := &RenameResult{From: note.Path, To: to, DryRun: opt.DryRun}

	bls, err := s.Store.Backlinks(ctx, note.ID)
	if err != nil {
		return nil, err
	}
	writes := map[string][]byte{}
	for _, bl := range bls {
		src := bl.From.Path
		if _, done := writes[src]; done || src == note.Path {
			continue
		}
		b, err := os.ReadFile(s.Vault.Abs(src))
		if err != nil {
			return nil, err
		}
		nb, n, err := parser.RewriteLinks(src, b, r.rewriter(path.Dir(src), false))
		if err != nil {
			return nil, err
		}
		if n > 0 {
			writes[src] = nb
			res.Changed = append(res.Changed, src)
			res.Links += n
		}
	}

	own := content
	if title != "" {
		if own, err = vault.SetTitle(own, note.Title, title); err != nil {
			return nil, err
		}
	}
	own, n, err := parser.RewriteLinks(note.Path, own, r.rewriter(path.Dir(to), true))
	if err != nil {
		return nil, err
	}
	res.Links += n
	if n > 0 || title != "" {
		res.Changed = append(res.Changed, to)
	}
	if opt.DryRun {
		return res, nil
	}

	before, err := s.brokenKeys(ctx, note.Path, to)
	if err != nil {
		return nil, err
	}
	for src, b := range writes {
		if err := os.WriteFile(s.Vault.Abs(src), b, 0o644); err != nil {
			return nil, err
		}
	}
	if to != note.Path {
		if err := os.MkdirAll(filepath.Dir(s.Vault.Abs(to)), 0o755); err != nil {
			return nil, err
		}
		if err := os.Rename(s.Vault.Abs(note.Path), s.Vault.Abs(to)); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(s.Vault.Abs(to), own, 0o644); err != nil {
		return nil, err
	}
	if _, err := s.Sync(ctx, false); err != nil {
		return nil, err
	}

	after, err := s.Store.Broken(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range after {
		if !before[brokenKey(b.From.Path, b)] {
			res.Broken = append(res.Broken, b)
		}
	}
	return res, nil
}

// renameTarget turns the --to argument into a vault-relative .md path.
func renameTarget(from, to string) (string, error) {
	to = strings.TrimSpace(filepath.ToSlash(to))
	if to == "" {
		return from, nil
	}
	if filepath.IsAbs(to) || strings.HasPrefix(to, "/") {
		return "", fmt.Errorf("%s: give a path inside the vault", to)
	}
	if !strings.Contains(to, "/") {
		to = path.Join(path.Dir(from), to)
	}
	to = path.Clean(to)
	if to == ".." || strings.HasPrefix(to, "../") {
		return "", fmt.Errorf("%s is outside the vault", to)
	}
	if !strings.EqualFold(path.Ext(to), ".md") {
		to += ".md"
	}
	return to, nil
}

// brokenKeys returns the links that are broken before the rename, keyed with
// the paths they will have afterwards.
func (s *Service) brokenKeys(ctx context.Context, from, to string) (map[string]bool, error) {
	bs, err := s.Store.Broken(ctx)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, b := range bs {
		p := b.From.Path
		if p == from {
			p = to
		}
		keys[brokenKey(p, b)] = true
	}
	return keys, nil
}

func brokenKey(p string, b domain.BrokenLink) string {
	return p + "\x00" + string(b.Kind) + "\x00" + strings.ToLower(b.Target)
}

// renamer decides the new target of each link during a rename.
type renamer struct {
	svc   *Service
	ctx   context.Context
	note  domain.Note // the note as it is before the rename
	to    string      // its new path
	title string      // its new title, "" when unchanged
}

// rewriter returns the RewriteFunc for a file in dir (its folder after the
// rename). self is set for the renamed note, whose other relative links must
// follow it to its new folder.
func (r *renamer) rewriter(dir string, self bool) parser.RewriteFunc {
	return func(l domain.Link, written string) (string, bool) {
		var repl string
		switch {
		case l.Kind == domain.LinkMarkdown && l.Target == r.note.Path:
			repl = relLink(dir, r.to, written)
		case l.Kind == domain.LinkMarkdown && self:
			repl = relLink(dir, l.Target, written)
		case l.Kind == domain.LinkWiki:
			var ok bool
			if repl, ok = r.wikiTarget(l.Target, written); !ok {
				return "", false
			}
		default:
			return "", false
		}
		if strings.EqualFold(repl, written) {
			return "", false
		}
		return repl, true
	}
}

// wikiTarget rewrites a [[target]] that resolves to the renamed note in the
// same form it was written in: path, file name or title. Links by an explicit
// id or an alias keep working and are left alone.
func (r *renamer) wikiTarget(target, written string) (string, bool) {
	ref, err := r.svc.Store.Resolve(r.ctx, target)
	if err != nil || ref == nil || ref.ID != r.note.ID {
		return "", false
	}
	oldBase := strings.TrimSuffix(r.note.Path, path.Ext(r.note.Path))
	newBase := strings.TrimSuffix(r.to, path.Ext(r.to))
	key := strings.ToLower(target)

	var repl string
	switch {
	case key == strings.ToLower(oldBase): // the path, which is also the id when there is none in the frontmatter
		repl = newBase
	case key == strings.ToLower(r.note.ID):
		return "", false
	case key == strings.ToLower(path.Base(oldBase)):
		repl = path.Base(newBase)
	case key == strings.ToLower(r.note.Title) && r.title != "":
		repl = r.title
	default:
		return "", false
	}
	// A file name or title another note already answers to would send the
	// link there; the path is unique.
	if repl != newBase {
		if other, err := r.svc.Store.Resolve(r.ctx, repl); err == nil && other != nil && other.ID != r.note.ID {
			repl = newBase
		}
	}
	if strings.HasSuffix(strings.ToLower(written), ".md") {
		repl += ".md"
	}
	return repl, true
}

// relLink writes a Markdown link from a file in dir to target in the style of
// written: vault-absolute when it started with "/", %20 for spaces when used.
func relLink(dir, target, written string) string {
	if strings.HasPrefix(written, "/") {
		return "/" + target
	}
	rel, err := filepath.Rel(filepath.FromSlash(dir), filepath.FromSlash(target))
	if err != nil {
		return target
	}
	rel = filepath.ToSlash(rel)
	if strings.Contains(written, "%20") {
		rel = strings.ReplaceAll(rel, " ", "%20")
	}
	return rel
}
