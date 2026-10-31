package parser

import "testing"

func TestBody(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"frontmatter":       {"---\ntitle: X\n---\n# X\n\ntext\n", "# X\n\ntext\n"},
		"crlf":              {"---\r\ntitle: X\r\n---\r\nbody\r\n", "body\r\n"},
		"none":              {"# Plain\n", "# Plain\n"},
		"invalid yaml kept": {"---\n: [\n---\nbody\n", "---\n: [\n---\nbody\n"},
	}
	for name, c := range cases {
		if got := string(Body([]byte(c.in))); got != c.want {
			t.Errorf("%s: Body = %q, want %q", name, got, c.want)
		}
	}
}
