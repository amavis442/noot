package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/vault"
)

type fakeReminders struct {
	all       bool
	text      string
	due, day  time.Time
	n         int
	completed error
}

func (f *fakeReminders) Reminders(all bool) ([]vault.Reminder, error) {
	f.all = all
	return []vault.Reminder{{N: 2, Text: "Tandarts", Due: "2026-10-30"}}, nil
}

func (f *fakeReminders) AddReminder(text string, due, _ time.Time) (vault.Reminder, error) {
	f.text, f.due = text, due
	if strings.TrimSpace(text) == "" {
		return vault.Reminder{}, fmt.Errorf("%w: the text is empty", app.ErrInvalidReminder)
	}
	return vault.Reminder{N: 3, Text: text}, nil
}

func (f *fakeReminders) CompleteReminder(n int, _ time.Time) (vault.Reminder, error) {
	f.n = n
	if f.completed != nil {
		return vault.Reminder{}, f.completed
	}
	return vault.Reminder{N: n, Text: "Tandarts", Done: true}, nil
}

func (f *fakeReminders) PlanReminder(n int, day time.Time) (vault.Reminder, string, error) {
	f.n, f.day = n, day
	return vault.Reminder{N: n, Text: "Tandarts", Planned: day.Format("2006-01-02")}, "agenda/2026-W45.md", nil
}

func remindersAPI(r *fakeReminders) http.Handler {
	p := testPorts()
	p.Reminders = r
	return New(p, fstest.MapFS{})
}

func TestListReminders(t *testing.T) {
	r := &fakeReminders{}
	h := remindersAPI(r)

	rec := do(t, h, "GET", "/api/reminders", "", "")
	var rs []vault.Reminder
	decode(t, rec, &rs)
	if rec.Code != http.StatusOK || len(rs) != 1 || rs[0].N != 2 || r.all {
		t.Errorf("list: %d %+v all=%v", rec.Code, rs, r.all)
	}
	do(t, h, "GET", "/api/reminders?all=true", "", "")
	if !r.all {
		t.Error("all=true was not passed on")
	}
}

func TestAddReminder(t *testing.T) {
	r := &fakeReminders{}
	h := remindersAPI(r)

	rec := do(t, h, "POST", "/api/reminders", "application/json", `{"text":"Melk","due":"2026-11-04"}`)
	if rec.Code != http.StatusCreated || r.text != "Melk" || r.due.Format("2006-01-02") != "2026-11-04" {
		t.Errorf("add: %d %s, text %q due %s", rec.Code, rec.Body, r.text, r.due)
	}
	rec = do(t, h, "POST", "/api/reminders", "application/json", `{"text":"Zonder datum"}`)
	if rec.Code != http.StatusCreated || !r.due.IsZero() {
		t.Errorf("add without due: %d, due %s", rec.Code, r.due)
	}
	if rec := do(t, h, "POST", "/api/reminders", "application/json", `{"text":"x","due":"ooit"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad due: status %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/reminders", "application/json", `{"text":" "}`); rec.Code != http.StatusBadRequest {
		t.Errorf("empty text: status %d", rec.Code)
	}
}

func TestCompleteAndPlanReminder(t *testing.T) {
	r := &fakeReminders{}
	h := remindersAPI(r)

	rec := do(t, h, "POST", "/api/reminders/2/done", "application/json", `{}`)
	var done vault.Reminder
	decode(t, rec, &done)
	if rec.Code != http.StatusOK || r.n != 2 || !done.Done {
		t.Errorf("done: %d %+v", rec.Code, done)
	}

	rec = do(t, h, "POST", "/api/reminders/2/plan", "application/json", `{"day":"2026-11-02"}`)
	var planned struct {
		Reminder vault.Reminder
		Agenda   string
	}
	decode(t, rec, &planned)
	if rec.Code != http.StatusOK || r.day.Format("2006-01-02") != "2026-11-02" || planned.Agenda != "agenda/2026-W45.md" || planned.Reminder.Planned != "2026-11-02" {
		t.Errorf("plan: %d %+v", rec.Code, planned)
	}

	if rec := do(t, h, "POST", "/api/reminders/two/done", "application/json", `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad number: status %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/reminders/2/plan", "application/json", `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("plan without day: status %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/reminders/2/done", "", ""); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("done without JSON: status %d", rec.Code)
	}

	r.completed = fmt.Errorf("no reminder 9: %w", app.ErrNotFound)
	if rec := do(t, h, "POST", "/api/reminders/9/done", "application/json", `{}`); rec.Code != http.StatusNotFound {
		t.Errorf("unknown reminder: status %d", rec.Code)
	}
	r.completed = fmt.Errorf("%w: reminder 2 is already done", app.ErrInvalidReminder)
	if rec := do(t, h, "POST", "/api/reminders/2/done", "application/json", `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("done twice: status %d", rec.Code)
	}
}
