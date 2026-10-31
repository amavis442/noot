package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/vault"
)

// AgendaWeek is the weekly agenda note of one ISO week.
type AgendaWeek struct {
	domain.Document
	Week   string      `json:"week"`   // ISO week, e.g. 2026-W45
	Monday string      `json:"monday"` // first day of the week, YYYY-MM-DD
	Intro  string      `json:"intro"`  // the text before the first day, frontmatter included
	Days   []AgendaDay `json:"days"`   // in file order; a day without its heading is left out
}

// AgendaDay is one day of a weekly agenda, to edit on its own.
type AgendaDay struct {
	Date    string `json:"date"`    // YYYY-MM-DD
	Heading string `json:"heading"` // e.g. "Monday 2 November"
	Content string `json:"content"` // the text under the heading
	Version string `json:"version"` // changes whenever the day's text does; SaveAgendaDay checks it
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
	monday := vault.MondayOf(day)
	intro, sections := vault.SplitAgenda(doc.Content, monday)
	days := make([]AgendaDay, 0, len(sections))
	for _, sec := range sections {
		days = append(days, agendaDay(sec))
	}
	return &AgendaWeek{
		Document: *doc,
		Week:     fmt.Sprintf("%d-W%02d", year, week),
		Monday:   monday.Format("2006-01-02"),
		Intro:    intro,
		Days:     days,
	}, nil
}

// SaveAgendaDay replaces the text of day in its week's agenda. ver is the
// Version of the day the caller read; when that day has changed since, nothing
// is written and the error wraps ErrConflict. Changes to other days do not
// matter: only this day is replaced in the current file.
func (s *Service) SaveAgendaDay(ctx context.Context, day time.Time, content, ver string) (*AgendaDay, error) {
	rel, _, err := s.Vault.Agenda(day)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.Vault.Abs(rel))
	if err != nil {
		return nil, err
	}
	monday := vault.MondayOf(day)
	cur, ok := findAgendaDay(string(b), monday, day)
	if !ok {
		return nil, fmt.Errorf("%s has no heading for %s: %w", rel, day.Format("Monday 2 January"), ErrNotFound)
	}
	if cur.Version != ver {
		return nil, fmt.Errorf("%s of %s was %w; reload it and edit again", cur.Heading, rel, ErrConflict)
	}
	updated, err := vault.SetAgendaDay(string(b), monday, day, content)
	if errors.Is(err, vault.ErrForeignDayHeading) {
		return nil, fmt.Errorf("%w: %w", ErrInvalidNote, err)
	}
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.Vault.Abs(rel), []byte(updated), 0o644); err != nil {
		return nil, err
	}
	if _, err := s.Sync(ctx, false); err != nil {
		return nil, err
	}
	saved, _ := findAgendaDay(updated, monday, day)
	return &saved, nil
}

// findAgendaDay returns day from the agenda content of the week starting monday.
func findAgendaDay(content string, monday, day time.Time) (AgendaDay, bool) {
	_, sections := vault.SplitAgenda(content, monday)
	for _, sec := range sections {
		if sec.Day.Equal(day) {
			return agendaDay(sec), true
		}
	}
	return AgendaDay{}, false
}

func agendaDay(sec vault.AgendaSection) AgendaDay {
	return AgendaDay{
		Date:    sec.Day.Format("2006-01-02"),
		Heading: sec.Heading,
		Content: sec.Body,
		Version: version([]byte(sec.Body)),
	}
}
