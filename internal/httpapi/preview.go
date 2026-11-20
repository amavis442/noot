package httpapi

import (
	"net/http"
)

// previewRequest is the content to render. Path is where the note lives, so
// relative links resolve as they will once it is saved; empty means the vault
// root.
type previewRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// preview renders unsaved content the way a saved note is rendered. It reads
// and writes nothing; it only looks up which links name a file.
type previewHandler struct{ render renderer }

func (h *previewHandler) preview(w http.ResponseWriter, r *http.Request) {
	var in previewRequest
	if !decodeJSON(w, r, &in) {
		return
	}
	html, err := h.render.html(r.Context(), in.Path, []byte(in.Content))
	if err != nil {
		// A broken frontmatter block while typing is expected, not a server fault.
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"html": html})
}
