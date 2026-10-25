package app

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/parser"
	"github.com/amavis442/noot/internal/vault"
)

// ContextOptions limits what Context takes from the daily logs.
type ContextOptions struct {
	Days  int // only logs of the last Days days, today included; 0 means all
	Limit int // at most Limit log entries, the newest; 0 means all
	Now   time.Time
}

// Context gathers a note, the notes around it and the recent log entries that
// link to it, so one call is enough to pick up work on a project.
func (s *Service) Context(ctx context.Context, ref string, opt ContextOptions) (*domain.NoteContext, error) {
	note, err := s.MustResolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.Vault.Abs(note.Path))
	if err != nil {
		return nil, err
	}
	nc := &domain.NoteContext{Note: *note, Content: string(b)}

	bls, err := s.Store.Backlinks(ctx, note.ID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{note.Path: true}
	var logPaths []string
	for _, bl := range bls {
		if seen[bl.From.Path] {
			continue
		}
		seen[bl.From.Path] = true
		if _, ok := vault.LogDate(bl.From.Path); ok {
			logPaths = append(logPaths, bl.From.Path)
			continue
		}
		nc.Related = append(nc.Related, bl)
	}

	siblings, err := s.siblings(ctx, note.Path)
	if err != nil {
		return nil, err
	}
	for _, sib := range siblings {
		if !seen[sib.Path] {
			seen[sib.Path] = true
			nc.Related = append(nc.Related, domain.Backlink{From: sib})
		}
	}

	nc.Logs, err = s.logExcerpts(ctx, *note, logPaths, opt)
	return nc, err
}

// siblings returns the other notes in the folder of a folder note, the
// projects/<project>/<project>.md layout where plans sit next to the main note.
func (s *Service) siblings(ctx context.Context, rel string) ([]domain.NoteRef, error) {
	dir := path.Dir(rel)
	if dir == "." || path.Base(dir) != strings.TrimSuffix(path.Base(rel), path.Ext(rel)) {
		return nil, nil
	}
	return s.Store.NotesInDir(ctx, dir)
}

// logExcerpts returns the entries of the given daily logs that link to note.
// logPaths are sorted by path, which for daily logs is by date.
func (s *Service) logExcerpts(ctx context.Context, note domain.NoteRef, logPaths []string, opt ContextOptions) ([]domain.LogExcerpt, error) {
	var since time.Time
	if opt.Days > 0 {
		y, m, d := opt.Now.Date()
		since = time.Date(y, m, d, 0, 0, 0, 0, opt.Now.Location()).AddDate(0, 0, 1-opt.Days)
	}
	var out []domain.LogExcerpt
	for _, p := range logPaths {
		if day, _ := vault.LogDate(p); day.Before(since) {
			continue
		}
		b, err := os.ReadFile(s.Vault.Abs(p))
		if err != nil {
			return nil, err
		}
		for _, entry := range vault.LogEntries(string(b)) {
			ok, err := s.linksTo(ctx, note, p, "## "+entry.Heading+"\n\n"+entry.Text)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, domain.LogExcerpt{Path: p, Heading: entry.Heading, Text: entry.Text})
			}
		}
	}
	if opt.Limit > 0 && len(out) > opt.Limit {
		out = out[len(out)-opt.Limit:]
	}
	return out, nil
}

// linksTo reports whether a piece of Markdown from file rel links to note.
func (s *Service) linksTo(ctx context.Context, note domain.NoteRef, rel, text string) (bool, error) {
	n, err := parser.Parse(rel, []byte(text))
	if err != nil {
		return false, err
	}
	for _, l := range n.Links {
		if l.Kind == domain.LinkMarkdown {
			if l.Target == note.Path {
				return true, nil
			}
			continue
		}
		r, err := s.Store.Resolve(ctx, l.Target)
		if err != nil {
			return false, err
		}
		if r != nil && r.ID == note.ID {
			return true, nil
		}
	}
	return false, nil
}
