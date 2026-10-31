package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/amavis442/noot/internal/domain"
)

// NewNoteDir is the folder CreateNote puts a note in when the draft names none.
const NewNoteDir = "zettels"

// ErrInvalidNote is wrapped by CreateNote when a draft cannot become a note.
var ErrInvalidNote = errors.New("invalid note")

// ErrConflict is wrapped by SaveNote when the file changed after the caller
// read it.
var ErrConflict = errors.New("changed in the meantime")

// tagRe is the character set of a #tag, as the parser recognises it.
var tagRe = regexp.MustCompile(`^[\p{L}\p{N}_\-/]+$`)

// NoteDraft is the input for a new note.
type NoteDraft struct {
	Title string
	Body  string   // optional Markdown below the title heading
	Dir   string   // folder inside the vault; NewNoteDir when empty
	Tags  []string // written to the frontmatter
}

func (d NoteDraft) validate() error {
	if strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidNote)
	}
	if strings.ContainsAny(d.Title, "\r\n") {
		return fmt.Errorf("%w: title must be a single line", ErrInvalidNote)
	}
	for _, t := range d.Tags {
		if !tagRe.MatchString(t) {
			return fmt.Errorf("%w: tag %q may only contain letters, digits, _, - and /", ErrInvalidNote, t)
		}
	}
	return nil
}

// CreateNote writes a note for the draft and indexes it, so it can be searched
// right away.
func (s *Service) CreateNote(ctx context.Context, d NoteDraft, now time.Time) (*domain.NoteRef, error) {
	if err := d.validate(); err != nil {
		return nil, err
	}
	if d.Dir == "" {
		d.Dir = NewNoteDir
	}
	rel, err := s.Vault.NewNote(d.Title, d.Dir, d.Tags, d.Body, now)
	if err != nil {
		return nil, err
	}
	if _, err := s.Sync(ctx, false); err != nil {
		return nil, err
	}
	return s.MustResolve(ctx, rel)
}

// Search runs a full-text search on the index.
func (s *Service) Search(ctx context.Context, q domain.SearchQuery) ([]domain.SearchHit, error) {
	return s.Store.Search(ctx, q)
}

// ReadNote returns the note ref resolves to with its file content and
// version. An unknown ref gives an error matching ErrNotFound.
func (s *Service) ReadNote(ctx context.Context, ref string) (*domain.Document, error) {
	note, err := s.MustResolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.Vault.Abs(note.Path))
	if err != nil {
		return nil, err
	}
	return &domain.Document{NoteRef: *note, Content: string(b), Version: version(b)}, nil
}

// SaveNote replaces the content of the note ref resolves to. ver is the
// Version the caller read; when the file has changed or gone since, nothing is
// written and the error wraps ErrConflict.
func (s *Service) SaveNote(ctx context.Context, ref, content, ver string) (*domain.Document, error) {
	note, err := s.MustResolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	abs := s.Vault.Abs(note.Path)
	cur, err := os.ReadFile(abs)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%s no longer exists: %w", note.Path, ErrConflict)
	}
	if err != nil {
		return nil, err
	}
	if version(cur) != ver {
		return nil, fmt.Errorf("%s was %w; reload it and edit again", note.Path, ErrConflict)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return s.readFresh(ctx, note.Path)
}

// readFresh syncs the index, so a note just created or changed on disk is
// known with its current title, and then reads the note at rel.
func (s *Service) readFresh(ctx context.Context, rel string) (*domain.Document, error) {
	if _, err := s.Sync(ctx, false); err != nil {
		return nil, err
	}
	return s.ReadNote(ctx, rel)
}

// version identifies file content, so a save can tell whether it changed.
func version(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:8])
}
