package parser

import (
	"strings"
	"testing"

	"github.com/amavis442/noot/internal/domain"
)

func TestRewriteLinks(t *testing.T) {
	src := "---\ntitle: \"[[old]] in frontmatter\"\n---\n" +
		"See [[old]], [[ old#Heading|label ]], [[old.md]] and [[keep]].\n\n" +
		"Also [md](../zettels/old.md#part), [web](https://x.nl/old.md) and [other](keep.md).\n\n" +
		"Inline `[[old]]` stays.\n\n" +
		"```\n[[old]] [md](../zettels/old.md)\n```\n"

	var seen []string
	got, n, err := RewriteLinks("projects/p.md", []byte(src), func(l domain.Link, written string) (string, bool) {
		seen = append(seen, string(l.Kind)+":"+l.Target+":"+written)
		switch {
		case l.Kind == domain.LinkWiki && l.Target == "old":
			return strings.Replace(written, "old", "fresh", 1), true
		case l.Kind == domain.LinkMarkdown && l.Target == "zettels/old.md":
			return "../zettels/fresh.md", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\ntitle: \"[[old]] in frontmatter\"\n---\n" +
		"See [[fresh]], [[ fresh#Heading|label ]], [[fresh.md]] and [[keep]].\n\n" +
		"Also [md](../zettels/fresh.md#part), [web](https://x.nl/old.md) and [other](keep.md).\n\n" +
		"Inline `[[old]]` stays.\n\n" +
		"```\n[[old]] [md](../zettels/old.md)\n```\n"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if n != 4 {
		t.Errorf("rewritten = %d, want 4", n)
	}
	wantSeen := "wiki:old:old,wiki:old:old,wiki:old:old.md,wiki:keep:keep,md:zettels/old.md:../zettels/old.md,md:projects/keep.md:keep.md"
	if strings.Join(seen, ",") != wantSeen {
		t.Errorf("seen = %v", seen)
	}
}

func TestRewriteLinksNoChange(t *testing.T) {
	src := "# Note\n\n[[a]]\n"
	got, n, err := RewriteLinks("n.md", []byte(src), func(domain.Link, string) (string, bool) { return "", false })
	if err != nil || n != 0 || string(got) != src {
		t.Errorf("got %q, %d, %v", got, n, err)
	}
}
