package vault

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RemindersFile holds every reminder as a Markdown task, open ones and done ones.
const RemindersFile = "reminders.md"

// Reminder is one task line of reminders.md:
//
//   - [ ] Call the dentist · due 2026-10-30 · planned 2026-10-28
//   - [x] Buy milk · done 2026-10-26
//
// N is its position among the task lines (1-based). Reminders are only ever
// appended and stay in the file when done, so the numbers do not shift.
type Reminder struct {
	N       int    `json:"n"`
	Text    string `json:"text"`
	Done    bool   `json:"done"`
	Due     string `json:"due,omitempty"`     // YYYY-MM-DD
	Planned string `json:"planned,omitempty"` // the agenda day, YYYY-MM-DD
	DoneOn  string `json:"done_on,omitempty"` // YYYY-MM-DD
}

var (
	reminderLine = regexp.MustCompile(`^- \[([ xX])\] (.*)$`)
	reminderMeta = regexp.MustCompile(`^(due|planned|done) (\d{4}-\d{2}-\d{2})$`)
)

// parseReminder reads a task line. Trailing " · key date" parts are metadata;
// anything else after a " · " stays part of the text.
func parseReminder(line string) (Reminder, bool) {
	m := reminderLine.FindStringSubmatch(line)
	if m == nil {
		return Reminder{}, false
	}
	r := Reminder{Done: m[1] != " ", Text: m[2]}
	for {
		i := strings.LastIndex(r.Text, " · ")
		if i < 0 {
			break
		}
		mm := reminderMeta.FindStringSubmatch(r.Text[i+len(" · "):])
		if mm == nil {
			break
		}
		switch mm[1] {
		case "due":
			r.Due = mm[2]
		case "planned":
			r.Planned = mm[2]
		case "done":
			r.DoneOn = mm[2]
		}
		r.Text = r.Text[:i]
	}
	return r, true
}

func (r Reminder) line() string {
	box := " "
	if r.Done {
		box = "x"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "- [%s] %s", box, r.Text)
	for _, m := range [][2]string{{"due", r.Due}, {"planned", r.Planned}, {"done", r.DoneOn}} {
		if m[1] != "" {
			fmt.Fprintf(&b, " · %s %s", m[0], m[1])
		}
	}
	return b.String()
}

// AddReminder appends a reminder to reminders.md (created when missing). A zero
// due means no due date.
func (v *Vault) AddReminder(text string, due, now time.Time) (Reminder, error) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return Reminder{}, fmt.Errorf("reminder is empty")
	}
	rs, err := v.Reminders()
	if err != nil {
		return Reminder{}, err
	}
	if !exists(v.Abs(RemindersFile)) {
		date := now.Format("2006-01-02")
		head := fmt.Sprintf("---\ntitle: Reminders\ncreated: %s\ntags: [reminders]\n---\n\n# Reminders\n\n", date)
		if err := v.write(RemindersFile, head); err != nil {
			return Reminder{}, err
		}
	}
	r := Reminder{N: len(rs) + 1, Text: text}
	if !due.IsZero() {
		r.Due = due.Format("2006-01-02")
	}
	return r, v.Append(RemindersFile, r.line())
}

