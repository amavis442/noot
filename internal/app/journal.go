package app

import (
	"context"
	"time"

	"github.com/amavis442/noot/internal/domain"
)

// JournalDay is the journal note of one day.
type JournalDay struct {
	domain.Document
	Date string `json:"date"` // YYYY-MM-DD
}

// Journal returns the journal note of day, creating it when missing.
func (s *Service) Journal(ctx context.Context, day time.Time) (*JournalDay, error) {
	rel, _, err := s.Vault.Journal(day)
	if err != nil {
		return nil, err
	}
	doc, err := s.readFresh(ctx, rel)
	if err != nil {
		return nil, err
	}
	return &JournalDay{Document: *doc, Date: day.Format("2006-01-02")}, nil
}
