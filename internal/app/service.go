// Package app contains the use cases: syncing the index with the vault and
// answering queries. It depends on interfaces, not on SQLite or the CLI.
package app

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/parser"
	"github.com/amavis442/noot/internal/vault"
)

// Store is the index port. internal/storage/sqlite implements it.
type Store interface {
	FileStates(ctx context.Context) (map[string]domain.FileState, error)
	Upsert(ctx context.Context, notes []domain.Note) (skipped []error, err error)
	DeletePaths(ctx context.Context, paths []string) error
	ResolveLinks(ctx context.Context) error
	Reset(ctx context.Context) error
	Count(ctx context.Context) (int, error)

	Resolve(ctx context.Context, ref string) (*domain.NoteRef, error)
	Search(ctx context.Context, q domain.SearchQuery) ([]domain.SearchHit, error)
	Backlinks(ctx context.Context, id string) ([]domain.Backlink, error)
	OutLinks(ctx context.Context, id string) ([]domain.OutLink, error)
	Broken(ctx context.Context) ([]domain.BrokenLink, error)
	Orphans(ctx context.Context) ([]domain.NoteRef, error)
	Tags(ctx context.Context) ([]domain.TagCount, error)
	NotesByTag(ctx context.Context, tag string) ([]domain.NoteRef, error)
}

// Service wires a vault to a store.
type Service struct {
	Vault *vault.Vault
	Store Store
}

// SyncStats reports what a sync did.
type SyncStats struct {
	Scanned, Updated, Deleted, Total int
	Errors                           []error
}

func (s SyncStats) String() string {
	return fmt.Sprintf("%d notes (%d updated, %d removed, %d errors)", s.Total, s.Updated, s.Deleted, len(s.Errors))
}

// Sync brings the index up to date. Files are re-parsed only when their mtime
// or size changed, unless full is set. Parse errors are collected, not fatal.
func (s *Service) Sync(ctx context.Context, full bool) (SyncStats, error) {
	var st SyncStats
	if full {
		if err := s.Store.Reset(ctx); err != nil {
			return st, err
		}
	}
	onDisk, err := s.Vault.Walk()
	if err != nil {
		return st, fmt.Errorf("scan vault: %w", err)
	}
	indexed, err := s.Store.FileStates(ctx)
	if err != nil {
		return st, err
	}
	st.Scanned = len(onDisk)

	var removed []string
	for p := range indexed {
		if _, ok := onDisk[p]; !ok {
			removed = append(removed, p)
		}
	}

	changed := make([]string, 0)
	for p, fs := range onDisk {
		if old, ok := indexed[p]; !ok || old != fs {
			changed = append(changed, p)
		}
	}
	sort.Strings(changed)

	notes := make([]domain.Note, 0, len(changed))
	seen := map[string]string{} // id → path within this batch
	for _, p := range changed {
		b, err := os.ReadFile(s.Vault.Abs(p))
		if err != nil {
			st.Errors = append(st.Errors, err)
			continue
		}
		n, err := parser.Parse(p, b)
		if err != nil {
			st.Errors = append(st.Errors, err)
			continue
		}
		if other, dup := seen[n.ID]; dup {
			st.Errors = append(st.Errors, fmt.Errorf("%s: id %q already used by %s, skipped", p, n.ID, other))
			continue
		}
		seen[n.ID] = p
		n.ModTime, n.Size = onDisk[p].ModTime, onDisk[p].Size
		notes = append(notes, n)
	}

	if err := s.Store.DeletePaths(ctx, removed); err != nil {
		return st, err
	}
	skipped, err := s.Store.Upsert(ctx, notes)
	if err != nil {
		return st, err
	}
	st.Errors = append(st.Errors, skipped...)
	if len(notes) > 0 || len(removed) > 0 {
		if err := s.Store.ResolveLinks(ctx); err != nil {
			return st, err
		}
	}
	st.Updated, st.Deleted = len(notes)-len(skipped), len(removed)
	st.Total, err = s.Store.Count(ctx)
	return st, err
}

// MustResolve finds a note or returns a helpful error.
func (s *Service) MustResolve(ctx context.Context, ref string) (*domain.NoteRef, error) {
	r, err := s.Store.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("no note matches %q (tried id, path, file name, title and aliases)", ref)
	}
	return r, nil
}
