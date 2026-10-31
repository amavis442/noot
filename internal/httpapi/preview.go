package httpapi

import (
	"net/http"

	"github.com/amavis442/noot/internal/parser"
)

// previewRequest is the content to render. Path is where the note lives, so
// relative links resolve as they will once it is saved; empty means the vault
// root.
type previewRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// preview renders unsaved content the way a saved note is rendered. It reads
// and writes nothing, so it needs no port.
func preview(w http.ResponseWriter, r *http.Request) {
	var in previewRequest
	if !decodeJSON(w, r, &in) {
		return
	}
	html, err := parser.RenderHTML(in.Path, []byte(in.Content), noteLinkPrefix)
	if err != nil {
		// A broken frontmatter block while typing is expected, not a server fault.
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"html": html})
}
