package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/config"
)

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Clean architectuur in Go!": "clean-architectuur-in-go",
		"  Café über naïef  ":       "cafe-uber-naief",
		"Wat is een Zettel?":        "wat-is-een-zettel",
		"!!!":                       "",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWalkSkipsNestedVaults(t *testing.T) {
	root := t.TempDir()
	parent := &Vault{Root: root}
	child := &Vault{Root: filepath.Join(root, "Claude")}
	for _, v := range []*Vault{parent, child} {
		if err := v.Init(config.KindPersonal); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(parent.Abs("top.md"), []byte("# top"), 0o644)
	_ = os.WriteFile(child.Abs("inner.md"), []byte("# inner"), 0o644)

	files, err := parent.Walk()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["top.md"]; !ok || len(files) != 1 {
		t.Errorf("parent walk = %v, want only top.md", files)
	}
	if parent.ShouldWatchDir(child.Root) {
		t.Error("nested vault should not be watched by the parent")
	}
	if files, _ := child.Walk(); len(files) != 1 {
		t.Errorf("child walk = %v", files)
	}
}

func TestSetTitle(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"frontmatter and h1",
			"---\nid: A1\ntitle: Alpha note\n---\n\n# Alpha note\n\nbody # Alpha note\n",
			"---\nid: A1\ntitle: \"Beta: note\"\n---\n\n# Beta: note\n\nbody # Alpha note\n"},
		{"h1 only", "# Alpha note\n\ntext\n", "# Beta: note\n\ntext\n"},
		{"other h1 kept", "---\ntitle: Alpha note\n---\n# Something else\n", "---\ntitle: \"Beta: note\"\n---\n# Something else\n"},
		{"crlf", "---\r\ntitle: Alpha note\r\n---\r\n# Alpha note\r\n", "---\r\ntitle: \"Beta: note\"\r\n---\r\n# Beta: note\r\n"},
	} {
		got, err := SetTitle([]byte(tc.in), "Alpha note", "Beta: note")
		if err != nil || string(got) != tc.want {
			t.Errorf("%s: got %q, %v\nwant %q", tc.name, got, err, tc.want)
		}
	}
	if _, err := SetTitle([]byte("just text\n"), "file-name", "Other"); err == nil {
		t.Error("a note whose title comes from its file name has no title line to change")
	}
}

// frontmatterField reads a single frontmatter value from a note on disk.
func frontmatterField(t *testing.T, v *Vault, rel, key string) string {
	t.Helper()
	b, err := os.ReadFile(v.Abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	t.Fatalf("%s: no %s in frontmatter:\n%s", rel, key, b)
	return ""
}

func TestNewNoteGivesEveryNoteItsOwnID(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 29, 12, 30, 0, 0, time.UTC)

	// Notes created within the same second, even the same millisecond, must
	// still get different ids: the index skips a note whose id another file
	// already uses.
	seen := map[string]string{}
	for _, title := range []string{"Eerste", "Tweede", "Derde"} {
		rel, err := v.NewNote(title, "zettels", nil, "", now)
		if err != nil {
			t.Fatal(err)
		}
		id := frontmatterField(t, v, rel, "id")
		if other, dup := seen[id]; dup {
			t.Fatalf("%s and %s share id %s", other, rel, id)
		}
		seen[id] = rel
		if len(id) != idLen {
			t.Errorf("%s: id %q is %d characters, want %d", rel, id, len(id), idLen)
		}
		if got := frontmatterField(t, v, rel, "created"); got != "2026-10-29" {
			t.Errorf("%s: created = %q, want 2026-10-29", rel, got)
		}
	}
}

func TestNewNoteWritesBodyBelowTheHeading(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)

	rel, err := v.NewNote("Met tekst", "zettels", []string{"idee"}, "Eerste regel.\nTweede regel.", now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(v.Abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(b), "---\n\n# Met tekst\n\nEerste regel.\nTweede regel.\n") {
		t.Errorf("note does not end with heading and body:\n%s", b)
	}

	rel, err = v.NewNote("Zonder tekst", "zettels", nil, "  \n", now)
	if err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(v.Abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(b), "---\n\n# Zonder tekst\n\n") {
		t.Errorf("blank body should leave only the heading:\n%s", b)
	}
}
