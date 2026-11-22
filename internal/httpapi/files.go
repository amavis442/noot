package httpapi

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/parser"
)

// Files is what the file endpoint and the renderer need from the app layer:
// the files of the vault that notes link to (images, PDFs, ...).
type Files interface {
	// LinkFile returns the vault path of the file a wikilink target names; false
	// when the target names a note or nothing.
	LinkFile(ctx context.Context, target string) (string, bool, error)
	// FileLocation returns where the indexed file at the vault-relative path
	// rel is on disk.
	FileLocation(ctx context.Context, rel string) (string, error)
}

// fileLinkPrefix is where links to files inside a rendered note point to.
const fileLinkPrefix = "/api/files/"

// renderer turns Markdown into HTML with links to notes and, when it has a
// Files port, to files. The zero value renders links to notes only.
type renderer struct{ files Files }

func (rd renderer) html(ctx context.Context, relPath string, content []byte) (string, error) {
	opt := parser.RenderOptions{NotePrefix: noteLinkPrefix}
	if rd.files != nil {
		opt.FilePrefix = fileLinkPrefix
		opt.File = func(target string) (string, bool) {
			p, ok, err := rd.files.LinkFile(ctx, target)
			return p, err == nil && ok
		}
	}
	return parser.Render(relPath, content, opt)
}

type fileHandler struct{ files Files }

// serve sends a file of the vault. A page that shows a note must not be able
// to run what a note links to, so the file is only shown in the page when it is
// a raster image; everything else is a download with a neutral type, and
// nothing may be sniffed or scripted.
func (h *fileHandler) serve(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	abs, err := h.files.FileLocation(r.Context(), rel)
	if err != nil {
		writeAppError(w, err)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		writeError(w, http.StatusNotFound, errors.New("file not found"))
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, errors.New("file not found"))
		return
	}

	hd := w.Header()
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	if domain.IsImage(rel) {
		hd.Set("Content-Type", mime.TypeByExtension(path.Ext(rel)))
	} else {
		hd.Set("Content-Type", downloadType(rel))
		hd.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(rel)}))
	}
	http.ServeContent(w, r, path.Base(rel), fi.ModTime(), f)
}

// downloadType is the Content-Type of a file that is not shown in the page:
// what its extension says, unless a browser could run it.
func downloadType(rel string) string {
	t := mime.TypeByExtension(path.Ext(rel))
	switch mt, _, _ := mime.ParseMediaType(t); {
	case mt == "", strings.Contains(mt, "html"), strings.Contains(mt, "xml"), strings.Contains(mt, "javascript"), strings.Contains(mt, "svg"):
		return "application/octet-stream"
	}
	return t
}
