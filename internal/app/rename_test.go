package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amavis442/noot/internal/app"
)

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRenameMovesNoteAndRewritesLinks(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "projects/noot.md", "---\ntitle: noot\naliases: [amavis442/noot]\n---\n# noot\n\n"+
		"See [the plan](plan.md), [[noot]] itself, [me](noot.md) and [vaults](../decisions/vaults.md).\n")
	write(t, root, "projects/plan.md", "# Plan\n\nFor [[noot]], [[projects/noot|the project]], [[noot#Status]], "+
		"[[amavis442/noot]] and [main](noot.md#status).\n")
	write(t, root, "decisions/vaults.md", "# Vaults\n\nApplies to [[Noot]] and [[projects/noot.md]].\n\n"+
		"`[[projects/noot]]` in code stays.\n")
	write(t, root, "zettels/nootje.md", "# Nootje\n\n[[nootje]] is another note.\n")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Rename(ctx, "noot", app.RenameOptions{To: "projects/noot/noot"})
	if err != nil {
		t.Fatal(err)
	}
	if res.From != "projects/noot.md" || res.To != "projects/noot/noot.md" {
		t.Errorf("result = %+v", res)
	}
	if strings.Join(res.Changed, ",") != "decisions/vaults.md,projects/plan.md,projects/noot/noot.md" {
		t.Errorf("changed = %v", res.Changed)
	}
	// plan: 2, vaults: 1, the moved note: plan, vaults (its own [me](noot.md) stays valid).
	if res.Links != 5 || len(res.Broken) != 0 {
		t.Errorf("links = %d, broken = %+v", res.Links, res.Broken)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "noot.md")); !os.IsNotExist(err) {
		t.Errorf("old file still there: %v", err)
	}

	if got := read(t, root, "projects/noot/noot.md"); !strings.Contains(got,
		"See [the plan](../plan.md), [[noot]] itself, [me](noot.md) and [vaults](../../decisions/vaults.md).") {
		t.Errorf("moved note:\n%s", got)
	}
	if got := read(t, root, "projects/plan.md"); !strings.Contains(got,
		"For [[noot]], [[projects/noot/noot|the project]], [[noot#Status]], [[amavis442/noot]] and [main](noot/noot.md#status).") {
		t.Errorf("plan:\n%s", got)
	}
	if got := read(t, root, "decisions/vaults.md"); !strings.Contains(got, "Applies to [[Noot]] and [[projects/noot/noot.md]].") ||
		!strings.Contains(got, "`[[projects/noot]]` in code stays.") {
		t.Errorf("vaults:\n%s", got)
	}
	if got := read(t, root, "zettels/nootje.md"); got != "# Nootje\n\n[[nootje]] is another note.\n" {
		t.Errorf("unrelated note changed:\n%s", got)
	}

	bls, _ := svc.Store.Backlinks(ctx, "projects/noot/noot")
	if len(bls) != 2 {
		t.Errorf("backlinks after rename = %+v", bls)
	}
}

func TestRenameWithTitle(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "zettels/alpha.md", "---\nid: A1\ntitle: Alpha note\n---\n# Alpha note\n\nbody\n")
	write(t, root, "x.md", "# X\n\n[[alpha]], [[Alpha note]], [[A1]] and [a](zettels/alpha.md).\n")
	write(t, root, "zettels/beta.md", "# Taken\n")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Rename(ctx, "A1", app.RenameOptions{To: "beta"}); err == nil {
		t.Error("renaming onto an existing note should fail")
	}

	res, err := svc.Rename(ctx, "A1", app.RenameOptions{To: "gamma", Title: "Gamma note"})
	if err != nil {
		t.Fatal(err)
	}
	if res.To != "zettels/gamma.md" || res.Links != 3 {
		t.Errorf("result = %+v", res)
	}
	if got := read(t, root, "x.md"); got != "# X\n\n[[gamma]], [[Gamma note]], [[A1]] and [a](zettels/gamma.md).\n" {
		t.Errorf("x:\n%s", got)
	}
	if got := read(t, root, "zettels/gamma.md"); got != "---\nid: A1\ntitle: Gamma note\n---\n# Gamma note\n\nbody\n" {
		t.Errorf("gamma:\n%s", got)
	}

	// Title only: the file stays where it is.
	res, err = svc.Rename(ctx, "A1", app.RenameOptions{Title: "Delta"})
	if err != nil || res.To != "zettels/gamma.md" || res.Links != 1 {
		t.Errorf("title only = %+v, %v", res, err)
	}
	if got := read(t, root, "x.md"); !strings.Contains(got, "[[Delta]]") {
		t.Errorf("x after title rename:\n%s", got)
	}
}

func TestRenameAvoidsNamesOfOtherNotes(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "a/old.md", "# Old\n")
	write(t, root, "x.md", "# X\n\n[[old]]\n")
	write(t, root, "new.md", "# Other\n")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}
	// [[new]] would resolve to new.md, so the link to a/new.md must use the path.
	if _, err := svc.Rename(ctx, "a/old", app.RenameOptions{To: "new"}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, "x.md"); got != "# X\n\n[[a/new]]\n" {
		t.Errorf("x:\n%s", got)
	}
}

