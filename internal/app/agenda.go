package app

import (
	"context"
	"fmt"
	"time"

	"github.com/amavis442/noot/internal/domain"
)

// AgendaWeek is the weekly agenda note of one ISO week.
type AgendaWeek struct {
	domain.Document
	Week   string `json:"week"`   // ISO week, e.g. 2026-W45
	Monday string `json:"monday"` // first day of the week, YYYY-MM-DD
}

// Agenda returns the agenda note of the week day falls in, creating it when missing.
func (s *Service) Agenda(ctx context.Context, day time.Time) (*AgendaWeek, error) {
	rel, _, err := s.Vault.Agenda(day)
	if err != nil {
		return nil, err
	}
	doc, err := s.readFresh(ctx, rel)
	if err != nil {
		return nil, err
	}
	year, week := day.ISOWeek()
	y, m, d := day.Date()
	monday := time.Date(y, m, d, 0, 0, 0, 0, day.Location())
	monday = monday.AddDate(0, 0, -(int(monday.Weekday())+6)%7)
	return &AgendaWeek{Document: *doc, Week: fmt.Sprintf("%d-W%02d", year, week), Monday: monday.Format("2006-01-02")}, nil
}
