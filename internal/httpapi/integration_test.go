package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	srv := httptest.NewServer(httpapi.New(&app.Service{Vault: v, Store: st}, fstest.MapFS{}))
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
