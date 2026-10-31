package app

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/amavis442/noot/internal/vault"
)

// ErrInvalidReminder is wrapped when a reminder cannot be added or changed as asked.
var ErrInvalidReminder = errors.New("invalid reminder")

// Reminders returns the open reminders, or all with done ones last when all is
// set. Within each group the earliest date (due or planned) comes first and
// undated reminders last.
func (s *Service) Reminders(all bool) ([]vault.Reminder, error) {
	rs, err := s.Vault.Reminders()
	if err != nil {
		return nil, err
	}
	if !all {
		rs = slices.DeleteFunc(rs, func(r vault.Reminder) bool { return r.Done })
	}
	key := func(r vault.Reminder) string {
		d := "9999"
		for _, s := range []string{r.Due, r.Planned} {
			if s != "" && s < d {
				d = s
			}
		}
		return strconv.FormatBool(r.Done) + d
	}
	slices.SortStableFunc(rs, func(a, b vault.Reminder) int { return strings.Compare(key(a), key(b)) })
	if rs == nil {
		rs = []vault.Reminder{}
	}
	return rs, nil
}

// AddReminder adds a reminder; a zero due means no due date.
func (s *Service) AddReminder(text string, due, now time.Time) (vault.Reminder, error) {
	if strings.TrimSpace(text) == "" {
		return vault.Reminder{}, fmt.Errorf("%w: the text is empty", ErrInvalidReminder)
	}
	return s.Vault.AddReminder(text, due, now)
}

// CompleteReminder marks reminder n done, and ticks it in the agenda when planned.
func (s *Service) CompleteReminder(n int, now time.Time) (vault.Reminder, error) {
	if _, err := s.openReminder(n); err != nil {
		return vault.Reminder{}, err
	}
	return s.Vault.CompleteReminder(n, now)
}

// PlanReminder puts reminder n under Planned of day in that week's agenda and
// returns the reminder and the agenda's path.
func (s *Service) PlanReminder(n int, day time.Time) (vault.Reminder, string, error) {
	if _, err := s.openReminder(n); err != nil {
		return vault.Reminder{}, "", err
	}
	return s.Vault.PlanReminder(n, day)
}

// openReminder returns reminder n, or an error when it is missing or done.
func (s *Service) openReminder(n int) (vault.Reminder, error) {
	rs, err := s.Vault.Reminders()
	if err != nil {
		return vault.Reminder{}, err
	}
	if n < 1 || n > len(rs) {
		return vault.Reminder{}, fmt.Errorf("no reminder %d: %w", n, ErrNotFound)
	}
	if rs[n-1].Done {
		return vault.Reminder{}, fmt.Errorf("%w: reminder %d is already done", ErrInvalidReminder, n)
	}
	return rs[n-1], nil
}
