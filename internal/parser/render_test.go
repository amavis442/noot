package parser

import (
	"strings"
	"testing"
)

func TestRenderHTML(t *testing.T) {
	src := "---\ntitle: Hidden frontmatter\n---\n" +
		"# Kop\n\n" +
		"See [[Other note|the other]], [[zettels/b#Part]] and [md](../zettels/c.md#x).\n\n" +
		"Web [site](https://example.com) stays. <script>alert(1)</script>\n\n" +
		"Inline `[[code]]` stays.\n"

	got, err := RenderHTML("projects/p.md", []byte(src), "#/note/")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<h1",
		`<a href="#/note/Other%20note">the other</a>`,
		`<a href="#/note/zettels%2Fb">zettels/b#Part</a>`,
		`<a href="#/note/zettels%2Fc.md#x">md</a>`,
		`<a href="https://example.com">site</a>`,
		"<code>[[code]]</code>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	for _, not := range []string{"Hidden frontmatter", "<script>"} {
		if strings.Contains(got, not) {
			t.Errorf("unexpected %s in:\n%s", not, got)
		}
	}
}

func TestRenderHTMLBrokenFrontmatter(t *testing.T) {
	_, err := RenderHTML("a.md", []byte("---\ntitle: [oops\n---\n# x"), "#/note/")
	if err == nil || !strings.HasPrefix(err.Error(), "frontmatter: ") {
		t.Fatalf("err = %v, want it to name the frontmatter", err)
	}
}
