package domain

import (
	"testing"
	"time"
)

func TestTaskDateIsTheEarliestOfDueAndPlanned(t *testing.T) {
	cases := []struct {
		task Task
		want string
	}{
		{Task{}, ""},
		{Task{Due: "2026-11-10"}, "2026-11-10"},
		{Task{Planned: "2026-11-09"}, "2026-11-09"},
		{Task{Due: "2026-11-10", Planned: "2026-11-09"}, "2026-11-09"},
		{Task{Due: "2026-11-08", Planned: "2026-11-09"}, "2026-11-08"},
	}
	for _, c := range cases {
		if got := c.task.Date(); got != c.want {
			t.Errorf("%+v.Date() = %q, want %q", c.task, got, c.want)
		}
	}
}

func TestTaskGroup(t *testing.T) {
	today := time.Date(2026, 11, 18, 14, 0, 0, 0, time.Local) // a Wednesday; its week ends on Sunday the 22nd
	cases := []struct {
		name string
		task Task
		want TaskGroup
	}{
		{"no date is backlog", Task{Text: "x"}, GroupBacklog},
		{"due yesterday", Task{Due: "2026-11-17"}, GroupOverdue},
		{"planned on a past day", Task{Planned: "2026-11-16"}, GroupOverdue},
		{"due today", Task{Due: "2026-11-18"}, GroupToday},
		{"planned today", Task{Planned: "2026-11-18"}, GroupToday},
		{"planned earlier than a future due date is overdue", Task{Planned: "2026-11-17", Due: "2026-12-01"}, GroupOverdue},
		{"tomorrow", Task{Planned: "2026-11-19"}, GroupThisWeek},
		{"sunday still counts", Task{Due: "2026-11-22"}, GroupThisWeek},
		{"next monday is later", Task{Due: "2026-11-23"}, GroupLater},
		{"done wins over everything", Task{Done: true, Due: "2026-11-01"}, GroupDone},
		{"done without date", Task{Done: true}, GroupDone},
	}
	for _, c := range cases {
		if got := c.task.Group(today); got != c.want {
			t.Errorf("%s: Group = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTaskGroupOnASundayEndsThatWeek(t *testing.T) {
	sunday := time.Date(2026, 11, 22, 9, 0, 0, 0, time.Local)
	if got := (Task{Due: "2026-11-22"}).Group(sunday); got != GroupToday {
		t.Errorf("today = %q", got)
	}
	if got := (Task{Due: "2026-11-23"}).Group(sunday); got != GroupLater {
		t.Errorf("tomorrow, a new week = %q", got)
	}
}
