package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/amavis442/noot/internal/domain"
)

// ErrConflict is wrapped by SaveJournal when the file changed after the
// caller read it.
var ErrConflict = errors.New("changed in the meantime")

// JournalDay is the journal note of one day.
type JournalDay struct {
	domain.Document
	Date    string `json:"date"`    // YYYY-MM-DD
	Version string `json:"version"` // changes whenever the file does; SaveJournal needs it
}

// Journal returns the journal note of day, creating it when missing.
func (s *Service) Journal(ctx context.Context, day time.Time) (*JournalDay, error) {
	rel, _, err := s.Vault.Journal(day)
	if err != nil {
		return nil, err
	}
	// Sync also picks up a note that was just created or changed on disk.
	if _, err := s.Sync(ctx, false); err != nil {
		return nil, err
	}
	doc, err := s.ReadNote(ctx, rel)
	if err != nil {
		return nil, err
	}
	return &JournalDay{Document: *doc, Date: day.Format("2006-01-02"), Version: version(doc.Content)}, nil
}

// SaveJournal replaces the content of day's journal note. version is the
// Version the caller read; when the file has changed since, nothing is written
// and the error wraps ErrConflict.
func (s *Service) SaveJournal(ctx context.Context, day time.Time, content, ver string) (*JournalDay, error) {
	rel := "journal/" + day.Format("2006-01-02") + ".md"
	cur, err := os.ReadFile(s.Vault.Abs(rel))
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%s no longer exists: %w", rel, ErrConflict)
	}
	if err != nil {
		return nil, err
	}
	if version(string(cur)) != ver {
		return nil, fmt.Errorf("%s was %w; reload it and edit again", rel, ErrConflict)
	}
	if err := os.WriteFile(s.Vault.Abs(rel), []byte(content), 0o644); err != nil {
		return nil, err
	}
	return s.Journal(ctx, day)
}

// version identifies file content, so a save can tell whether it changed.
func version(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:8])
}
