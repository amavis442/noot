package domain

import "time"

// TaskSource says where in the vault a task lives.
type TaskSource string

const (
	SourceReminders TaskSource = "reminders" // reminders.md
	SourceBacklog   TaskSource = "backlog"   // agenda/agenda.md, the undated backlog
	SourceWeek      TaskSource = "week"      // agenda/YYYY-Www.md, under Planned or Doing
)

// Task is one checkbox line from the vault, wherever it was written. noot only
// reads tasks and ticks them off; the line stays the one source of truth.
type Task struct {
	Source  TaskSource `json:"source"`
	File    string     `json:"file"`        // vault-relative, e.g. "reminders.md"
	Line    int        `json:"line"`        // 1-based line of the checkbox
	N       int        `json:"n,omitempty"` // reminder number; 0 for other sources
	Text    string     `json:"text"`        // without the checkbox and the " · key date" metadata
	Done    bool       `json:"done"`
	Due     string     `json:"due,omitempty"`     // YYYY-MM-DD
	Planned string     `json:"planned,omitempty"` // the agenda day, YYYY-MM-DD
	DoneOn  string     `json:"done_on,omitempty"` // YYYY-MM-DD
	Section string     `json:"section,omitempty"` // "Planned", "Doing" or "Done"; empty for reminders
}

// TaskGroup is the part of the overview a task is listed under.
type TaskGroup string

const (
	GroupOverdue  TaskGroup = "overdue"
	GroupToday    TaskGroup = "today"
	GroupThisWeek TaskGroup = "week"
	GroupLater    TaskGroup = "later"
	GroupBacklog  TaskGroup = "backlog"
	GroupDone     TaskGroup = "done"
)

// TaskGroups lists the groups in the order they are shown.
var TaskGroups = []TaskGroup{GroupOverdue, GroupToday, GroupThisWeek, GroupLater, GroupBacklog, GroupDone}

// Date is the earliest of the due and the planned date, or "" without either.
func (t Task) Date() string {
	switch {
	case t.Due == "":
		return t.Planned
	case t.Planned == "" || t.Due < t.Planned:
		return t.Due
	}
	return t.Planned
}

// Group says where t belongs on the day of now. Done tasks are always done;
// an open task goes by its earliest date: past is overdue, then today, the
// rest of the ISO week (up to Sunday), later, and without a date the backlog.
func (t Task) Group(now time.Time) TaskGroup {
	d := t.Date()
	if t.Done {
		return GroupDone
	}
	if d == "" {
		return GroupBacklog
	}
	y, m, day := now.Date()
	today := time.Date(y, m, day, 0, 0, 0, 0, now.Location())
	sunday := today.AddDate(0, 0, 6-(int(today.Weekday())+6)%7)
	switch {
	case d < today.Format("2006-01-02"):
		return GroupOverdue
	case d == today.Format("2006-01-02"):
		return GroupToday
	case d <= sunday.Format("2006-01-02"):
		return GroupThisWeek
	}
	return GroupLater
}
