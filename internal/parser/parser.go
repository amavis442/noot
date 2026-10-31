// Package parser turns a Markdown file into a domain.Note: frontmatter,
// title, tags, wikilinks and Markdown links.
package parser

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/amavis442/noot/internal/domain"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"go.yaml.in/yaml/v3"
)

var (
	wikiRe = regexp.MustCompile(`\[\[([^\[\]\n]+?)\]\]`)
	// #tag, #nested/tag, #dutch-ë; must start after whitespace/punctuation and contain a letter.
	tagRe = regexp.MustCompile(`(?:^|[\s(,;])#([\p{L}\p{N}_\-/]*\p{L}[\p{L}\p{N}_\-/]*)`)

	md = goldmark.New(goldmark.WithExtensions(extension.GFM))
)

const maxContext = 160

// Parse parses the raw file content. relPath is the vault-relative path with
// forward slashes ("zettels/foo.md").
func Parse(relPath string, content []byte) (domain.Note, error) {
	n := domain.Note{Path: relPath}

	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return n, fmt.Errorf("%s: frontmatter: %w", relPath, err)
	}
	n.Body = string(body)

	n.ID = strings.TrimSpace(fm.str("id"))
	if n.ID == "" {
		n.ID = strings.TrimSuffix(relPath, path.Ext(relPath))
	}
	n.Title = strings.TrimSpace(fm.str("title"))
	n.Created = strings.TrimSpace(fm.str("created"))
	n.Aliases = fm.list("aliases")
	tags := fm.list("tags")

	w := walker{src: body, dir: path.Dir(relPath)}
	if err := w.walk(md.Parser().Parse(text.NewReader(body))); err != nil {
		return n, fmt.Errorf("%s: %w", relPath, err)
	}
	if n.Title == "" {
		n.Title = w.h1
	}
	if n.Title == "" {
		n.Title = strings.TrimSuffix(path.Base(relPath), path.Ext(relPath))
	}
	n.Links = w.links
	n.Tags = normaliseTags(append(tags, w.tags...))
	return n, nil
}

// ---------- frontmatter ----------

type frontmatter map[string]any

// Body returns content without its frontmatter. Content whose frontmatter is
// not valid YAML is returned as it is, so nothing is hidden from the reader.
func Body(content []byte) []byte {
	_, body, err := splitFrontmatter(content)
	if err != nil {
		return content
	}
	return body
}

func splitFrontmatter(src []byte) (frontmatter, []byte, error) {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")) // UTF-8 BOM (Windows editors)
	nl := []byte("\n")
	if !bytes.HasPrefix(src, []byte("---\n")) && !bytes.HasPrefix(src, []byte("---\r\n")) {
		return frontmatter{}, src, nil
	}
	rest := src[bytes.Index(src, nl)+1:]
	for off := 0; off < len(rest); {
		end := bytes.Index(rest[off:], nl)
		line := rest[off:]
		if end >= 0 {
			line = rest[off : off+end]
		}
		if t := bytes.TrimRight(line, "\r"); bytes.Equal(t, []byte("---")) || bytes.Equal(t, []byte("...")) {
			fm := frontmatter{}
			if err := yaml.Unmarshal(rest[:off], &fm); err != nil {
				return nil, nil, err
			}
			if end < 0 {
				return fm, nil, nil
			}
			return fm, rest[off+end+1:], nil
		}
		if end < 0 {
			break
		}
		off += end + 1
	}
	// No closing fence: treat the whole file as body.
	return frontmatter{}, src, nil
}

func (f frontmatter) str(key string) string {
	switch v := f[key].(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v) // dates, numbers
	}
}

