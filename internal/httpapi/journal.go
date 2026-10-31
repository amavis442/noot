package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/parser"
	"github.com/amavis442/noot/internal/vault"
)

// Journal is what the journal endpoint needs from the app layer.
type Journal interface {
	Journal(ctx context.Context, day time.Time) (*app.JournalDay, error)
}

// Agenda is what the agenda endpoint needs from the app layer.
type Agenda interface {
	Agenda(ctx context.Context, day time.Time) (*app.AgendaWeek, error)
	SaveAgendaDay(ctx context.Context, day time.Time, content, version string) (*app.AgendaDay, error)
}

// Journal and agenda notes are saved like any other note, through PUT
// /api/notes/{ref}; these endpoints find (or create) the note of a day.
type dayHandler struct {
	journal Journal
	agenda  Agenda
}

// journalDay is a journal note as the API returns it.
type journalDay struct {
	note
	Date string `json:"date"`
}

// agendaWeek is an agenda note as the API returns it.
type agendaWeek struct {
	note
	Week      string      `json:"week"`
	Monday    string      `json:"monday"`
	IntroHTML string      `json:"intro_html"`
	Days      []agendaDay `json:"days"`
}

// agendaDay is one day of an agenda as the API returns it.
type agendaDay struct {
	app.AgendaDay
	HTML string `json:"html"`
}

func (h *dayHandler) getJournal(w http.ResponseWriter, r *http.Request) {
	day, ok := parseDay(w, r.PathValue("day"))
	if !ok {
		return
	}
	j, err := h.journal.Journal(r.Context(), day)
	if err != nil {
		writeAppError(w, err)
		return
	}
	n, err := render(j.Document)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, journalDay{note: n, Date: j.Date})
}

func (h *dayHandler) getAgenda(w http.ResponseWriter, r *http.Request) {
	day, ok := parseDay(w, r.PathValue("day"))
	if !ok {
		return
	}
	a, err := h.agenda.Agenda(r.Context(), day)
	if err != nil {
		writeAppError(w, err)
		return
	}
	n, err := render(a.Document)
	if err != nil {
		writeAppError(w, err)
		return
	}
	week := agendaWeek{note: n, Week: a.Week, Monday: a.Monday, Days: []agendaDay{}}
	if week.IntroHTML, err = parser.RenderHTML(a.Path, []byte(a.Intro), noteLinkPrefix); err != nil {
		writeAppError(w, err)
		return
	}
	for _, d := range a.Days {
		day, err := renderDay(a.Path, d)
		if err != nil {
			writeAppError(w, err)
			return
		}
		week.Days = append(week.Days, day)
	}
	writeJSON(w, http.StatusOK, week)
}

func (h *dayHandler) putAgendaDay(w http.ResponseWriter, r *http.Request) {
	day, ok := parseDay(w, r.PathValue("day"))
	if !ok {
		return
	}
	var e edit
	if !decodeJSON(w, r, &e) {
		return
	}
	if e.Version == "" {
		writeError(w, http.StatusBadRequest, errors.New("version is required: send the version the day was loaded with"))
		return
	}
	saved, err := h.agenda.SaveAgendaDay(r.Context(), day, e.Content, e.Version)
	if err != nil {
		writeAppError(w, err)
		return
	}
	d, err := renderDay(vault.AgendaPath(day), *saved)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// renderDay adds the HTML of a day; rel is the agenda note it is part of.
func renderDay(rel string, d app.AgendaDay) (agendaDay, error) {
	html, err := parser.RenderHTML(rel, []byte(d.Content), noteLinkPrefix)
	return agendaDay{AgendaDay: d, HTML: html}, err
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
