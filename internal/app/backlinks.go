package app

import (
	"context"

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
