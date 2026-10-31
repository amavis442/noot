package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/amavis442/noot/internal/domain"
)

// RenderHTML renders the body of the note at relPath as HTML, without its
// frontmatter. Wikilinks and relative Markdown links become links to
// linkPrefix + the escaped target, so a viewer can resolve them as notes. Raw
// HTML in the note is left out, so the result is safe to show in a page.
func RenderHTML(relPath string, content []byte, linkPrefix string) (string, error) {
	href := func(target string) string { return linkPrefix + url.PathEscape(target) }

	content, _, err := RewriteLinks(relPath, content, func(l domain.Link, _ string) (string, bool) {
		if l.Kind != domain.LinkMarkdown {
			return "", false
		}
		return href(l.Target), true
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
		src.Write(body[last:m[0]])
		src.WriteString("[" + label + "](" + href(target) + ")")
		last = m[1]
	}
	src.Write(body[last:])

	var out bytes.Buffer
	if err := md.Convert(src.Bytes(), &out); err != nil {
		return "", err
	}
	return out.String(), nil
}
