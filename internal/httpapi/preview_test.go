package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPreview(t *testing.T) {
	h := New(testPorts(), fstest.MapFS{})

	body := `{"path":"journal/2026-10-31.md","content":"---\ntitle: x\n---\n# Kop\n\nZie [[Thee]] en [bonen](../zettels/bonen.md). <script>x</script>"}`
	rec := do(t, h, "POST", "/api/preview", "application/json", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got struct{ HTML string }
	decode(t, rec, &got)
	for _, want := range []string{"<h1", `<a href="#/note/Thee">Thee</a>`, `<a href="#/note/zettels%2Fbonen.md">bonen</a>`} {
		if !strings.Contains(got.HTML, want) {
			t.Errorf("missing %s in %s", want, got.HTML)
		}
	}
	for _, not := range []string{"title: x", "<script>"} {
		if strings.Contains(got.HTML, not) {
			t.Errorf("unexpected %s in %s", not, got.HTML)
		}
	}

	// Without a path, relative links resolve from the vault root.
	rec = do(t, h, "POST", "/api/preview", "application/json", `{"content":"[a](b.md)"}`)
	decode(t, rec, &got)
	if rec.Code != http.StatusOK || !strings.Contains(got.HTML, `href="#/note/b.md"`) {
		t.Errorf("no path: %d %s", rec.Code, got.HTML)
	}

	// Half-typed frontmatter is the user's to fix, so it is a 400 with the reason.
	rec = do(t, h, "POST", "/api/preview", "application/json", `{"content":"---\ntitle: [oops\n---\n# x"}`)
	var e struct{ Error string }
	decode(t, rec, &e)
	if rec.Code != http.StatusBadRequest || !strings.Contains(e.Error, "frontmatter") {
		t.Errorf("broken frontmatter: %d %+v", rec.Code, e)
	}

	if rec := do(t, h, "POST", "/api/preview", "text/plain", "# x"); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text body: status %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/preview", "application/json", `{"content":"x","path":"../../etc/passwd"}`); rec.Code != http.StatusOK {
		t.Errorf("odd path: status %d; preview never touches the disk", rec.Code)
	}
}
