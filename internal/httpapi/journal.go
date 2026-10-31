package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/vault"
)

// Journal is what the journal endpoints need from the app layer.
type Journal interface {
	Journal(ctx context.Context, day time.Time) (*app.JournalDay, error)
	SaveJournal(ctx context.Context, day time.Time, content, version string) (*app.JournalDay, error)
}

type journalHandler struct{ journal Journal }

// journalDay is a journal note as the API returns it.
type journalDay struct {
	note
	Date    string `json:"date"`
	Version string `json:"version"`
}

func (h *journalHandler) get(w http.ResponseWriter, r *http.Request) {
	day, ok := parseDay(w, r.PathValue("day"))
	if !ok {
		return
	}
	j, err := h.journal.Journal(r.Context(), day)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJournal(w, j)
}

// journalEdit is the request body for saving a journal note: the whole file
// and the version it was loaded with.
type journalEdit struct {
	Content string `json:"content"`
	Version string `json:"version"`
}

func (h *journalHandler) put(w http.ResponseWriter, r *http.Request) {
	day, ok := parseDay(w, r.PathValue("day"))
	if !ok {
		return
	}
	var e journalEdit
	if !decodeJSON(w, r, &e) {
		return
	}
	if e.Version == "" {
		writeError(w, http.StatusBadRequest, errors.New("version is required: send the version the note was loaded with"))
		return
	}
	j, err := h.journal.SaveJournal(r.Context(), day, e.Content, e.Version)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJournal(w, j)
}

func writeJournal(w http.ResponseWriter, j *app.JournalDay) {
	n, err := render(j.Document)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, journalDay{note: n, Date: j.Date, Version: j.Version})
}

// parseDay reads a day the way the command line does (YYYY-MM-DD, today,
// morgen, +2, friday …). On failure it writes a 400 and returns false.
func parseDay(w http.ResponseWriter, s string) (time.Time, bool) {
	day, err := vault.ParseDay(s, time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return time.Time{}, false
	}
	return day, true
}
