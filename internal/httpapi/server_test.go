package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
)

// fakeNotes records what the handlers ask for and answers with canned data.
type fakeNotes struct {
	query  domain.SearchQuery
	draft  app.NoteDraft
	docs   map[string]domain.Document
	hits   []domain.SearchHit
	create error
}

func (f *fakeNotes) Search(_ context.Context, q domain.SearchQuery) ([]domain.SearchHit, error) {
	f.query = q
	return f.hits, nil
}

func (f *fakeNotes) ReadNote(_ context.Context, ref string) (*domain.Document, error) {
	d, ok := f.docs[ref]
	if !ok {
		return nil, fmt.Errorf("lookup: %w", app.ErrNotFound)
	}
	return &d, nil
}

func (f *fakeNotes) CreateNote(_ context.Context, d app.NoteDraft, _ time.Time) (*domain.NoteRef, error) {
	f.draft = d
	if f.create != nil {
		return nil, f.create
	}
	return &domain.NoteRef{ID: "N1", Path: "zettels/x.md", Title: d.Title}, nil
}

func do(t *testing.T, h http.Handler, method, target, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, body %s", ct, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
}

func TestSearch(t *testing.T) {
	notes := &fakeNotes{hits: []domain.SearchHit{{NoteRef: domain.NoteRef{ID: "A", Path: "a.md", Title: "Alpha"}, Snippet: "…alpha…"}}}
	h := New(notes, fstest.MapFS{})

	rec := do(t, h, "GET", "/api/search?q=alp&tag=go&tag=web&limit=5", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var hits []domain.SearchHit
	decode(t, rec, &hits)
	if len(hits) != 1 || hits[0].Title != "Alpha" || hits[0].Snippet != "…alpha…" {
		t.Errorf("hits = %+v", hits)
	}
	if q := notes.query; q.Text != "alp" || strings.Join(q.Tags, ",") != "go,web" || q.Limit != 5 || q.Raw {
		t.Errorf("query = %+v", q)
	}

	notes.hits = nil
	rec = do(t, h, "GET", "/api/search?q=none", "", "")
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("no hits should be [], got %s", rec.Body)
	}
	if rec = do(t, h, "GET", "/api/search?limit=many", "", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad limit: status %d", rec.Code)
	}
}

func TestReadNote(t *testing.T) {
	notes := &fakeNotes{docs: map[string]domain.Document{
		"zettels/a.md": {NoteRef: domain.NoteRef{ID: "A", Path: "zettels/a.md", Title: "Alpha"}, Content: "---\nid: A\n---\n# Alpha\n\nSee [[Beta]].\n"},
	}}
	h := New(notes, fstest.MapFS{})

	rec := do(t, h, "GET", "/api/notes/zettels%2Fa.md", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var doc struct {
		ID, Path, Title, Content, HTML string
	}
	decode(t, rec, &doc)
	if doc.ID != "A" || doc.Path != "zettels/a.md" || !strings.Contains(doc.Content, "See [[Beta]].") {
		t.Errorf("doc = %+v", doc)
	}
	if !strings.Contains(doc.HTML, `<a href="#/note/Beta">Beta</a>`) {
		t.Errorf("html = %s", doc.HTML)
	}

	// An unescaped slash works too.
	if rec = do(t, h, "GET", "/api/notes/zettels/a.md", "", ""); rec.Code != http.StatusOK {
		t.Errorf("unescaped path: status %d", rec.Code)
	}

	rec = do(t, h, "GET", "/api/notes/missing", "", "")
	var e struct{ Error string }
	decode(t, rec, &e)
	if rec.Code != http.StatusNotFound || e.Error == "" {
		t.Errorf("missing: status %d, %+v", rec.Code, e)
	}
}

func TestCreateNote(t *testing.T) {
	notes := &fakeNotes{}
	h := New(notes, fstest.MapFS{})

	rec := do(t, h, "POST", "/api/notes", "application/json", `{"title":"Nieuw","body":"tekst","tags":["idee"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var ref domain.NoteRef
	decode(t, rec, &ref)
	if ref.ID != "N1" || ref.Title != "Nieuw" {
		t.Errorf("ref = %+v", ref)
	}
	if notes.draft.Title != "Nieuw" || notes.draft.Body != "tekst" || strings.Join(notes.draft.Tags, ",") != "idee" || notes.draft.Dir != "" {
		t.Errorf("draft = %+v", notes.draft)
	}

	// A browser form cannot send JSON cross-site without a preflight.
	if rec = do(t, h, "POST", "/api/notes", "application/x-www-form-urlencoded", "title=x"); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("form post: status %d", rec.Code)
	}
	if rec = do(t, h, "POST", "/api/notes", "application/json", `{"title":`); rec.Code != http.StatusBadRequest {
		t.Errorf("broken json: status %d", rec.Code)
	}
	if rec = do(t, h, "POST", "/api/notes", "application/json", `{"title":"x","dir":"../../etc"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown field dir: status %d", rec.Code)
	}

	notes.create = fmt.Errorf("%w: title is required", app.ErrInvalidNote)
	rec = do(t, h, "POST", "/api/notes", "application/json", `{"title":""}`)
	var e struct{ Error string }
	decode(t, rec, &e)
	if rec.Code != http.StatusBadRequest || !strings.Contains(e.Error, "title is required") {
		t.Errorf("invalid draft: status %d, %+v", rec.Code, e)
	}

	notes.create = fmt.Errorf("disk full")
	if rec = do(t, h, "POST", "/api/notes", "application/json", `{"title":"x"}`); rec.Code != http.StatusInternalServerError {
		t.Errorf("failure: status %d", rec.Code)
	}
}

func TestStaticFiles(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>noot</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	h := New(&fakeNotes{}, assets)

	if rec := do(t, h, "GET", "/", "", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>noot</title>") {
		t.Errorf("index: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, "GET", "/assets/app.js", "", ""); rec.Code != http.StatusOK {
		t.Errorf("asset: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/unknown", "", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown api route: %d", rec.Code)
	}
}

func TestStaticFilesWithoutBuild(t *testing.T) {
	h := New(&fakeNotes{}, fstest.MapFS{".gitkeep": {}})
	rec := do(t, h, "GET", "/", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "make web") {
		t.Errorf("placeholder: %d %s", rec.Code, rec.Body)
	}
}

func TestLoopbackOnly(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := LoopbackOnly(ok)
	for host, want := range map[string]int{
		"localhost:4242":    http.StatusNoContent,
		"127.0.0.1:4242":    http.StatusNoContent,
		"[::1]:4242":        http.StatusNoContent,
		"localhost":         http.StatusNoContent,
		"evil.example:4242": http.StatusForbidden,
		"192.168.1.5:4242":  http.StatusForbidden,
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %s: status %d, want %d", host, rec.Code, want)
		}
	}
}
