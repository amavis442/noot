package parser

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/amavis442/noot/internal/domain"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// mdLinkRe finds the destination of an inline Markdown link: [label](dest).
var mdLinkRe = regexp.MustCompile(`\]\(([^)\s]+)`)

// RewriteFunc gets a link as the index stores it and its target as written in
// the file (without label, heading or anchor). It returns the replacement for
// the written target, or false to leave the link alone.
type RewriteFunc func(l domain.Link, written string) (string, bool)

// RewriteLinks rewrites the targets of the wikilinks and Markdown links in the
// body of the note at relPath, skipping code, exactly the links Parse indexes.
// Labels, headings and anchors are kept. It returns the new content and the
// number of links rewritten.
func RewriteLinks(relPath string, content []byte, fn RewriteFunc) ([]byte, int, error) {
	_, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, 0, err
	}
	offset := len(content) - len(body)
	code := codeRanges(body)
	inCode := func(pos int) bool {
		return slices.ContainsFunc(code, func(r [2]int) bool { return pos >= r[0] && pos < r[1] })
	}

	type edit struct {
		start, stop int
		text        string
	}
	var edits []edit
	for _, m := range wikiRe.FindAllSubmatchIndex(body, -1) {
		if inCode(m[0]) {
			continue
		}
		inner := string(body[m[2]:m[3]])
		end := len(inner)
		if i := strings.IndexByte(inner, '|'); i >= 0 {
			end = i
		}
		if i := strings.IndexAny(inner[:end], "#^"); i >= 0 {
			end = i
		}
		lead := len(inner[:end]) - len(strings.TrimLeft(inner[:end], " \t"))
		written := strings.TrimSpace(inner[:end])
		target := strings.TrimSuffix(written, ".md")
		if target == "" {
			continue
		}
		if repl, ok := fn(domain.Link{Target: target, Kind: domain.LinkWiki}, written); ok {
			start := m[2] + lead
			edits = append(edits, edit{start, start + len(written), repl})
		}
	}
	w := walker{dir: path.Dir(relPath)}
	for _, m := range mdLinkRe.FindAllSubmatchIndex(body, -1) {
		if inCode(m[0]) {
			continue
		}
		dest := string(body[m[2]:m[3]])
		target, ok := w.mdTarget(dest)
		if !ok {
			continue
		}
		written := dest
		if i := strings.IndexAny(dest, "#?"); i >= 0 {
			written = dest[:i]
		}
		if repl, ok := fn(domain.Link{Target: target, Kind: domain.LinkMarkdown}, written); ok {
			edits = append(edits, edit{m[2], m[2] + len(written), repl})
		}
	}
	if len(edits) == 0 {
		return content, 0, nil
	}

	slices.SortFunc(edits, func(a, b edit) int { return a.start - b.start })
	var out strings.Builder
	out.Write(content[:offset])
	last := 0
	for _, e := range edits {
		out.Write(body[last:e.start])
		out.WriteString(e.text)
		last = e.stop
	}
	out.Write(body[last:])
	return []byte(out.String()), len(edits), nil
}

// codeRanges returns the byte ranges of code blocks and code spans in body.
func codeRanges(body []byte) [][2]int {
	var out [][2]int
	doc := md.Parser().Parse(text.NewReader(body))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			if lines := t.Lines(); lines.Len() > 0 {
				out = append(out, [2]int{lines.At(0).Start, lines.At(lines.Len() - 1).Stop})
			}
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			first, last := t.FirstChild(), t.LastChild()
			ft, ok1 := first.(*ast.Text)
			lt, ok2 := last.(*ast.Text)
			if ok1 && ok2 {
				// Include the backticks around the span.
				out = append(out, [2]int{ft.Segment.Start - 1, lt.Segment.Stop + 1})
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return out
}
