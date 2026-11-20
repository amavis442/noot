package app

import (
	"context"
	"fmt"
	"os"

	"github.com/amavis442/noot/internal/domain"
)

// BacklinksTo returns the vault-relative path of what ref names and the notes
// linking to it. ref is a note (id, path, file name, title or alias) or, when
// no note answers to it, a file of the vault such as an image or a PDF.
func (s *Service) BacklinksTo(ctx context.Context, ref string) (string, []domain.Backlink, error) {
	note, err := s.Store.Resolve(ctx, ref)
	if err != nil {
		return "", nil, err
	}
	if note != nil {
		bls, err := s.Store.Backlinks(ctx, note.ID)
		return note.Path, bls, err
	}
	file, err := s.Store.ResolveFile(ctx, ref)
	if err != nil {
		return "", nil, err
	}
	if file == nil {
		return "", nil, notFoundError{ref}
	}
	bls, err := s.Store.FileBacklinks(ctx, file.Path)
	return file.Path, bls, err
}

// LinkFile returns the vault path of the file that a wikilink target names. A
// note answering to the target wins, then the result is false.
func (s *Service) LinkFile(ctx context.Context, target string) (string, bool, error) {
	note, err := s.Store.Resolve(ctx, target)
	if err != nil || note != nil {
		return "", false, err
	}
	file, err := s.Store.ResolveFile(ctx, target)
	if err != nil || file == nil {
		return "", false, err
	}
	return file.Path, true, nil
}

// FileLocation returns where the file at the vault-relative path rel is on
// disk. Only a regular file the index knows is found: no note, hidden file,
// folder, symbolic link or path outside the vault.
func (s *Service) FileLocation(ctx context.Context, rel string) (string, error) {
	file, err := s.Store.ResolveFile(ctx, rel)
	if err != nil {
		return "", err
	}
	if file == nil || file.Path != rel {
		return "", fileNotFoundError{rel}
	}
	abs := s.Vault.Abs(file.Path)
	if fi, err := os.Lstat(abs); err != nil || !fi.Mode().IsRegular() {
		return "", fileNotFoundError{rel}
	}
	return abs, nil
}

type fileNotFoundError struct{ path string }

func (e fileNotFoundError) Error() string        { return fmt.Sprintf("no file at %q", e.path) }
func (e fileNotFoundError) Is(target error) bool { return target == ErrNotFound }