func TestRenameDryRunAndErrors(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "a.md", "# A\n")
	write(t, root, "b.md", "# B\n\n[[a]]\n")
	write(t, root, "plain.md", "no title line\n")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Rename(ctx, "a", app.RenameOptions{To: "c", DryRun: true})
	if err != nil || res.To != "c.md" || strings.Join(res.Changed, ",") != "b.md" || res.Links != 1 {
		t.Errorf("dry run = %+v, %v", res, err)
	}
	if read(t, root, "b.md") != "# B\n\n[[a]]\n" || read(t, root, "a.md") != "# A\n" {
		t.Error("dry run changed files")
	}

	for name, tc := range map[string]struct {
		note string
		opt  app.RenameOptions
	}{
		"nothing to do":  {"b", app.RenameOptions{}},
		"same path":      {"b", app.RenameOptions{To: "b"}},
		"outside vault":  {"b", app.RenameOptions{To: "../escape"}},
		"from file name": {"plain", app.RenameOptions{Title: "Other"}},
	} {
		if _, err := svc.Rename(ctx, tc.note, tc.opt); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRenameFileRewritesLinks(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "notes/a.md", "# A\n\nBy name [[photo.png]], by path ![[assets/photo.png|label]], "+
		"[pic](../assets/photo.png#x) and [[other.png]].\n\n`[[photo.png]]` in code stays.\n")
	write(t, root, "b.md", "# B\n\n[[Photo.PNG]] and [[assets/photo.png]].\n")
	write(t, root, "assets/photo.png", "png")
	write(t, root, "other.png", "png")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Rename(ctx, "photo.png", app.RenameOptions{To: "img/pic.png"})
	if err != nil {
		t.Fatal(err)
	}
	if res.From != "assets/photo.png" || res.To != "img/pic.png" || res.Links != 5 || len(res.Broken) != 0 {
		t.Errorf("result = %+v", res)
	}
	if strings.Join(res.Changed, ",") != "b.md,notes/a.md" {
		t.Errorf("changed = %v", res.Changed)
	}
	if got := read(t, root, "notes/a.md"); !strings.Contains(got,
		"By name [[pic.png]], by path ![[img/pic.png|label]], [pic](../img/pic.png#x) and [[other.png]].") ||
		!strings.Contains(got, "`[[photo.png]]` in code stays.") {
		t.Errorf("a:\n%s", got)
	}
	if got := read(t, root, "b.md"); !strings.Contains(got, "[[pic.png]] and [[img/pic.png]].") {
		t.Errorf("b:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "assets", "photo.png")); !os.IsNotExist(err) {
		t.Errorf("old file still there: %v", err)
	}
	if got := read(t, root, "img/pic.png"); got != "png" {
		t.Errorf("moved file = %q", got)
	}
	if bls, _ := svc.Store.FileBacklinks(ctx, "img/pic.png"); len(bls) != 2 {
		t.Errorf("backlinks after rename = %+v", bls)
	}
}

func TestRenameFileKeepsLinksUnambiguous(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "a.md", "# A\n\n[[photo.png]] and [[pic.png]].\n")
	write(t, root, "photo.png", "p")
	write(t, root, "pic.png", "q")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}
	// "pic.png" already names another file: the link to the moved file must
	// use its path, and the link that meant the other file stays as it is.
	dry, err := svc.Rename(ctx, "photo.png", app.RenameOptions{To: "sub/pic.png", DryRun: true})
	if err != nil || !dry.DryRun || dry.Links != 1 {
		t.Fatalf("dry run = %+v, %v", dry, err)
	}
	if _, err := os.Stat(filepath.Join(root, "photo.png")); err != nil {
		t.Fatalf("dry run moved the file: %v", err)
	}
	if _, err := svc.Rename(ctx, "photo.png", app.RenameOptions{To: "sub/pic.png"}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, "a.md"); !strings.Contains(got, "[[sub/pic.png]] and [[pic.png]].") {
		t.Errorf("a:\n%s", got)
	}
	if broken, _ := svc.Store.Broken(ctx); len(broken) != 0 {
		t.Errorf("broken = %+v", broken)
	}
	if bls, _ := svc.Store.FileBacklinks(ctx, "pic.png"); len(bls) != 1 {
		t.Errorf("the other pic.png lost its link: %+v", bls)
	}
}

func TestRenameFileRefusesToShadowAnotherFile(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "a.md", "# A\n\n[[pic.png]]\n")
	write(t, root, "deep/er/pic.png", "q")
	write(t, root, "photo.png", "p")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}
	// By file name [[pic.png]] means the shortest path: the moved file would
	// take over that link from deep/er/pic.png.
	_, err := svc.Rename(ctx, "photo.png", app.RenameOptions{To: "pic.png"})
	if err == nil || !strings.Contains(err.Error(), "deep/er/pic.png") {
		t.Errorf("err = %v", err)
	}
}

func TestRenameFileErrors(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "a.md", "# A\n\n[[photo.png]]\n")
	write(t, root, "photo.png", "p")
	write(t, root, "taken.png", "t")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}
	for name, opt := range map[string]app.RenameOptions{
		"title":     {Title: "x"},
		"nothing":   {},
		"exists":    {To: "taken.png"},
		"to a note": {To: "photo.md"},
		"hidden":    {To: ".photo.png"},
		"outside":   {To: "../photo.png"},
	} {
		if _, err := svc.Rename(ctx, "photo.png", opt); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := svc.Rename(ctx, "nope.png", app.RenameOptions{To: "x.png"}); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	if got := read(t, root, "a.md"); !strings.Contains(got, "[[photo.png]]") {
		t.Errorf("a changed:\n%s", got)
	}
}
