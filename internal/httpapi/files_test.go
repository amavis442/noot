package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/config"
	"github.com/amavis442/noot/internal/httpapi"
	"github.com/amavis442/noot/internal/storage/sqlite"
	"github.com/amavis442/noot/internal/vault"
)

// filesServer runs the API against a real vault holding notes and attachments.
func filesServer(t *testing.T, files map[string]string) (*httptest.Server, *vault.Vault) {
	t.Helper()
	v := &vault.Vault{Root: t.TempDir()}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		p := v.Abs(rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := sqlite.Open(v.IndexFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc := &app.Service{Vault: v, Store: st}
	if _, err := svc.Sync(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.New(httpapi.Ports{Notes: svc, Journal: svc, Agenda: svc, Reminders: svc, Files: svc}, fstest.MapFS{}))
	t.Cleanup(srv.Close)
	return srv, v
}

func fetch(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	// Redirects are not followed: the mux cleans ".." out of a path with a 301,
	// which is a refusal here, not a file.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func TestServeFiles(t *testing.T) {
	srv, v := filesServer(t, map[string]string{
		"notes/a.md":       "# A\n\nHoliday ![[photo.png|Holiday]], the [[scan.pdf]], ![[logo.svg]], [[Beta]] and [doc](../my%20doc.txt).\n",
		"notes/beta.md":    "# Beta\n",
		"assets/photo.png": "PNGDATA",
		"scan.pdf":         "%PDF-1.4 data",
		"logo.svg":         `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"page.html":        "<script>alert(1)</script>",
		"my doc.txt":       "plain text",
		"café.png":         "ACCENT",
		".secret.png":      "hidden",
	})
	secret := filepath.Join(t.TempDir(), "secret.txt")
	_ = os.WriteFile(secret, []byte("TOPSECRET"), 0o644)
	if err := os.Symlink(secret, v.Abs("link.png")); err != nil {
		t.Logf("no symlinks: %v", err)
	}

	// A note shows its files: images inline, everything else as a link.
	res, body := fetch(t, srv.URL+"/api/notes/notes%2Fa.md")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("note: %d %s", res.StatusCode, body)
	}
	var n struct{ HTML string }
	if err := json.Unmarshal([]byte(body), &n); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<img src="/api/files/assets/photo.png" alt="Holiday">`,
		`<a href="/api/files/scan.pdf">scan.pdf</a>`,
		`<a href="/api/files/logo.svg">logo.svg</a>`,
		`<a href="#/note/Beta">Beta</a>`,
		`<a href="/api/files/my%20doc.txt">doc</a>`,
	} {
		if !strings.Contains(n.HTML, want) {
			t.Errorf("note html: missing %s in\n%s", want, n.HTML)
		}
	}

	// Images are shown in the page; everything else is a download.
	res, body = fetch(t, srv.URL+"/api/files/assets/photo.png")
	if res.StatusCode != http.StatusOK || body != "PNGDATA" || res.Header.Get("Content-Type") != "image/png" ||
		strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("image: %d %q %v", res.StatusCode, body, res.Header)
	}
	for _, p := range []string{"scan.pdf", "logo.svg", "page.html", "my%20doc.txt"} {
		res, _ = fetch(t, srv.URL+"/api/files/"+p)
		if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") ||
			res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("%s: %d %v", p, res.StatusCode, res.Header)
		}
	}
	for _, p := range []string{"logo.svg", "page.html"} {
		if res, _ = fetch(t, srv.URL+"/api/files/"+p); res.Header.Get("Content-Type") != "application/octet-stream" {
			t.Errorf("%s is served as %q: a browser must not run it", p, res.Header.Get("Content-Type"))
		}
	}
	if res, body = fetch(t, srv.URL+"/api/files/caf%C3%A9.png"); res.StatusCode != http.StatusOK || body != "ACCENT" {
		t.Errorf("non-ascii name: %d %q", res.StatusCode, body)
	}

	// Only indexed files of the vault are served.
	for name, p := range map[string]string{
		"a note":         "notes/a.md",
		"unknown":        "nope.png",
		"by file name":   "photo.png", // the URL names a path; a name is for wikilinks
		"hidden":         ".secret.png",
		"the index":      ".noot/index.db",
		"the git config": ".git/config",
		"traversal":      "../../../../etc/passwd",
		"encoded":        "%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		"encoded slash":  "assets%2f..%2f..%2fetc%2fpasswd",
		"symlink":        "link.png",
		"a folder":       "assets",
		"empty":          "",
		"backslash":      "..%5c..%5cetc%5cpasswd",
	} {
		res, body = fetch(t, srv.URL+"/api/files/"+p)
		if res.StatusCode == http.StatusOK || strings.Contains(body, "TOPSECRET") || strings.Contains(body, "root:") {
			t.Errorf("%s: %d %q", name, res.StatusCode, body)
		}
	}

	// Writing is not possible.
	req, _ := http.NewRequest("PUT", srv.URL+"/api/files/assets/photo.png", strings.NewReader("x"))
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode == http.StatusOK {
		t.Errorf("PUT: %v %v", res, err)
	}
}

func TestRenderedNoteWithoutFilesPort(t *testing.T) {
	// A server built without the Files port still renders notes, as before.
	v := &vault.Vault{Root: t.TempDir()}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(v.Abs("a.md"), []byte("# A\n\n![[photo.png]]\n"), 0o644)
	st, err := sqlite.Open(v.IndexFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc := &app.Service{Vault: v, Store: st}
	_, _ = svc.Sync(t.Context(), false)
	srv := httptest.NewServer(httpapi.New(httpapi.Ports{Notes: svc, Journal: svc, Agenda: svc, Reminders: svc}, fstest.MapFS{}))
	t.Cleanup(srv.Close)

	res, body := fetch(t, srv.URL+"/api/notes/a.md")
	if res.StatusCode != http.StatusOK || !strings.Contains(body, `href=\"#/note/photo.png\"`) {
		t.Errorf("note: %d %s", res.StatusCode, body)
	}
	if res, _ = fetch(t, srv.URL+"/api/files/photo.png"); res.StatusCode != http.StatusNotFound {
		t.Errorf("files without port: %d", res.StatusCode)
	}
}
