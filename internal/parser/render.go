package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/amavis442/noot/internal/domain"
)

// RenderOptions says where the links of a rendered note point to.
type RenderOptions struct {
	// NotePrefix is put before the escaped target of a link to a note.
	NotePrefix string
	// FilePrefix is put before the escaped vault path of a link to a file that
	// is not a note (an image, a PDF). Empty leaves such links as they are.
	FilePrefix string
	// File tells whether a wikilink target names a file instead of a note, and
	// which one (vault-relative path). Nil means no wikilink does.
	File func(target string) (path string, ok bool)
}

// RenderHTML renders the body of the note at relPath as HTML, without its
// frontmatter. Wikilinks and relative Markdown links become links to
// linkPrefix + the escaped target, so a viewer can resolve them as notes. Raw
// HTML in the note is left out, so the result is safe to show in a page.
func RenderHTML(relPath string, content []byte, linkPrefix string) (string, error) {
	return Render(relPath, content, RenderOptions{NotePrefix: linkPrefix})
}

// Render is RenderHTML that also links to files: an embedded image
// (![[photo.png]]) becomes an <img>, any other link to a file a plain link.
func Render(relPath string, content []byte, opt RenderOptions) (string, error) {
	noteHref := func(target string) string { return opt.NotePrefix + url.PathEscape(target) }
	fileHref := func(p string) string {
		segs := strings.Split(p, "/")
		for i, s := range segs {
			segs[i] = url.PathEscape(s)
		}
		return opt.FilePrefix + strings.Join(segs, "/")
	}

	content, _, err := RewriteLinks(relPath, content, func(l domain.Link, _ string) (string, bool) {
		if l.Kind != domain.LinkMarkdown {
			return "", false
		}
		if !strings.EqualFold(path.Ext(l.Target), ".md") {
			if opt.FilePrefix == "" {
				return "", false
			}
			return fileHref(l.Target), true
		}
		return noteHref(l.Target), true
	})
	if err != nil {
		// RewriteLinks only fails on the frontmatter.
		return "", fmt.Errorf("frontmatter: %w", err)
	}
	_, body, err := splitFrontmatter(content)
	if err != nil {
		return "", fmt.Errorf("frontmatter: %w", err)
	}

	code := codeRanges(body)
	var src bytes.Buffer
	last := 0
	for _, m := range wikiRe.FindAllSubmatchIndex(body, -1) {
		inCode := slices.ContainsFunc(code, func(r [2]int) bool { return m[0] >= r[0] && m[0] < r[1] })
		inner := string(body[m[2]:m[3]])
		target := wikiTarget(inner)
		if inCode || target == "" {
			continue
		}
		label := strings.TrimSpace(inner)
		if i := strings.IndexByte(inner, '|'); i >= 0 {
			label = strings.TrimSpace(inner[i+1:])
		}

		// An embed (![[x]]) shows an image when the target is one; anything else
		// becomes a plain link, since an <img> of a note would only be broken.
		start, href := m[0], noteHref(target)
		embed := m[0] > 0 && body[m[0]-1] == '!'
		if embed {
			start--
		}
		image := false
		if opt.File != nil && opt.FilePrefix != "" {
			if p, ok := opt.File(target); ok {
				href, image = fileHref(p), embed && domain.IsImage(p)
			}
		}
		link := "[" + label + "](" + href + ")"
		if image {
			link = "!" + link
		}
		src.Write(body[last:start])
		src.WriteString(link)
		last = m[1]
	}
	src.Write(body[last:])

	var out bytes.Buffer
	if err := md.Convert(src.Bytes(), &out); err != nil {
		return "", err
	}
	return out.String(), nil
}