// Reminders returns every reminder in file order; none when reminders.md is missing.
func (v *Vault) Reminders() ([]Reminder, error) {
	b, err := os.ReadFile(v.Abs(RemindersFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Reminder
	for _, line := range strings.Split(string(b), "\n") {
		if r, ok := parseReminder(strings.TrimRight(line, "\r")); ok {
			r.N = len(out) + 1
			out = append(out, r)
		}
	}
	return out, nil
}

// CompleteReminder marks reminder n done. When it was planned, its line in the
// agenda is ticked as well.
func (v *Vault) CompleteReminder(n int, now time.Time) (Reminder, error) {
	r, err := v.updateReminder(n, func(r *Reminder) error {
		if r.Done {
			return fmt.Errorf("reminder %d is already done", n)
		}
		r.Done, r.DoneOn = true, now.Format("2006-01-02")
		return nil
	})
	if err != nil || r.Planned == "" {
		return r, err
	}
	day, err := time.ParseInLocation("2006-01-02", r.Planned, now.Location())
	if err != nil {
		return r, nil
	}
	rel := AgendaPath(day)
	b, err := os.ReadFile(v.Abs(rel))
	if err != nil {
		return r, nil // the agenda is gone; nothing to tick
	}
	open := agendaItem(r.Text) + "\n"
	if s := string(b); strings.Contains(s, open) {
		err = os.WriteFile(v.Abs(rel), []byte(strings.Replace(s, open, "- [x]"+strings.TrimPrefix(open, "- [ ]"), 1)), 0o644)
	}
	return r, err
}

// PlanReminder puts reminder n under "### Planned" of day in that week's agenda
// (created when missing) and records the day on the reminder.
func (v *Vault) PlanReminder(n int, day time.Time) (Reminder, string, error) {
	rs, err := v.Reminders()
	if err != nil {
		return Reminder{}, "", err
	}
	if n < 1 || n > len(rs) {
		return Reminder{}, "", fmt.Errorf("no reminder %d", n)
	}
	if rs[n-1].Done {
		return Reminder{}, "", fmt.Errorf("reminder %d is already done", n)
	}
	rel, _, err := v.Agenda(day)
	if err != nil {
		return Reminder{}, "", err
	}
	b, err := os.ReadFile(v.Abs(rel))
	if err != nil {
		return Reminder{}, "", err
	}
	content, err := insertPlanned(string(b), day, agendaItem(rs[n-1].Text))
	if err != nil {
		return Reminder{}, "", err
	}
	if err := os.WriteFile(v.Abs(rel), []byte(content), 0o644); err != nil {
		return Reminder{}, "", err
	}
	r, err := v.updateReminder(n, func(r *Reminder) error {
		r.Planned = day.Format("2006-01-02")
		return nil
	})
	return r, rel, err
}

// agendaItem is how a reminder appears in the agenda; the link gives
// reminders.md a backlink from every week it was planned in.
func agendaItem(text string) string {
	return "- [ ] " + text + " ([[reminders]])"
}

// insertPlanned adds item at the end of the "### Planned" list of day, or at
// the end of the day when its Planned subheading is missing.
func insertPlanned(content string, day time.Time, item string) (string, error) {
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	heading := "## " + day.Format("Monday 2 January")
	text := func(i int) string { return strings.TrimRight(lines[i], "\r\n") }

	start := -1
	for i := range lines {
		if text(i) == heading {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("no %q in the agenda", heading)
	}
	end := len(lines) // end of the day: the next "## " heading
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(text(i), "## ") || text(i) == "##" {
			end = i
			break
		}
	}
	for i := start + 1; i < end; i++ {
		if text(i) == "### Planned" {
			start = i
			for j := i + 1; j < end; j++ { // the Planned list ends at the next heading
				if strings.HasPrefix(text(j), "#") {
					end = j
					break
				}
			}
			break
		}
	}

	at := end
	for at > start+1 && strings.TrimSpace(text(at-1)) == "" {
		at--
	}
	if !strings.HasSuffix(lines[at-1], "\n") {
		lines[at-1] += "\n"
	}
	insert := []string{item + "\n"}
	if at == start+1 { // right under the heading: keep a blank line between
		insert = []string{"\n", item + "\n"}
	}
	out := append(append(append([]string{}, lines[:at]...), insert...), lines[at:]...)
	return strings.Join(out, ""), nil
}

// updateReminder rewrites the line of reminder n after fn changed it.
func (v *Vault) updateReminder(n int, fn func(r *Reminder) error) (Reminder, error) {
	b, err := os.ReadFile(v.Abs(RemindersFile))
	if os.IsNotExist(err) {
		return Reminder{}, fmt.Errorf("no reminder %d", n)
	}
	if err != nil {
		return Reminder{}, err
	}
	lines := strings.SplitAfter(string(b), "\n")
	count := 0
	for i, line := range lines {
		body := strings.TrimRight(line, "\r\n")
		r, ok := parseReminder(body)
		if !ok {
			continue
		}
		if count++; count != n {
			continue
		}
		r.N = n
		if err := fn(&r); err != nil {
			return Reminder{}, err
		}
		lines[i] = r.line() + line[len(body):]
		return r, os.WriteFile(v.Abs(RemindersFile), []byte(strings.Join(lines, "")), 0o644)
	}
	return Reminder{}, fmt.Errorf("no reminder %d", n)
}

// dutchWeekdays are indexed by time.Weekday; the first two letters are the
// usual abbreviation (ma, di, wo …).
var dutchWeekdays = [7]string{"zondag", "maandag", "dinsdag", "woensdag", "donderdag", "vrijdag", "zaterdag"}

// ParseDay understands YYYY-MM-DD, today, tomorrow, +N or +Nd (days from now)
// and weekday names (monday or mon: the next one after today). The Dutch words
// work too: vandaag, morgen, vrijdag or vr.
func ParseDay(s string, now time.Time) (time.Time, error) {
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "today", "vandaag":
		return today, nil
	case "tomorrow", "morgen":
		return today.AddDate(0, 0, 1), nil
	}
	if n, ok := strings.CutPrefix(s, "+"); ok {
		days, err := strconv.Atoi(strings.TrimSuffix(n, "d"))
		if err != nil {
			return time.Time{}, fmt.Errorf("%q: use +N or +Nd for N days from today", s)
		}
		return today.AddDate(0, 0, days), nil
	}
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		name, nl := strings.ToLower(wd.String()), dutchWeekdays[wd]
		if (len(s) >= 3 && strings.HasPrefix(name, s)) || s == nl || s == nl[:2] {
			ahead := (int(wd) - int(today.Weekday()) + 7) % 7
			if ahead == 0 {
				ahead = 7
			}
			return today.AddDate(0, 0, ahead), nil
		}
	}
	t, err := time.ParseInLocation("2006-01-02", s, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not a day: use YYYY-MM-DD, today, tomorrow, +N or a weekday (also in Dutch)", s)
	}
	return t, nil
}
