package vault

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amavis442/noot/internal/domain"
)

// BacklogFile is the agenda of things without a day yet: checkbox lines under
// "## Planned", "## Doing" and "## Done".
const BacklogFile = "agenda/agenda.md"

// plannedLink marks a week line that was put there by PlanReminder. The
// reminder in reminders.md is the task, so the line is not one of its own.
const plannedLink = "([[reminders]])"

var weekFileName = regexp.MustCompile(`^(\d{4})-W(\d{2})\.md$`)

// Tasks returns every checkbox task of the vault, in this order: reminders.md,
// agenda/agenda.md, then the weekly agendas oldest first. A missing file adds
// nothing. Only checkbox lines count, not plain bullets; in a week only those
// under "### Planned" and "### Doing", and not the ones PlanReminder wrote.
func (v *Vault) Tasks() ([]domain.Task, error) {
	var out []domain.Task

	lines, err := v.fileLines(RemindersFile)
	if err != nil {
		return nil, err
	}
	n := 0
	for i, line := range lines {
		if r, ok := parseReminder(line); ok {
			n++
			out = append(out, reminderTask(r, RemindersFile, i+1, n))
		}
	}

	lines, err = v.fileLines(BacklogFile)
	if err != nil {
		return nil, err
	}
	section := ""
	for i, line := range lines {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			section = strings.TrimSpace(h)
			continue
		}
		if r, ok := parseReminder(line); ok {
			t := reminderTask(r, BacklogFile, i+1, 0)
			t.Source, t.Section = domain.SourceBacklog, section
			out = append(out, t)
		}
	}

	entries, err := os.ReadDir(v.Abs("agenda"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		monday, ok := weekMonday(e.Name())
		if !ok || e.IsDir() {
			continue
		}
		rel := "agenda/" + e.Name()
		b, err := os.ReadFile(v.Abs(rel))
		if err != nil {
			return nil, err
		}
		out = append(out, weekTasks(string(b), monday, rel)...)
	}
	return out, nil
}

// reminderTask turns a parsed checkbox line into a task.
func reminderTask(r Reminder, file string, line, n int) domain.Task {
	return domain.Task{
		Source: domain.SourceReminders, File: file, Line: line, N: n,
		Text: r.Text, Done: r.Done, Due: r.Due, Planned: r.Planned, DoneOn: r.DoneOn,
	}
}

// weekTasks reads the open and ticked tasks under Planned and Doing of every
// day of one weekly agenda. A task's Planned is the day it stands under.
func weekTasks(content string, monday time.Time, rel string) []domain.Task {
	lines := strings.SplitAfter(content, "\n")
	days := findDays(lines, monday)
	var out []domain.Task
	for k, d := range days {
		end := len(lines)
		if k+1 < len(days) {
			end = days[k+1].line
		}
		section := ""
		for i := d.line + 1; i < end; i++ {
			line := strings.TrimRight(lines[i], "\r\n")
			if strings.HasPrefix(line, "#") {
				section = strings.TrimSpace(strings.TrimLeft(line, "#"))
				continue
			}
			if section != "Planned" && section != "Doing" {
				continue
			}
			r, ok := parseReminder(line)
			if !ok || strings.Contains(r.Text, plannedLink) {
				continue
			}
			t := reminderTask(r, rel, i+1, 0)
			t.Source, t.Section = domain.SourceWeek, section
			if t.Planned == "" {
				t.Planned = d.day.Format("2006-01-02")
			}
			out = append(out, t)
		}
	}
	return out
}

// weekMonday returns the Monday of the ISO week a file name like
// "2026-W47.md" stands for; false for any other name or a week that does not exist.
func weekMonday(name string) (time.Time, bool) {
	m := weekFileName.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, false
	}
	year, _ := strconv.Atoi(m[1])
	week, _ := strconv.Atoi(m[2])
	// 4 January is always in week 1.
	monday := MondayOf(time.Date(year, time.January, 4, 0, 0, 0, 0, time.Local)).AddDate(0, 0, 7*(week-1))
	if y, w := monday.ISOWeek(); y != year || w != week {
		return time.Time{}, false
	}
	return monday, true
}

// fileLines returns the lines of rel without line endings; none when it is missing.
func (v *Vault) fileLines(rel string) ([]string, error) {
	b, err := os.ReadFile(v.Abs(rel))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	lines := strings.Split(string(b), "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines, nil
}
