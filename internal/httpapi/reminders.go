package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/amavis442/noot/internal/vault"
)

// Reminders is what the reminder endpoints need from the app layer.
type Reminders interface {
	Reminders(all bool) ([]vault.Reminder, error)
	AddReminder(text string, due, now time.Time) (vault.Reminder, error)
	CompleteReminder(n int, now time.Time) (vault.Reminder, error)
	PlanReminder(n int, day time.Time) (vault.Reminder, string, error)
}

type reminderHandler struct{ reminders Reminders }

func (h *reminderHandler) list(w http.ResponseWriter, r *http.Request) {
	all := false
	if s := r.URL.Query().Get("all"); s != "" {
		var err error
		if all, err = strconv.ParseBool(s); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("all must be true or false"))
			return
		}
	}
	rs, err := h.reminders.Reminders(all)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rs)
}

// newReminder is the request body for adding a reminder; due is optional.
type newReminder struct {
	Text string `json:"text"`
	Due  string `json:"due"`
}

func (h *reminderHandler) add(w http.ResponseWriter, r *http.Request) {
	var in newReminder
	if !decodeJSON(w, r, &in) {
		return
	}
	var due time.Time
	if in.Due != "" {
		var ok bool
		if due, ok = parseDay(w, in.Due); !ok {
			return
		}
	}
	rem, err := h.reminders.AddReminder(in.Text, due, time.Now())
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rem)
}

func (h *reminderHandler) done(w http.ResponseWriter, r *http.Request) {
	n, ok := reminderNumber(w, r)
	if !ok || !decodeJSON(w, r, &struct{}{}) {
		return
	}
	rem, err := h.reminders.CompleteReminder(n, time.Now())
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rem)
}

// planRequest is the request body for planning a reminder in the agenda.
type planRequest struct {
	Day string `json:"day"`
}

// planned is the answer to planning: the reminder and the agenda note it went to.
type planned struct {
	Reminder vault.Reminder `json:"reminder"`
	Agenda   string         `json:"agenda"`
}

func (h *reminderHandler) plan(w http.ResponseWriter, r *http.Request) {
	n, ok := reminderNumber(w, r)
	if !ok {
		return
	}
	var in planRequest
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Day == "" {
		writeError(w, http.StatusBadRequest, errors.New("day is required"))
		return
	}
	day, ok := parseDay(w, in.Day)
	if !ok {
		return
	}
	rem, agenda, err := h.reminders.PlanReminder(n, day)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planned{Reminder: rem, Agenda: agenda})
}

// reminderNumber reads {n} from the path. On failure it writes a 400 and returns false.
func reminderNumber(w http.ResponseWriter, r *http.Request) (int, bool) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("%q is not a reminder number", r.PathValue("n")))
		return 0, false
	}
	return n, true
}
