package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/config"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/httpapi"
	"github.com/amavis442/noot/internal/storage/sqlite"
	"github.com/amavis442/noot/internal/vault"
)

// TestCreateThenSearchAndRead runs the API against a real vault and index.
func TestCreateThenSearchAndRead(t *testing.T) {
	v := &vault.Vault{Root: t.TempDir()}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	st, err := sqlite.Open(v.IndexFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc := &app.Service{Vault: v, Store: st}
	srv := httptest.NewServer(httpapi.New(httpapi.Ports{Notes: svc, Journal: svc, Reminders: svc}, fstest.MapFS{}))
	t.Cleanup(srv.Close)

	res, err := http.Post(srv.URL+"/api/notes", "application/json",
		strings.NewReader(`{"title":"Zuurdesem","body":"Voeden met rogge, zie [[Brood]].","tags":["bakken"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var ref domain.NoteRef
	if err := json.NewDecoder(res.Body).Decode(&ref); err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %+v %v", res.StatusCode, ref, err)
	}
	res.Body.Close()

	var hits []domain.SearchHit
	getJSON(t, srv.URL+"/api/search?q=rogge", &hits)
	if len(hits) != 1 || hits[0].ID != ref.ID {
		t.Fatalf("search = %+v", hits)
	}
	getJSON(t, srv.URL+"/api/search?tag=bakken", &hits)
	if len(hits) != 1 {
		t.Fatalf("search by tag = %+v", hits)
	}

	var doc struct{ Title, Content, HTML string }
	getJSON(t, srv.URL+"/api/notes/"+url.PathEscape(ref.Path), &doc)
	if doc.Title != "Zuurdesem" || !strings.Contains(doc.Content, "Voeden met rogge") ||
		!strings.Contains(doc.HTML, `<a href="#/note/Brood">Brood</a>`) {
		t.Errorf("note = %+v", doc)
	}
}

func getJSON(t *testing.T, u string, v any) {
	t.Helper()
	res, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d", u, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func newServer(t *testing.T) (*httptest.Server, *vault.Vault) {
	t.Helper()
	v := &vault.Vault{Root: t.TempDir()}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	st, err := sqlite.Open(v.IndexFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc := &app.Service{Vault: v, Store: st}
	srv := httptest.NewServer(httpapi.New(httpapi.Ports{Notes: svc, Journal: svc, Reminders: svc}, fstest.MapFS{}))
	t.Cleanup(srv.Close)
	return srv, v
}

func send(t *testing.T, method, u, body string, v any) int {
	t.Helper()
	req, err := http.NewRequest(method, u, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if v != nil {
		if err := json.NewDecoder(res.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	return res.StatusCode
}

// TestEditJournal loads a day, saves an edit and refuses a stale save.
func TestEditJournal(t *testing.T) {
	srv, v := newServer(t)

	var day struct{ Date, Path, Content, Version string }
	getJSON(t, srv.URL+"/api/journal/2026-10-31", &day)
	if day.Path != "journal/2026-10-31.md" || day.Version == "" {
		t.Fatalf("journal = %+v", day)
	}

	edit, _ := json.Marshal(map[string]string{"content": day.Content + "keyword-plover\n", "version": day.Version})
	var saved struct{ Version, HTML string }
	if code := send(t, "PUT", srv.URL+"/api/journal/2026-10-31", string(edit), &saved); code != http.StatusOK {
		t.Fatalf("save: %d", code)
	}
	var hits []domain.SearchHit
	getJSON(t, srv.URL+"/api/search?q=plover", &hits)
	if len(hits) != 1 || !strings.Contains(saved.HTML, "keyword-plover") {
		t.Errorf("after save: hits %+v, html %s", hits, saved.HTML)
	}

	// The first version is stale now.
	var e struct{ Error string }
	if code := send(t, "PUT", srv.URL+"/api/journal/2026-10-31", string(edit), &e); code != http.StatusConflict {
		t.Errorf("stale save: %d %+v", code, e)
	}
	b, _ := os.ReadFile(v.Abs(day.Path))
	if strings.Count(string(b), "keyword-plover") != 1 {
		t.Errorf("file after stale save:\n%s", b)
	}
}

// TestReminderLifecycle adds, plans, completes and lists reminders.
func TestReminderLifecycle(t *testing.T) {
	srv, v := newServer(t)

	var r struct {
		N               int
		Text, Due       string
		Done            bool
		Planned, DoneOn string
	}
	if code := send(t, "POST", srv.URL+"/api/reminders", `{"text":"Tandarts bellen","due":"2026-11-06"}`, &r); code != http.StatusCreated || r.N != 1 || r.Due != "2026-11-06" {
		t.Fatalf("add: %d %+v", code, r)
	}
	var p struct {
		Agenda string
	}
	if code := send(t, "POST", srv.URL+"/api/reminders/1/plan", `{"day":"2026-11-04"}`, &p); code != http.StatusOK || p.Agenda != "agenda/2026-W45.md" {
		t.Fatalf("plan: %d %+v", code, p)
	}
	agenda, _ := os.ReadFile(v.Abs(p.Agenda))
	if !strings.Contains(string(agenda), "- [ ] Tandarts bellen") {
		t.Errorf("agenda:\n%s", agenda)
	}
	if code := send(t, "POST", srv.URL+"/api/reminders/1/done", `{}`, &r); code != http.StatusOK || !r.Done {
		t.Fatalf("done: %d %+v", code, r)
	}
	agenda, _ = os.ReadFile(v.Abs(p.Agenda))
	if !strings.Contains(string(agenda), "- [x] Tandarts bellen") {
		t.Errorf("agenda not ticked:\n%s", agenda)
	}

	var open, all []struct{ N int }
	getJSON(t, srv.URL+"/api/reminders", &open)
	getJSON(t, srv.URL+"/api/reminders?all=true", &all)
	if len(open) != 0 || len(all) != 1 {
		t.Errorf("open %+v, all %+v", open, all)
	}
	if code := send(t, "POST", srv.URL+"/api/reminders/1/done", `{}`, nil); code != http.StatusBadRequest {
		t.Errorf("done twice: %d", code)
	}
	if code := send(t, "POST", srv.URL+"/api/reminders/7/plan", `{"day":"morgen"}`, nil); code != http.StatusNotFound {
		t.Errorf("plan unknown: %d", code)
	}
}
