package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitEnv gives tests a fixed identity and no user/system config, so they
// behave the same on every machine.
func gitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// repoWithRemote returns a working repo whose origin is a bare repo.
func repoWithRemote(t *testing.T) (work, bare string) {
	t.Helper()
	gitEnv(t)
	root := t.TempDir()
	bare, work = filepath.Join(root, "remote.git"), filepath.Join(root, "work")
	run(t, root, "init", "-q", "--bare", "-b", "main", bare)
	run(t, root, "init", "-q", "-b", "main", work)
	run(t, work, "remote", "add", "origin", bare)
	return work, bare
}

func TestIsRepo(t *testing.T) {
	gitEnv(t)
	ctx := context.Background()
	plain := t.TempDir()
	if ok, err := New(plain).IsRepo(ctx); err != nil || ok {
		t.Errorf("plain dir: ok=%v err=%v", ok, err)
	}
	run(t, plain, "init", "-q")
	if ok, err := New(plain).IsRepo(ctx); err != nil || !ok {
		t.Errorf("repo: ok=%v err=%v", ok, err)
	}
	sub := filepath.Join(plain, "sub")
	_ = os.Mkdir(sub, 0o755)
	if ok, _ := New(sub).IsRepo(ctx); ok {
		t.Error("a subdirectory of a repo is not a vault repo root")
	}
}

func TestCommitAndPushFlow(t *testing.T) {
	ctx := context.Background()
	work, bare := repoWithRemote(t)
	g := New(work)

	write(t, work, "zettels/a.md", "# A")
	write(t, work, "b.md", "# B")
	if err := g.AddAll(ctx); err != nil {
		t.Fatal(err)
	}
	changes, err := g.StagedChanges(ctx)
	if err != nil || len(changes) != 2 || changes[0] != "A\tb.md" || changes[1] != "A\tzettels/a.md" {
		t.Fatalf("staged = %q, %v", changes, err)
	}
	tree, err := g.WriteTree(ctx)
	if err != nil || len(tree) < 40 {
		t.Fatalf("tree = %q, %v", tree, err)
	}
	hash, err := g.Commit(ctx, "noot: "+tree[:12], strings.Join(changes, "\n"))
	if err != nil || len(hash) < 7 {
		t.Fatalf("commit = %q, %v", hash, err)
	}
	if msg := run(t, work, "log", "-1", "--format=%s%n%b"); !strings.Contains(msg, "noot: "+tree[:12]) || !strings.Contains(msg, "A\tzettels/a.md") {
		t.Errorf("commit message = %q", msg)
	}

	if up, _ := g.HasUpstream(ctx); up {
		t.Error("fresh branch should have no upstream")
	}
	if ok, _ := g.HasRemote(ctx, "origin"); !ok {
		t.Error("origin should exist")
	}
	if ok, _ := g.HasRemote(ctx, "nope"); ok {
		t.Error("unknown remote should not exist")
	}
	if err := g.PushSetUpstream(ctx, "origin"); err != nil {
		t.Fatal(err)
	}
	if up, _ := g.HasUpstream(ctx); !up {
		t.Error("push -u should set the upstream")
	}
	if remoteHead := run(t, bare, "rev-parse", "main"); !strings.HasPrefix(remoteHead, hash) {
		t.Errorf("remote main = %s, want %s", remoteHead, hash)
	}

	// Nothing staged now; one new commit makes us one ahead.
	if c, _ := g.StagedChanges(ctx); len(c) != 0 {
		t.Errorf("staged after commit = %q", c)
	}
	write(t, work, "b.md", "# B changed")
	_ = g.AddAll(ctx)
	if c, _ := g.StagedChanges(ctx); len(c) != 1 || c[0] != "M\tb.md" {
		t.Errorf("modified = %q", c)
	}
	if _, err := g.Commit(ctx, "second", ""); err != nil {
		t.Fatal(err)
	}
	if n, err := g.Ahead(ctx); err != nil || n != 1 {
		t.Errorf("ahead = %d, %v", n, err)
	}
	if err := g.Push(ctx); err != nil {
		t.Fatal(err)
	}
	if n, _ := g.Ahead(ctx); n != 0 {
		t.Errorf("ahead after push = %d", n)
	}
}

func TestPullRebaseBringsInOtherClone(t *testing.T) {
	ctx := context.Background()
	work, bare := repoWithRemote(t)
	g := New(work)
	write(t, work, "a.md", "# A")
	_ = g.AddAll(ctx)
	if _, err := g.Commit(ctx, "first", ""); err != nil {
		t.Fatal(err)
	}
	if err := g.PushSetUpstream(ctx, "origin"); err != nil {
		t.Fatal(err)
	}

	// Another machine pushes a change.
	other := filepath.Join(filepath.Dir(bare), "other")
	run(t, filepath.Dir(bare), "clone", "-q", bare, other)
	write(t, other, "from-other.md", "# Other")
	run(t, other, "add", "-A")
	run(t, other, "commit", "-q", "-m", "other")
	run(t, other, "push", "-q")

	// Local change, then pull --rebase must succeed and keep both.
	write(t, work, "local.md", "# Local")
	_ = g.AddAll(ctx)
	if _, err := g.Commit(ctx, "local", ""); err != nil {
		t.Fatal(err)
	}
	if err := g.PullRebase(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(work, "from-other.md")); err != nil {
		t.Error("pull did not bring in the other clone's file")
	}
	if err := g.Push(ctx); err != nil {
		t.Fatalf("push after rebase: %v", err)
	}
}

func TestErrorsIncludeGitOutput(t *testing.T) {
	gitEnv(t)
	err := New(t.TempDir()).AddAll(context.Background())
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
		t.Errorf("err = %v", err)
	}
}

func TestMsysToWindows(t *testing.T) {
	for in, want := range map[string]string{
		`\d\Noot\Patrick`: `d:\Noot\Patrick`,
		`\C\Users\amavi`:  `C:\Users\amavi`,
		`D:\Noot\Patrick`: `D:\Noot\Patrick`,
		`\\server\share`:  `\\server\share`,
		`\tmp`:            `\tmp`,
	} {
		if got := msysToWindows(in); got != want {
			t.Errorf("msysToWindows(%q) = %q, want %q", in, got, want)
		}
	}
}
