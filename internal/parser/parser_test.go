package parser

import (
	"slices"
	"strings"
	"testing"

	"github.com/amavis442/noot/internal/domain"
)

func TestParse(t *testing.T) {
	src := "\xef\xbb\xbf---\r\nid: 42\r\ntitle: Frontmatter title\r\ntags: go, Architectuur\r\naliases: [ca]\r\n---\r\n" +
		"# Heading title\n\n" +
		"See [[Other note#Section|label]] and [x](../projects/p.md). #refactoring #123 mail@x.nl\n\n" +
		"```go\n// [[in-code]] #nottag\n```\n\n" +
		"Inline `[[in-span]]` and an ![[embed]] plus [web](https://example.com/a.md).\n\n" +
		"- item with #go/generics\n"

	n, err := Parse("zettels/note.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != "42" || n.Title != "Frontmatter title" {
		t.Errorf("id/title = %q/%q", n.ID, n.Title)
	}
	if want := []string{"architectuur", "go", "go/generics", "refactoring"}; !slices.Equal(n.Tags, want) {
		t.Errorf("tags = %v, want %v", n.Tags, want)
	}
	if !slices.Equal(n.Aliases, []string{"ca"}) {
		t.Errorf("aliases = %v", n.Aliases)
	}

	var got []string
	for _, l := range n.Links {
		got = append(got, string(l.Kind)+":"+l.Target)
	}
	want := []string{"md:projects/p.md", "wiki:Other note", "wiki:embed"}
	if !slices.Equal(got, want) {
		t.Errorf("links = %v, want %v", got, want)
	}
	if n.Links[1].Context == "" {
		t.Error("expected link context")
	}
	if ctx := n.Links[2].Context; !strings.Contains(ctx, "`[[in-span]]`") {
		t.Errorf("context should keep inline code, got %q", ctx)
	}
}

func TestParseDefaults(t *testing.T) {
	n, err := Parse("journal/2026-10-24.md", []byte("no frontmatter, no heading"))
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != "journal/2026-10-24" || n.Title != "2026-10-24" {
		t.Errorf("got id=%q title=%q", n.ID, n.Title)
	}

	n, _ = Parse("a.md", []byte("# From H1\n\nbody"))
	if n.Title != "From H1" {
		t.Errorf("title = %q", n.Title)
	}
}

func TestUnclosedFrontmatterIsBody(t *testing.T) {
	n, err := Parse("a.md", []byte("---\nnot: closed\n\n[[link]]"))
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Links) != 1 || n.Links[0].Kind != domain.LinkWiki {
		t.Errorf("links = %+v", n.Links)
	}
}
