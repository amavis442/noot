package editor_test

import (
	"runtime"
	"slices"
	"testing"

	"github.com/amavis442/noot/internal/editor"
)

func TestCommandPrefersVisualOverEditor(t *testing.T) {
	t.Setenv("VISUAL", "nano")
	t.Setenv("EDITOR", "vim")
	c := editor.Command("note.md")
	if want := []string{"nano", "note.md"}; !slices.Equal(c.Args, want) {
		t.Fatalf("args = %q, want %q", c.Args, want)
	}
}

func TestCommandSplitsEditorArguments(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "code -w")
	c := editor.Command("note.md")
	if want := []string{"code", "-w", "note.md"}; !slices.Equal(c.Args, want) {
		t.Fatalf("args = %q, want %q", c.Args, want)
	}
}

func TestCommandFallsBackPerOS(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	want := "vi"
	if runtime.GOOS == "windows" {
		want = "notepad"
	}
	c := editor.Command("note.md")
	if !slices.Equal(c.Args, []string{want, "note.md"}) {
		t.Fatalf("args = %q, want %s note.md", c.Args, want)
	}
}