// list accepts both `tags: [a, b]` and `tags: a, b`.
func (f frontmatter) list(key string) []string {
	var out []string
	switch v := f[key].(type) {
	case string:
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	case []any:
		for _, s := range v {
			if s := strings.TrimSpace(fmt.Sprint(s)); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// ---------- AST walk ----------

type walker struct {
	src   []byte
	dir   string
	h1    string
	tags  []string
	links []domain.Link

	buf     strings.Builder // text of the current block, without code: used to find links and tags
	show    strings.Builder // same block including `code` and autolinks: used for titles and context
	inBlock int
	pending []domain.Link // md links found in the current block (context filled on flush)
}

func isTextBlock(n ast.Node) bool {
	switch n.(type) {
	case *ast.Paragraph, *ast.Heading, *ast.TextBlock, *east.TableCell:
		return true
	}
	return false
}

func (w *walker) walk(doc ast.Node) error {
	return ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if isTextBlock(n) {
			if entering {
				w.inBlock++
				if w.inBlock == 1 {
					w.buf.Reset()
					w.show.Reset()
				}
			} else {
				w.inBlock--
				if h, ok := n.(*ast.Heading); ok && h.Level == 1 && w.h1 == "" {
					w.h1 = strings.TrimSpace(w.show.String())
				}
				if w.inBlock == 0 {
					w.flush()
				}
			}
			return ast.WalkContinue, nil
		}
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.CodeSpan:
			w.show.WriteByte('`')
			for c := t.FirstChild(); c != nil; c = c.NextSibling() {
				if tx, ok := c.(*ast.Text); ok {
					w.show.Write(tx.Segment.Value(w.src))
				}
			}
			w.show.WriteByte('`')
			return ast.WalkSkipChildren, nil
		case *ast.AutoLink:
			w.show.Write(t.URL(w.src))
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		case *ast.Link:
			if target, ok := w.mdTarget(string(t.Destination)); ok {
				w.pending = append(w.pending, domain.Link{Target: target, Kind: domain.LinkMarkdown})
			}
		case *ast.Text:
			w.buf.Write(t.Segment.Value(w.src))
			w.show.Write(t.Segment.Value(w.src))
			if t.SoftLineBreak() || t.HardLineBreak() {
				w.buf.WriteByte(' ')
				w.show.WriteByte(' ')
			}
		case *ast.String:
			w.buf.Write(t.Value)
			w.show.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
}

// flush extracts wikilinks and tags from the finished block.
func (w *walker) flush() {
	block := w.buf.String()
	ctx := context(w.show.String())
	for _, l := range w.pending {
		l.Context = ctx
		w.links = append(w.links, l)
	}
	w.pending = w.pending[:0]

	for _, m := range wikiRe.FindAllStringSubmatch(block, -1) {
		if target := wikiTarget(m[1]); target != "" {
			w.links = append(w.links, domain.Link{Target: target, Kind: domain.LinkWiki, Context: ctx})
		}
	}
	for _, m := range tagRe.FindAllStringSubmatch(block, -1) {
		w.tags = append(w.tags, m[1])
	}
}

// wikiTarget strips label, heading and block refs: [[a/b#h|label]] → "a/b".
func wikiTarget(raw string) string {
	if i := strings.IndexAny(raw, "|"); i >= 0 {
		raw = raw[:i]
	}
	if i := strings.IndexAny(raw, "#^"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimSpace(raw)
	return strings.TrimSuffix(raw, ".md")
}

// mdTarget converts a relative .md link into a vault-relative path.
func (w *walker) mdTarget(dest string) (string, bool) {
	if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "mailto:") || strings.HasPrefix(dest, "#") {
		return "", false
	}
	if i := strings.IndexAny(dest, "#?"); i >= 0 {
		dest = dest[:i]
	}
	dest = strings.ReplaceAll(dest, "%20", " ")
	if !strings.EqualFold(path.Ext(dest), ".md") {
		return "", false
	}
	var p string
	if strings.HasPrefix(dest, "/") {
		p = path.Clean(strings.TrimPrefix(dest, "/"))
	} else {
		p = path.Clean(path.Join(w.dir, dest))
	}
	if strings.HasPrefix(p, "../") {
		return "", false // points outside the vault
	}
	return p, true
}

func context(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= maxContext {
		return s
	}
	r := []rune(s)
	return string(r[:maxContext-1]) + "…"
}

func normaliseTags(in []string) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.Trim(strings.TrimSpace(t), "#/"))
		if t != "" {
			out = append(out, t)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
