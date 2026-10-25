package app_test

import (
	"context"
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
