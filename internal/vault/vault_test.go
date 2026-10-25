package vault

import (
	"os"
	"path/filepath"
	"testing"

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
