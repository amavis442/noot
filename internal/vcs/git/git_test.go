package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// TestNoGlobCharactersInArgs guards against MSYS2/Cygwin git on Windows: when
// such a git is started from a non-MSYS process, its runtime glob- and
// brace-expands the arguments, so "@{u}" stops meaning "upstream". Every
// argument noot passes must be free of glob characters.
func TestNoGlobCharactersInArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script wrapper around git")
	}
	ctx := context.Background()
	work, _ := repoWithRemote(t)
	logFile := filepath.Join(t.TempDir(), "args.log")
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "git-wrapper")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> '" + logFile + "'; done\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	g := &Repo{dir: work, bin: wrapper}

	write(t, work, "a.md", "# A")
	_, _ = g.IsRepo(ctx)
	_ = g.AddAll(ctx)
	_, _ = g.StagedChanges(ctx)
	_, _ = g.WriteTree(ctx)
	_, _ = g.Commit(ctx, "first", "")
	_, _ = g.HasUpstream(ctx)
	_, _ = g.HasRemote(ctx, "origin")
	_ = g.PushSetUpstream(ctx, "origin")
	if up, err := g.HasUpstream(ctx); err != nil || !up {
		t.Fatalf("upstream not detected after push -u: %v %v", up, err)
	}
	_ = g.PullRebase(ctx)
	if n, err := g.Ahead(ctx); err != nil || n != 0 {
		t.Errorf("ahead = %d, %v", n, err)
	}
	_ = g.Push(ctx)

	b, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.ContainsAny(arg, "{}*?[") {
			t.Errorf("argument %q contains glob characters; MSYS2 git would expand it", arg)
		}
	}
}

func TestUpstreamOnDetachedHeadAndUntrackedBranch(t *testing.T) {
	ctx := context.Background()
	work, _ := repoWithRemote(t)
	g := New(work)
	write(t, work, "a.md", "# A")
	_ = g.AddAll(ctx)
	if _, err := g.Commit(ctx, "first", ""); err != nil {
		t.Fatal(err)
	}
	if up, err := g.HasUpstream(ctx); err != nil || up {
		t.Errorf("untracked branch: up=%v err=%v", up, err)
	}
	run(t, work, "checkout", "-q", "--detach")
	if up, err := g.HasUpstream(ctx); err != nil || up {
		t.Errorf("detached HEAD: up=%v err=%v", up, err)
	}
}

func TestHeadAndPullWithLocalChanges(t *testing.T) {
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
	before, err := g.Head(ctx)
	if err != nil || len(before) != 40 {
		t.Fatalf("head = %q, %v", before, err)
	}

	other := filepath.Join(filepath.Dir(bare), "other")
	run(t, filepath.Dir(bare), "clone", "-q", bare, other)
	write(t, other, "b.md", "# B")
	run(t, other, "add", "-A")
	run(t, other, "commit", "-q", "-m", "other")
	run(t, other, "push", "-q")

	// Uncommitted local edit: pull must still work and keep it (autostash).
	write(t, work, "a.md", "# A, edited but not committed")
	if err := g.PullRebase(ctx); err != nil {
		t.Fatalf("pull with local changes: %v", err)
	}
	after, _ := g.Head(ctx)
	if after == before {
		t.Error("HEAD should move after pulling a new commit")
	}
	if b, _ := os.ReadFile(filepath.Join(work, "a.md")); string(b) != "# A, edited but not committed" {
		t.Errorf("local edit lost: %q", b)
	}
}

func TestChangesDoesNotStage(t *testing.T) {
	ctx := context.Background()
	work, _ := repoWithRemote(t)
	g := New(work)
	write(t, work, "keep.md", "# Keep")
	write(t, work, "gone.md", "# Gone")
	write(t, work, "edit.md", "# Edit")
	_ = g.AddAll(ctx)
	if _, err := g.Commit(ctx, "first", ""); err != nil {
		t.Fatal(err)
	}

	write(t, work, "edit.md", "# Edited")
	write(t, work, "sub dir/fresh é.md", "# Fresh")
	if err := os.Remove(filepath.Join(work, "gone.md")); err != nil {
		t.Fatal(err)
	}
	got, err := g.Changes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := "M\tedit.md,D\tgone.md,A\tsub dir/fresh é.md"
	if strings.Join(got, ",") != want {
		t.Errorf("changes = %q, want %q", got, want)
	}
	if staged, _ := g.StagedChanges(ctx); len(staged) != 0 {
		t.Errorf("Changes must not stage anything: %v", staged)
	}
}
