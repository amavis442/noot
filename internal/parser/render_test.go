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

func TestRenderFiles(t *testing.T) {
	files := map[string]string{ // wikilink target → vault path
		"photo.png":      "assets/photo.png",
		"scan.pdf":       "scan.pdf",
		"logo.svg":       "logo.svg",
		"a (1) b.png":    "a (1) b.png",
		"img/inline.gif": "img/inline.gif",
	}
	opts := RenderOptions{
		NotePrefix: "#/note/",
		FilePrefix: "/api/files/",
		File: func(target string) (string, bool) {
			p, ok := files[target]
			return p, ok
		},
	}
	src := "Embed ![[photo.png|Holiday]], link [[photo.png]], pdf ![[scan.pdf]] and [[scan.pdf|the scan]].\n\n" +
		"Svg ![[logo.svg]], odd name ![[a (1) b.png]], note [[Other note]], missing ![[nope.png]].\n\n" +
		"Md [pic](../img/inline.gif), ![alt](../assets/photo.png), [doc](../my%20doc.pdf) and [note](../b.md).\n\n" +
		"`![[photo.png]]` in code stays.\n"
	got, err := Render("notes/n.md", []byte(src), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<img src="/api/files/assets/photo.png" alt="Holiday"`,
		`<a href="/api/files/assets/photo.png">photo.png</a>`,
		`<a href="/api/files/scan.pdf">scan.pdf</a>`, // an embedded PDF is a link, not an image
		`<a href="/api/files/scan.pdf">the scan</a>`,
		`<a href="/api/files/logo.svg">logo.svg</a>`, // svg can carry scripts: never an <img>
		`<img src="/api/files/a%20%281%29%20b.png"`,
		`<a href="#/note/Other%20note">Other note</a>`,
		`<a href="#/note/nope.png">nope.png</a>`, // not a file: a note link, never a broken <img>
		`<a href="/api/files/img/inline.gif">pic</a>`,
		`<img src="/api/files/assets/photo.png" alt="alt"`,
		`<a href="/api/files/my%20doc.pdf">doc</a>`,
		`<a href="#/note/b.md">note</a>`,
		"<code>![[photo.png]]</code>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<img src=\"/api/files/logo.svg") || strings.Contains(got, "<img src=\"/api/files/scan.pdf") {
		t.Errorf("only raster images become <img>:\n%s", got)
	}
}

func TestRenderWithoutFilesKeepsNoteLinks(t *testing.T) {
	got, err := RenderHTML("n.md", []byte("A [[photo.png]] and [x](a.pdf)."), "#/note/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<a href="#/note/photo.png">`) || !strings.Contains(got, `<a href="a.pdf">x</a>`) {
		t.Errorf("got:\n%s", got)
	}
}
