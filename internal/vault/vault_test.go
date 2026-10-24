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
