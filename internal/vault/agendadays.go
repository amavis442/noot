package vault

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNoAgendaDay is wrapped by SetAgendaDay when the agenda lacks the heading
// of the day.
var ErrNoAgendaDay = errors.New("day not in the agenda")

// ErrForeignDayHeading is wrapped by SetAgendaDay when the new text holds the
// heading of another day of the week, which would move that day into this one.
var ErrForeignDayHeading = errors.New("text holds the heading of another day")

// AgendaSection is one day of a weekly agenda: the text under its
// "## Monday 2 November" heading, up to the next day heading of the week.
type AgendaSection struct {
	Day     time.Time
	Heading string // e.g. "Monday 2 November"
	Body    string // without the blank lines around it, line endings as \n
}

// dayLine is a day heading found in an agenda: its line and its day.
type dayLine struct {
	line    int
	day     time.Time
	heading string
}

// findDays returns the day headings of the week starting monday, in file
// order. Days whose heading is missing are left out.
func findDays(lines []string, monday time.Time) []dayLine {
	byText := map[string]time.Time{}
	for i := range 7 {
		d := monday.AddDate(0, 0, i)
		byText[d.Format("Monday 2 January")] = d
	}
	var out []dayLine
	for i, l := range lines {
		h, ok := strings.CutPrefix(strings.TrimRight(l, "\r\n"), "## ")
		if d, known := byText[h]; ok && known {
			out = append(out, dayLine{i, d, h})
			delete(byText, h) // a repeated heading stays part of the day before
		}
	}
	return out
}

// SplitAgenda splits a weekly agenda (the week starting monday) into the text
// before the first day, frontmatter included, and its days in file order.
// Text after a day belongs to that day, also under other "##" headings.
func SplitAgenda(content string, monday time.Time) (intro string, days []AgendaSection) {
	lines := strings.SplitAfter(content, "\n")
	found := findDays(lines, monday)
	if len(found) == 0 {
		return content, nil
	}
	intro = strings.Join(lines[:found[0].line], "")
	for k, f := range found {
		end := len(lines)
		if k+1 < len(found) {
			end = found[k+1].line
		}
		days = append(days, AgendaSection{
			Day:     f.day,
			Heading: f.heading,
			Body:    trimBlankLines(strings.Join(lines[f.line+1:end], "")),
		})
	}
	return intro, days
}

// SetAgendaDay replaces the text of day in a weekly agenda (the week starting
// monday) with body and returns the new content; the other days stay as they
// are.
func SetAgendaDay(content string, monday, day time.Time, body string) (string, error) {
	lines := strings.SplitAfter(content, "\n")
	found := findDays(lines, monday)
	body = trimBlankLines(body)

	k := -1
	heading := day.Format("Monday 2 January")
	for i, f := range found {
		if f.heading == heading {
			k = i
		}
	}
	if k < 0 {
		return "", fmt.Errorf("no %q in the agenda: %w", "## "+heading, ErrNoAgendaDay)
	}
	for i := range 7 {
		other := monday.AddDate(0, 0, i).Format("Monday 2 January")
		for _, l := range strings.Split(body, "\n") {
			if other != heading && l == "## "+other {
				return "", fmt.Errorf("%q: %w; edit that day on its own", l, ErrForeignDayHeading)
			}
		}
	}

	end := len(lines)
	if k+1 < len(found) {
		end = found[k+1].line
	}
	head := strings.Join(lines[:found[k].line+1], "")
	if !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	text := "\n"
	if body != "" {
		text = "\n" + body + "\n\n"
	}
	return head + text + strings.Join(lines[end:], ""), nil
}

// trimBlankLines drops the blank lines at the start and the end of s and
// turns \r\n into \n.
func trimBlankLines(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
