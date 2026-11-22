package vault

import (
	"errors"
	"fmt"
	"os"
	"path"
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

// ErrTaskChanged is wrapped by CompleteTask when the line is no longer the one
// the task was read from, because the file changed since it was listed.
var ErrTaskChanged = errors.New("task changed on disk")

// CompleteTask ticks t off in its file, after checking that the line still
// holds the same open task (file, line and text). Where the tick goes depends
// on the source:
//
//   - reminders.md: like CompleteReminder; the line stays and gets "· done <day>",
//     and a planned reminder is ticked in its week as well.
//   - agenda/agenda.md: the item gets "· done <day>" and moves, with its
//     indented continuation lines, to the end of "## Done" (created when missing).
//   - a weekly agenda: the checkbox is ticked in place.
func (v *Vault) CompleteTask(t domain.Task, now time.Time) error {
	if t.Done {
		return fmt.Errorf("%q is already done", t.Text)
	}
	if !taskFile(t) {
		return fmt.Errorf("%q is not a file with tasks", t.File)
	}
	b, err := os.ReadFile(v.Abs(t.File))
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrTaskChanged, t.File, err)
	}
	lines := strings.SplitAfter(string(b), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	i := t.Line - 1
	if i < 0 || i >= len(lines) {
		return fmt.Errorf("%w: %s has no line %d", ErrTaskChanged, t.File, t.Line)
	}
	body := strings.TrimRight(lines[i], "\r\n")
	r, ok := parseReminder(body)
	if !ok || r.Done || r.Text != t.Text {
		return fmt.Errorf("%w: line %d of %s is no longer %q", ErrTaskChanged, t.Line, t.File, t.Text)
	}

	switch t.Source {
	case domain.SourceReminders:
		n := 0
		for _, l := range lines[:i+1] {
			if _, ok := parseReminder(strings.TrimRight(l, "\r\n")); ok {
				n++
			}
		}
		_, err := v.CompleteReminder(n, now)
		return err
	case domain.SourceBacklog:
		return v.write(t.File, completeBacklog(lines, i, now))
	}
	lines[i] = "- [x]" + strings.TrimPrefix(body, "- [ ]") + lines[i][len(body):]
	return v.write(t.File, strings.Join(lines, ""))
}

// taskFile reports whether the task's file is one that Tasks reads, for its source.
func taskFile(t domain.Task) bool {
	switch t.Source {
	case domain.SourceReminders:
		return t.File == RemindersFile
	case domain.SourceBacklog:
		return t.File == BacklogFile
	case domain.SourceWeek:
		dir, name := path.Split(t.File)
		_, ok := weekMonday(name)
		return dir == "agenda/" && ok
	}
	return false
}

// completeBacklog ticks the item at lines[i] with today's date and moves it,
// with its continuation lines, to the end of "## Done". An item that already
// stands under "## Done" is ticked in place.
func completeBacklog(lines []string, i int, now time.Time) string {
	body := strings.TrimRight(lines[i], "\r\n")
	eol := lines[i][len(body):]
	if eol == "" {
		eol = "\n"
	}
	r, _ := parseReminder(body)
	r.Done, r.DoneOn = true, now.Format("2006-01-02")
	item := []string{r.line() + eol}
	end := i + 1
	for ; end < len(lines); end++ {
		l := lines[end]
		if !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") || strings.TrimSpace(l) == "" {
			break
		}
		item = append(item, l)
	}

	inDone := false
	for j := i - 1; j >= 0; j-- {
		if h, ok := strings.CutPrefix(strings.TrimRight(lines[j], "\r\n"), "## "); ok {
			inDone = strings.TrimSpace(h) == "Done"
			break
		}
	}
	if inDone {
		return strings.Join(append(append(append([]string{}, lines[:i]...), item[0]), lines[i+1:]...), "")
	}

	rest := append(append([]string{}, lines[:i]...), lines[end:]...)
	done := -1
	for j, l := range rest {
		if strings.TrimRight(l, "\r\n") == "## Done" {
			done = j
			break
		}
	}
	if done < 0 {
		if n := len(rest); n > 0 && !strings.HasSuffix(rest[n-1], "\n") {
			rest[n-1] += eol
		}
		return strings.Join(append(append(rest, eol, "## Done"+eol, eol), item...), "")
	}
	at := len(rest) // the end of the section: the next "## " heading or the end of the file
	for j := done + 1; j < len(rest); j++ {
		if strings.HasPrefix(rest[j], "## ") {
			at = j
			break
		}
	}
	for at > done+1 && strings.TrimSpace(rest[at-1]) == "" {
		at--
	}
	insert := item
	if at == done+1 { // right under the heading: keep a blank line between
		insert = append([]string{eol}, item...)
	}
	if !strings.HasSuffix(rest[at-1], "\n") {
		rest[at-1] += eol
	}
	return strings.Join(append(append(append([]string{}, rest[:at]...), insert...), rest[at:]...), "")
}
