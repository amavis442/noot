// Package httpapi is the HTTP adapter behind `noot serve`: a small JSON API
// over the app use cases plus the static files of the web frontend.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/parser"
)

// Notes is what the note endpoints need from the app layer.
type Notes interface {
	Search(ctx context.Context, q domain.SearchQuery) ([]domain.SearchHit, error)
	ReadNote(ctx context.Context, ref string) (*domain.Document, error)
	CreateNote(ctx context.Context, d app.NoteDraft, now time.Time) (*domain.NoteRef, error)
}

// Ports are the app use cases the API calls; *app.Service implements each.
type Ports struct {
	Notes     Notes
	Journal   Journal
	Reminders Reminders
}

// noteLinkPrefix is where links inside a rendered note point to: the
// frontend's route for showing a note.
const noteLinkPrefix = "#/note/"

// maxBody limits the size of a request body.
const maxBody = 1 << 20

// placeholder is served at / when the binary was built without the frontend.
const placeholder = `<!doctype html>
<meta charset="utf-8">
<title>noot</title>
<p>The noot web frontend is not built into this binary. Run <code>make web</code>
and rebuild noot. The JSON API under <code>/api/</code> works regardless.</p>
`

// New returns the handler for the API and the frontend files in assets.
func New(p Ports, assets fs.FS) http.Handler {
	h := &handler{notes: p.Notes}
	j := &journalHandler{journal: p.Journal}
	rh := &reminderHandler{reminders: p.Reminders}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", h.search)
	mux.HandleFunc("GET /api/notes/{ref...}", h.readNote)
	mux.HandleFunc("POST /api/notes", h.createNote)
	mux.HandleFunc("GET /api/journal/{day}", j.get)
	mux.HandleFunc("PUT /api/journal/{day}", j.put)
	mux.HandleFunc("GET /api/reminders", rh.list)
	mux.HandleFunc("POST /api/reminders", rh.add)
	mux.HandleFunc("POST /api/reminders/{n}/done", rh.done)
	mux.HandleFunc("POST /api/reminders/{n}/plan", rh.plan)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, errors.New("no such endpoint"))
	})
	mux.Handle("/", staticFiles(assets))
	return mux
}

type handler struct{ notes Notes }

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	q := domain.SearchQuery{Text: r.URL.Query().Get("q"), Tags: r.URL.Query()["tag"]}
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("limit must be a number"))
			return
		}
		q.Limit = n
	}
	hits, err := h.notes.Search(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if hits == nil {
		hits = []domain.SearchHit{}
	}
	writeJSON(w, http.StatusOK, hits)
}

// note is a note as the API returns it: the file content plus rendered HTML.
type note struct {
	domain.Document
	HTML string `json:"html"`
}

func (h *handler) readNote(w http.ResponseWriter, r *http.Request) {
	doc, err := h.notes.ReadNote(r.Context(), r.PathValue("ref"))
	if err != nil {
		writeAppError(w, err)
		return
	}
	n, err := render(*doc)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// render adds the HTML of a note's body.
func render(doc domain.Document) (note, error) {
	html, err := parser.RenderHTML(doc.Path, []byte(doc.Content), noteLinkPrefix)
	return note{Document: doc, HTML: html}, err
}

// draft is the request body for a new note. The folder is not part of it:
// notes made through the API always go to app.NewNoteDir.
type draft struct {
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

func (h *handler) createNote(w http.ResponseWriter, r *http.Request) {
	var d draft
	if !decodeJSON(w, r, &d) {
		return
	}
	ref, err := h.notes.CreateNote(r.Context(), app.NoteDraft{Title: d.Title, Body: d.Body, Tags: d.Tags}, time.Now())
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ref)
}

// decodeJSON reads the request body into v. On failure it writes the error
// response and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	// Requiring JSON keeps other sites out: a cross-site form cannot send it,
	// and a cross-site fetch that does needs a CORS preflight we never allow.
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, errors.New("send the request body as application/json"))
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

// writeAppError answers with the status that fits an error of the app layer.
func writeAppError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, app.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, app.ErrInvalidNote), errors.Is(err, app.ErrInvalidReminder):
		status = http.StatusBadRequest
	case errors.Is(err, app.ErrConflict):
		status = http.StatusConflict
	}
	writeError(w, status, err)
}

func staticFiles(assets fs.FS) http.Handler {
	if _, err := fs.Stat(assets, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(placeholder))
		})
	}
	return http.FileServerFS(assets)
}

// LoopbackOnly rejects requests whose Host header is not a loopback name. A
// server listening on localhost uses it against DNS rebinding: a foreign site
// whose name resolves to 127.0.0.1 still sends its own name as Host.
func LoopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			writeError(w, http.StatusForbidden, errors.New("only requests to localhost are served"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
