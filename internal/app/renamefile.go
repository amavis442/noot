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
)

// renameFile moves a file that is not a note and rewrites the links to it in
// every note that has one. The index must be up to date.
func (s *Service) renameFile(ctx context.Context, from string, opt RenameOptions) (*RenameResult, error) {
	if opt.Title != "" {
		return nil, errors.New("a file has no title: give a new path")
	}
	to, err := fileRenameTarget(from, opt.To)
	if err != nil {
		return nil, err
	}
	if to == from {
		return nil, errors.New("nothing to rename: give a new path")
	}
	if !strings.EqualFold(to, from) {
		if _, err := os.Stat(s.Vault.Abs(to)); err == nil {
			return nil, fmt.Errorf("%s already exists", to)
		}
	}
	// A link by file name means the shortest path. Moving this file to a
	// shorter path than another file of that name would take its links over.
	if other, err := s.Store.ResolveFile(ctx, path.Base(to)); err != nil {
		return nil, err
	} else if other != nil && other.Path != from && !strings.EqualFold(other.Path, to) && shorterPath(to, other.Path) {
		return nil, fmt.Errorf("%s would take over the links by file name to %s: choose a longer path", to, other.Path)
	}

	r := &fileRenamer{svc: s, ctx: ctx, from: from, to: to}
	res := &RenameResult{From: from, To: to, DryRun: opt.DryRun}

	bls, err := s.Store.FileBacklinks(ctx, from)
	if err != nil {
		return nil, err
	}
	writes := map[string][]byte{}
	for _, bl := range bls {
		src := bl.From.Path
		if _, done := writes[src]; done {
			continue
		}
		b, err := os.ReadFile(s.Vault.Abs(src))
		if err != nil {
			return nil, err
		}
		nb, n, err := parser.RewriteLinks(src, b, r.rewriter(path.Dir(src)))
		if err != nil {
			return nil, err
		}
		if n > 0 {
			writes[src] = nb
			res.Changed = append(res.Changed, src)
			res.Links += n
		}
	}
	if opt.DryRun {
		return res, nil
	}

	before, err := s.brokenKeys(ctx, "", "")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(s.Vault.Abs(to)), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(s.Vault.Abs(from), s.Vault.Abs(to)); err != nil {
		return nil, err
	}
	for src, b := range writes {
		if err := os.WriteFile(s.Vault.Abs(src), b, 0o644); err != nil {
			return nil, err
		}
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

// shorterPath orders paths the way the index picks a file by name: the
// shortest path first, then alphabetically.
func shorterPath(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return strings.ToLower(a) < strings.ToLower(b)
}

// fileRenameTarget turns the new path of a file into a vault-relative path. A
// bare name stays in the same folder and a missing extension is kept.
func fileRenameTarget(from, to string) (string, error) {
	to = strings.TrimSpace(filepath.ToSlash(to))
	if to == "" {
		return "", errors.New("nothing to rename: give a new path")
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
	if path.Ext(to) == "" {
		to += path.Ext(from)
	}
	if strings.EqualFold(path.Ext(to), ".md") {
		return "", fmt.Errorf("%s would turn the file into a note", to)
	}
	for _, part := range strings.Split(to, "/") {
		if strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("%s is hidden: noot does not index hidden files", to)
		}
	}
	return to, nil
}

// fileRenamer decides the new target of each link to the moved file.
type fileRenamer struct {
	svc      *Service
	ctx      context.Context
	from, to string
}

// rewriter returns the RewriteFunc for a note in dir.
func (r *fileRenamer) rewriter(dir string) parser.RewriteFunc {
	return func(l domain.Link, written string) (string, bool) {
		var repl string
		switch l.Kind {
		case domain.LinkMarkdown:
			if !strings.EqualFold(l.Target, r.from) {
				return "", false
			}
			repl = relLink(dir, r.to, written)
		case domain.LinkWiki:
			var ok bool
			if repl, ok = r.wikiTarget(l.Target); !ok {
				return "", false
			}
		default:
			return "", false
		}
		if repl == written {
			return "", false
		}
		return repl, true
	}
}

// wikiTarget rewrites a [[target]] that resolves to the moved file in the form
// it was written in: the path or the file name. A name that would no longer
// point to the file alone (a note or another file answers to it) becomes the
// path, which is unique.
func (r *fileRenamer) wikiTarget(target string) (string, bool) {
	if note, err := r.svc.Store.Resolve(r.ctx, target); err != nil || note != nil {
		return "", false // a note wins over a file
	}
	if f, err := r.svc.Store.ResolveFile(r.ctx, target); err != nil || f == nil || f.Path != r.from {
		return "", false
	}
	key := strings.ToLower(target)
	switch {
	case !strings.Contains(target, "/") && key == strings.ToLower(path.Base(r.from)):
		// Written as a file name (also for a file in the vault root, where the
		// path is the file name).
		name := path.Base(r.to)
		if note, err := r.svc.Store.Resolve(r.ctx, name); err != nil || note != nil {
			return r.to, true
		}
		if other, err := r.svc.Store.ResolveFile(r.ctx, name); err != nil || (other != nil && other.Path != r.from) {
			return r.to, true
		}
		return name, true
	case key == strings.ToLower(r.from):
		return r.to, true
	}
	return "", false
}
