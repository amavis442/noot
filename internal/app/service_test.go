package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/config"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/storage/sqlite"
	"github.com/amavis442/noot/internal/vault"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (*app.Service, string) {
	t.Helper()
	root := t.TempDir()
	v := &vault.Vault{Root: root}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	st, err := sqlite.Open(v.IndexFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return &app.Service{Vault: v, Store: st}, root
}

func TestSyncAndQueries(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)

	write(t, root, "zettels/a.md", "---\nid: A1\ntitle: Alpha note\ntags: [go]\naliases: [first]\n---\nLinks to [[b]] and [[missing]].")
	write(t, root, "zettels/b.md", "# Beta\nBack to [[first]] and [alpha](a.md). Über café #Tést")
	write(t, root, "lonely.md", "# Lonely\nnothing here")
	write(t, root, ".git/ignored.md", "[[a]]")
	write(t, root, "node_modules/x.md", "[[a]]")

	st, err := svc.Sync(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 3 || st.Updated != 3 || len(st.Errors) != 0 {
		t.Fatalf("stats = %+v", st)
	}

	// Second sync is a no-op.
	if st, _ = svc.Sync(ctx, false); st.Updated != 0 {
		t.Errorf("expected no changes, got %+v", st)
	}

	a, err := svc.MustResolve(ctx, "Alpha Note")
	if err != nil || a.ID != "A1" {
		t.Fatalf("resolve by title: %+v %v", a, err)
	}
	bls, _ := svc.Store.Backlinks(ctx, "A1")
	if len(bls) != 1 || bls[0].From.Path != "zettels/b.md" {
		t.Errorf("backlinks = %+v", bls)
	}
	broken, _ := svc.Store.Broken(ctx)
	if len(broken) != 1 || broken[0].Target != "missing" {
		t.Errorf("broken = %+v", broken)
	}
	orphans, _ := svc.Store.Orphans(ctx)
	if len(orphans) != 1 || orphans[0].Path != "lonely.md" {
		t.Errorf("orphans = %+v", orphans)
	}

	// Diacritics are folded: "cafe" finds "café", "test" finds the #Tést tag.
	for _, q := range []string{"cafe", "uber", "tes"} {
		hits, err := svc.Store.Search(ctx, domain.SearchQuery{Text: q})
		if err != nil || len(hits) != 1 || hits[0].Path != "zettels/b.md" {
			t.Errorf("search %q = %+v %v", q, hits, err)
		}
	}
	hits, _ := svc.Store.Search(ctx, domain.SearchQuery{Tags: []string{"go"}})
	if len(hits) != 1 || hits[0].ID != "A1" {
		t.Errorf("tag filter = %+v", hits)
	}

	// Deleting a file removes it and breaks links to it.
	if err := os.Remove(filepath.Join(root, "zettels", "a.md")); err != nil {
		t.Fatal(err)
	}
	if st, _ = svc.Sync(ctx, false); st.Deleted != 1 || st.Total != 2 {
		t.Errorf("after delete: %+v", st)
	}
	broken, _ = svc.Store.Broken(ctx)
	if len(broken) != 2 {
		t.Errorf("expected 2 broken links after delete, got %+v", broken)
	}
}

func TestDuplicateIDIsReported(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "one.md", "---\nid: same\n---\n")
	write(t, root, "two.md", "---\nid: same\n---\n")

	st, err := svc.Sync(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 1 || len(st.Errors) != 1 {
		t.Errorf("stats = %+v", st)
	}
}
