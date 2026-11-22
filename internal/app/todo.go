package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/vault"
)

// ErrInvalidTask is wrapped when a task cannot be changed as asked.
var ErrInvalidTask = errors.New("invalid task")

// TodoScope says how much of the overview Tasks returns.
type TodoScope int

const (
	TodoOpen  TodoScope = iota // every open task, with and without a date
	TodoToday                  // open tasks that are overdue or for today
	TodoWeek                   // as TodoToday, plus the rest of this week
	TodoAll                    // everything, done tasks last
)

// Tasks returns the tasks of the vault for the day of now, in the order they
// are shown: overdue, today, the rest of the week, later, backlog and done;
// within a group the earliest date first. noot keeps no copy: the files are
// read each time, as with Reminders.
func (s *Service) Tasks(scope TodoScope, now time.Time) ([]domain.Task, error) {
	all, err := s.Vault.Tasks()
	if err != nil {
		return nil, err
	}
	rank := func(t domain.Task) int { return slices.Index(domain.TaskGroups, t.Group(now)) }
	out := make([]domain.Task, 0, len(all))
	for _, t := range all {
		g := t.Group(now)
		switch {
		case scope == TodoAll:
		case g == domain.GroupDone:
			continue
		case scope == TodoToday && g != domain.GroupOverdue && g != domain.GroupToday:
			continue
		case scope == TodoWeek && g != domain.GroupOverdue && g != domain.GroupToday && g != domain.GroupThisWeek:
			continue
		}
		out = append(out, t)
	}
	slices.SortStableFunc(out, func(a, b domain.Task) int {
		if c := rank(a) - rank(b); c != 0 {
			return c
		}
		return strings.Compare(a.Date(), b.Date())
	})
	return out, nil
}

// CompleteTask ticks t off in its source file. It fails with
// vault.ErrTaskChanged when the line is no longer the task that was listed.
func (s *Service) CompleteTask(t domain.Task, now time.Time) error {
	if t.Done {
		return fmt.Errorf("%w: %q is already done", ErrInvalidTask, t.Text)
	}
	return s.Vault.CompleteTask(t, now)
}

// PlanTask puts a reminder under Planned of day in that week's agenda and
// returns the agenda's path. Only reminders can be planned; the other tasks
// already live in the agenda.
func (s *Service) PlanTask(t domain.Task, day time.Time) (string, error) {
	if t.Source != domain.SourceReminders {
		return "", fmt.Errorf("%w: only reminders can be planned, %q is in %s", ErrInvalidTask, t.Text, t.File)
	}
	rs, err := s.Vault.Reminders()
	if err != nil {
		return "", err
	}
	if t.N < 1 || t.N > len(rs) || rs[t.N-1].Text != t.Text || rs[t.N-1].Done {
		return "", fmt.Errorf("%w: reminder %d is no longer %q", vault.ErrTaskChanged, t.N, t.Text)
	}
	_, rel, err := s.Vault.PlanReminder(t.N, day)
	return rel, err
}
