// Package e2e builds the real noot binary once and drives it as a user would:
// separate processes, real files, a throwaway config. Runs as part of `go test ./...`.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	bin      string // path to the built binary
	cacheDir string // shared wazero compilation cache, so SQLite compiles once per run
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "noot-e2e-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bin = filepath.Join(dir, "noot")
	cacheDir = filepath.Join(dir, "cache")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "../cmd/noot")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building noot:", err)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// env is one isolated "machine": its own config file and working directory.
type env struct {
	t      *testing.T
	root   string // temp dir holding vaults
	config string
	cwd    string
	extra  []string // extra environment variables (KEY=VALUE)
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	return &env{t: t, root: root, config: filepath.Join(root, "config.yaml"), cwd: root}
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
}

func (e *env) environ() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "NOOT_") || strings.HasPrefix(kv, "EDITOR=") || strings.HasPrefix(kv, "VISUAL=") {
			continue
		}
		out = append(out, kv)
	}
	return append(append(out, "NOOT_CONFIG="+e.config, "NOOT_CACHE_DIR="+cacheDir), e.extra...)
}

// run executes noot with stdin and returns its output; it never fails the test itself.
func (e *env) runIn(stdin string, args ...string) result {
	e.t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = e.cwd
	cmd.Env = e.environ()
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		e.t.Fatalf("running noot %v: %v", args, err)
	}
	return result{out.String(), errb.String(), code}
}

func (e *env) run(args ...string) result { e.t.Helper(); return e.runIn("", args...) }

// ok runs a command that must succeed and returns stdout.
func (e *env) ok(args ...string) string {
	e.t.Helper()
	r := e.run(args...)
	if r.code != 0 {
		e.t.Fatalf("noot %s failed:\n%s", strings.Join(args, " "), r)
	}
	return r.stdout
}

func (e *env) path(rel string) string { return filepath.Join(e.root, filepath.FromSlash(rel)) }

func (e *env) write(rel, content string) {
	e.t.Helper()
	p := e.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) read(rel string) string {
	e.t.Helper()
	b, err := os.ReadFile(e.path(rel))
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b)
}

func contains(t *testing.T, what, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s: missing %q in:\n%s", what, w, got)
		}
	}
}

func lacks(t *testing.T, what, got string, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(got, n) {
			t.Errorf("%s: unexpected %q in:\n%s", what, n, got)
		}
	}
}

// ---------------------------------------------------------------- tests --

func TestInitAndRegisterTwoVaults(t *testing.T) {
	e := newEnv(t)
	contains(t, "init personal", e.ok("init", e.path("Patrick"), "--name", "patrick"), "personal vault ready")
	contains(t, "init agent", e.ok("init", e.path("Claude"), "--name", "claude", "--agent"), "agent vault ready")

	for _, f := range []string{"Patrick/journal", "Patrick/zettels", "Patrick/.gitignore",
		"Claude/AGENTS.md", "Claude/memory/index.md", "Claude/logs", "Claude/decisions"} {
		if _, err := os.Stat(e.path(f)); err != nil {
			t.Errorf("init did not create %s", f)
		}
	}
	contains(t, "gitignore", e.read("Patrick/.gitignore"), ".noot/")

	list := e.ok("vault", "list")
	contains(t, "vault list", list, "* patrick", "A claude", "agent", "personal")

	var cfg struct {
		Default, Agent string
		Vaults         map[string]struct{ Path, Kind string }
	}
	if err := json.Unmarshal([]byte(e.ok("vault", "list", "--json")), &cfg); err != nil {
		t.Fatalf("vault list --json: %v", err)
	}
	if cfg.Default != "patrick" || cfg.Agent != "claude" || cfg.Vaults["claude"].Kind != "agent" {
		t.Errorf("config = %+v", cfg)
	}
}

func TestInitAlwaysRegisters(t *testing.T) {
	e := newEnv(t)
	contains(t, "init without --name", e.ok("init", e.path("home/My Notes")), `"my-notes"`)
	contains(t, "init --agent without --name", e.ok("init", e.path("home/Claude"), "--agent"), `"claude"`)
	contains(t, "vault list", e.ok("vault", "list"), "* my-notes", "A claude")

	// A vault in cwd, and re-running init keeps the name.
	e.cwd = e.path("home/Claude")
	contains(t, "init in cwd", e.ok("init", "--agent"), `"claude"`)

	// The same folder name elsewhere must not overwrite the first vault, and
	// nothing is created for it.
	e.cwd = e.root
	r := e.run("init", e.path("work/Claude"), "--agent")
	if r.code == 0 || !strings.Contains(r.stderr, `"claude" is already used`) {
		t.Errorf("taken name:\n%s", r)
	}
	if _, err := os.Stat(e.path("work/Claude")); err == nil {
		t.Error("init created the folder although the name was taken")
	}
	contains(t, "with --name it works", e.ok("init", e.path("work/Claude"), "--agent", "--name", "work-claude"), `"work-claude"`)
	list := e.ok("vault", "list")
	contains(t, "both registered", list, "A claude", e.path("home/Claude"), "work-claude", e.path("work/Claude"))
}

func TestVaultManagement(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("a"))
	e.ok("init", e.path("b"))
	e.ok("vault", "add", "a", e.path("a"))
	e.ok("vault", "add", "b", e.path("b"), "--kind", "agent")
	e.ok("vault", "default", "b")
	contains(t, "after default", e.ok("vault", "list"), "* b")

	e.ok("vault", "remove", "b")
	lacks(t, "after remove", e.ok("vault", "list"), " b ")
	if _, err := os.Stat(e.path("b")); err != nil {
		t.Error("vault remove must not delete files")
	}

	for _, bad := range [][]string{
		{"vault", "add", "x", e.path("missing")},
		{"vault", "add", "x", e.path("a"), "--kind", "robot"},
		{"vault", "default", "nope"},
		{"vault", "remove", "nope"},
	} {
		if r := e.run(bad...); r.code == 0 {
			t.Errorf("noot %v should fail:\n%s", bad, r)
		}
	}
}

func TestVaultSelection(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("me"), "--name", "me")
	e.ok("init", e.path("bot"), "--name", "bot", "--agent")
	e.write("me/zettels/mine.md", "# Mine\nonly in me")
	e.write("bot/scratch/theirs.md", "# Theirs\nonly in bot")

	// Default vault when cwd is outside any vault.
	contains(t, "default", e.ok("search", "only"), "zettels/mine.md")
	// -A selects the agent vault, -V a name or a path.
	contains(t, "-A", e.ok("-A", "search", "only"), "scratch/theirs.md")
	contains(t, "-V name", e.ok("-V", "bot", "search", "only"), "scratch/theirs.md")
	contains(t, "-V path", e.ok("-V", e.path("bot"), "search", "only"), "scratch/theirs.md")

	// A cwd inside a vault wins over the default.
	e.cwd = e.path("bot/scratch")
	contains(t, "cwd", e.ok("search", "only"), "scratch/theirs.md")

	// $NOOT_VAULT wins over cwd.
	e.extra = []string{"NOOT_VAULT=me"}
	contains(t, "NOOT_VAULT", e.ok("search", "only"), "zettels/mine.md")

	r := e.run("-V", "nope", "tags")
	if r.code == 0 || !strings.Contains(r.stderr, `"nope" is neither a registered vault`) {
		t.Errorf("unknown vault:\n%s", r)
	}
}

func TestLinksTagsAndSearch(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")
	e.write("v/projects/api.md", "---\nid: api\ntitle: Gasolina API\ntags: [go, project]\naliases: [backend]\n---\nSync for [[app]]. Config in `config.json`. #go/fiber\n")
	e.write("v/projects/app.md", "# App\nTalks to [[backend|the API]] and [[missing-note]]. Café #flutter\n")
	e.write("v/zettels/lonely.md", "# Lonely\nNo links here.\n")
	e.write("v/zettels/code.md", "# Code\n```go\n// [[not-a-link]] #nottag\n```\n")

	bl := e.ok("backlinks", "Gasolina API")
	contains(t, "backlinks", bl, "1 backlinks to projects/api.md", "projects/app.md", "[[backend|the API]]")

	contains(t, "backlinks context keeps code", e.ok("backlinks", "app"), "`config.json`")

	links := e.ok("links", "app")
	contains(t, "links", links, "✓ backend", "projects/api.md", "✗ missing-note")

	contains(t, "broken", e.ok("broken"), "projects/app.md", "missing-note")
	lacks(t, "broken ignores code", e.ok("broken"), "not-a-link")

	orphans := e.ok("orphans")
	contains(t, "orphans", orphans, "zettels/lonely.md", "zettels/code.md")
	lacks(t, "orphans", orphans, "projects/api.md", "projects/app.md")

	tags := e.ok("tags")
	contains(t, "tags", tags, "#go", "#go/fiber", "#flutter", "#project")
	lacks(t, "tags ignores code", tags, "#nottag")
	contains(t, "tag with children", e.ok("tags", "go"), "projects/api.md")

	contains(t, "accent-insensitive search", e.ok("search", "cafe"), "projects/app.md")
	contains(t, "prefix search", e.ok("search", "gaso"), "projects/api.md")
	contains(t, "tag filter", e.ok("search", "--tag", "flutter"), "projects/app.md")
	lacks(t, "tag filter", e.ok("search", "--tag", "flutter"), "projects/api.md")
	contains(t, "raw FTS5", e.ok("search", "--raw", "sync OR café"), "projects/api.md", "projects/app.md")
	contains(t, "no matches", e.ok("search", "zzzzzz"), "no matches")

	var hits []struct{ Path, Title string }
	if err := json.Unmarshal([]byte(e.ok("search", "--json", "sync")), &hits); err != nil || len(hits) != 1 || hits[0].Title != "Gasolina API" {
		t.Errorf("search --json = %+v, %v", hits, err)
	}

	open := strings.TrimSpace(e.ok("open", "--print", "backend"))
	if open != e.path("v/projects/api.md") {
		t.Errorf("open --print = %q", open)
	}
	if r := e.run("open", "--print", "does-not-exist"); r.code == 0 {
		t.Errorf("open of unknown note should fail:\n%s", r)
	}
}

func TestNewTodayLogCatAppend(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("me"), "--name", "me")
	e.ok("init", e.path("bot"), "--name", "bot", "--agent")

	rel := strings.TrimSpace(e.ok("new", "Wat is een Zettel?", "-t", "idee", "-d", "zettels"))
	if rel != "zettels/wat-is-een-zettel.md" {
		t.Fatalf("new = %q", rel)
	}
	contains(t, "new note", e.read("me/"+rel), "title: Wat is een Zettel?", "tags: [idee]", "# Wat is een Zettel?")
	if again := strings.TrimSpace(e.ok("new", "Wat is een Zettel?")); again != "zettels/wat-is-een-zettel-2.md" {
		t.Errorf("duplicate title = %q", again)
	}

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if got := strings.TrimSpace(e.ok("today", "--print", "--offset", "-1")); got != "journal/"+yesterday+".md" {
		t.Errorf("today --offset -1 = %q", got)
	}
	contains(t, "journal", e.read("me/journal/"+yesterday+".md"), "tags: [journal]")

	logRel := strings.TrimSpace(e.ok("-A", "log", "-p", "noot", "-t", "release", "first entry"))
	if r := e.runIn("second entry from stdin\n", "-A", "log"); r.code != 0 {
		t.Fatalf("log from stdin:\n%s", r)
	}
	logText := e.read("bot/" + logRel)
	contains(t, "log", logText, "· [[noot]] #release", "first entry", "second entry from stdin")
	if strings.Count(logText, "title: Log") != 1 {
		t.Errorf("log frontmatter written twice:\n%s", logText)
	}
	if r := e.run("-A", "log"); r.code == 0 {
		t.Errorf("empty log entry should fail:\n%s", r)
	}

	// Text starting with "-" is text, not a flag.
	e.ok("-A", "append", "--create", "projects/noot", "- [ ] write e2e tests")
	e.ok("-A", "append", "noot", "- [ ] second item")
	contains(t, "append", e.read("bot/projects/noot.md"), "- [ ] write e2e tests\n- [ ] second item\n")
	if r := e.run("-A", "append", "unknown-note", "x"); r.code == 0 {
		t.Errorf("append without --create to an unknown note should fail:\n%s", r)
	}

	contains(t, "cat", e.ok("-A", "cat", "noot"), "- [ ] second item")
	var doc struct{ Path, Content string }
	if err := json.Unmarshal([]byte(e.ok("-A", "cat", "--json", "noot")), &doc); err != nil || doc.Path != "projects/noot.md" {
		t.Errorf("cat --json = %+v, %v", doc, err)
	}
	contains(t, "log links project", e.ok("-A", "backlinks", "noot"), logRel)
}

func TestSearchAllVaults(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("me"), "--name", "me")
	e.ok("init", e.path("bot"), "--name", "bot", "--agent")
	e.write("me/zettels/sqlite.md", "# SQLite tips\nUse WAL.")
	e.write("bot/memory/db.md", "# DB memory\nsqlite FTS5 is in use.")

	out := e.ok("search", "--all", "sqlite")
	contains(t, "search --all", out, "[me] zettels/sqlite.md", "[bot] memory/db.md")

	var res []struct {
		Vault string
		Hits  []struct{ Path string }
	}
	if err := json.Unmarshal([]byte(e.ok("search", "--all", "--json", "sqlite")), &res); err != nil || len(res) != 2 {
		t.Fatalf("search --all --json = %+v, %v", res, err)
	}
}

func TestIncrementalIndex(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")
	e.write("v/a.md", "# A")
	e.write("v/b.md", "# B")
	contains(t, "first index", e.ok("index"), "2 notes (2 updated, 0 removed")
	contains(t, "no changes", e.ok("index"), "2 notes (0 updated, 0 removed")

	e.write("v/a.md", "# A changed, longer")
	contains(t, "changed", e.ok("index"), "(1 updated, 0 removed")
	if err := os.Remove(e.path("v/b.md")); err != nil {
		t.Fatal(err)
	}
	contains(t, "removed", e.ok("index"), "1 notes (0 updated, 1 removed")
	contains(t, "full", e.ok("index", "--full"), "1 notes (1 updated")

	// Duplicate ids are reported, not fatal.
	e.write("v/one.md", "---\nid: same\n---\n")
	e.write("v/two.md", "---\nid: same\n---\n")
	r := e.run("index")
	if r.code != 0 || !strings.Contains(r.stderr, `id "same"`) || !strings.Contains(r.stderr, "already used") {
		t.Errorf("duplicate id:\n%s", r)
	}
	// Broken frontmatter is a warning too.
	e.write("v/bad.md", "---\ntitle: [unclosed\n---\n")
	if r := e.run("index"); r.code != 0 || !strings.Contains(r.stderr, "bad.md") {
		t.Errorf("bad frontmatter:\n%s", r)
	}
}

func TestNestedVaultIsSkipped(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("Noot"))
	e.ok("init", e.path("Noot/Claude"))
	e.write("Noot/top.md", "# Top")
	e.write("Noot/Claude/inner.md", "# Inner")

	contains(t, "parent", e.ok("-V", e.path("Noot"), "index"), "1 notes")
	contains(t, "child", e.ok("-V", e.path("Noot/Claude"), "index"), "1 notes")
}

func TestWatch(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")

	cmd := exec.Command(bin, "-V", "v", "watch")
	cmd.Dir, cmd.Env = e.cwd, e.environ()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	waitFor(t, "watcher start", func() bool { return strings.Contains(out.String(), "watching") })
	e.write("v/fresh/new-dir-note.md", "# Watched\nkeyword-zebra")
	waitFor(t, "watched file indexed", func() bool {
		return strings.Contains(e.run("-V", "v", "--no-sync", "search", "keyword-zebra").stdout, "fresh/new-dir-note.md")
	})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestCompilationCacheIsUsed(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"))
	e.ok("-V", e.path("v"), "tags")
	entries, err := os.ReadDir(cacheDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("wazero cache %s is empty (%v): every command would recompile SQLite", cacheDir, err)
	}
	start := time.Now()
	e.ok("-V", e.path("v"), "tags")
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("warm command took %s; the compilation cache does not seem to work", d)
	}
}

func TestVersionAndHelp(t *testing.T) {
	e := newEnv(t)
	contains(t, "version", e.ok("--version"), "noot version ")
	contains(t, "help", e.ok("--help"), "search", "backlinks", "vault", "log")
	if r := e.run("no-such-command"); r.code == 0 {
		t.Errorf("unknown command should fail:\n%s", r)
	}
}

// gitRepo turns dir into a git repo with a bare "origin", isolated from the
// user's git config, and returns the bare repo path.
func (e *env) gitRepo(dir string) string {
	e.t.Helper()
	e.extra = append(e.extra,
		"GIT_CONFIG_GLOBAL="+filepath.Join(e.root, "gitconfig"), "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	bare := e.path(filepath.Base(dir) + "-remote.git")
	e.git(e.root, "init", "-q", "--bare", "-b", "main", bare)
	e.git(dir, "init", "-q", "-b", "main")
	e.git(dir, "remote", "add", "origin", bare)
	return bare
}

func (e *env) git(dir string, args ...string) string {
	e.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, e.environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestPush(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")
	bare := e.gitRepo(e.path("v"))
	e.write("v/zettels/a.md", "# A\n[[b]]")
	e.write("v/zettels/b.md", "# B")
	e.ok("-V", "v", "index") // creates .noot/index.db, which must not be committed

	// First push: commit with tree-hash message, set upstream.
	out := e.ok("-V", "v", "push")
	contains(t, "first push", out, "committed", "pushed")
	subject := e.git(bare, "log", "-1", "--format=%s", "main")
	if !strings.HasPrefix(subject, "noot: ") || len(subject) != len("noot: ")+12 {
		t.Errorf("subject = %q, want noot: <12-char hash>", subject)
	}
	files := e.git(bare, "ls-tree", "-r", "--name-only", "main")
	contains(t, "pushed files", files, "zettels/a.md", "zettels/b.md", ".gitignore")
	lacks(t, "pushed files", files, ".noot/")
	body := e.git(bare, "log", "-1", "--format=%b", "main")
	contains(t, "body lists changes", body, "A\tzettels/a.md")

	// Nothing changed: nothing to do, and no network: it must work even
	// when the remote is unreachable.
	if err := os.Rename(bare, bare+".offline"); err != nil {
		t.Fatal(err)
	}
	out = e.ok("-V", "v", "push")
	contains(t, "no changes", out, "nothing to push")
	lacks(t, "no changes", out, "pulled")
	if err := os.Rename(bare+".offline", bare); err != nil {
		t.Fatal(err)
	}

	// Custom message, and the agent vault selection works too.
	e.write("v/zettels/a.md", "# A changed")
	e.ok("-V", "v", "push", "-m", "Weekly notes")
	if s := e.git(bare, "log", "-1", "--format=%s", "main"); s != "Weekly notes" {
		t.Errorf("custom subject = %q", s)
	}

	// Another machine pushed in the meantime: noot pulls (rebase) first.
	other := e.path("other")
	e.git(e.root, "clone", "-q", bare, other)
	e.write("other/zettels/from-other.md", "# From the other machine\nkeyword-otter")
	e.git(other, "add", "-A")
	e.git(other, "commit", "-q", "-m", "other machine")
	e.git(other, "push", "-q")
	e.write("v/zettels/local.md", "# Local")
	contains(t, "push after remote change", e.ok("-V", "v", "push"), "pulled", "pushed")
	files = e.git(bare, "ls-tree", "-r", "--name-only", "main")
	contains(t, "both changes on remote", files, "zettels/from-other.md", "zettels/local.md")
	// The index was updated with the pulled note.
	contains(t, "pulled note searchable", e.ok("-V", "v", "--no-sync", "search", "keyword-otter"), "zettels/from-other.md")

	var res struct {
		Commit string
		Pushed bool
	}
	e.write("v/zettels/json.md", "# JSON")
	if err := json.Unmarshal([]byte(e.ok("-V", "v", "push", "--json")), &res); err != nil || res.Commit == "" || !res.Pushed {
		t.Errorf("push --json = %+v, %v", res, err)
	}
}

func TestPushWithoutRemoteOrRepo(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")
	r := e.run("-V", "v", "push")
	if r.code == 0 || !strings.Contains(r.stderr, "not a git repository") {
		t.Errorf("push outside a repo:\n%s", r)
	}

	e.gitRepo(e.path("v"))
	e.git(e.path("v"), "remote", "remove", "origin")
	e.write("v/a.md", "# A")
	out := e.ok("-V", "v", "push")
	contains(t, "local only", out, "committed", "no remote configured")
	if log := e.git(e.path("v"), "log", "--oneline"); !strings.Contains(log, "noot: ") {
		t.Errorf("expected a local commit, log = %q", log)
	}
}

func TestPull(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")
	bare := e.gitRepo(e.path("v"))
	e.write("v/a.md", "# A")
	e.ok("-V", "v", "push")

	contains(t, "up to date", e.ok("-V", "v", "pull"), "already up to date")

	other := e.path("other")
	e.git(e.root, "clone", "-q", bare, other)
	e.write("other/zettels/remote.md", "# Remote\nkeyword-heron")
	e.git(other, "add", "-A")
	e.git(other, "commit", "-q", "-m", "other machine")
	e.git(other, "push", "-q")

	e.write("v/a.md", "# A, local edit")
	contains(t, "pull", e.ok("-V", "v", "pull"), "pulled", "index updated")
	contains(t, "pulled note indexed", e.ok("-V", "v", "--no-sync", "search", "keyword-heron"), "zettels/remote.md")
	if got := e.read("v/a.md"); got != "# A, local edit" {
		t.Errorf("local edit lost: %q", got)
	}

	e2 := newEnv(t)
	e2.ok("init", e2.path("plain"), "--name", "plain")
	if r := e2.run("-V", "plain", "pull"); r.code == 0 || !strings.Contains(r.stderr, "not a git repository") {
		t.Errorf("pull outside a repo:\n%s", r)
	}
}

func TestContext(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("bot"), "--name", "bot", "--agent")
	e.write("bot/projects/noot/noot.md", "---\ntitle: noot\n---\n# noot\n\nStatus: green.\n")
	e.write("bot/projects/noot/serve-plan.md", "# Serve plan\n")
	e.write("bot/decisions/2026-10-24-vaults.md", "# Separate vaults\n\nApplies to [[noot]].\n")
	e.ok("-A", "log", "-p", "noot", "Template updated")
	e.ok("-A", "log", "-p", "other", "Unrelated work")

	out := e.ok("-A", "context", "noot")
	contains(t, "context", out,
		"==> projects/noot/noot.md (noot)", "Status: green.",
		"==> related", "decisions/2026-10-24-vaults.md", "Applies to [[noot]].", "projects/noot/serve-plan.md",
		"==> log, last 7 days", "· [[noot]]", "Template updated")
	lacks(t, "context", out, "Unrelated work")

	var nc struct {
		Note    struct{ Path string }
		Content string
		Related []struct{ From struct{ Path string } }
		Logs    []struct{ Path, Heading, Text string }
	}
	if err := json.Unmarshal([]byte(e.ok("-A", "context", "--json", "noot")), &nc); err != nil {
		t.Fatal(err)
	}
	if nc.Note.Path != "projects/noot/noot.md" || len(nc.Related) != 2 || len(nc.Logs) != 1 || nc.Logs[0].Text != "Template updated" {
		t.Errorf("context --json = %+v", nc)
	}

	if r := e.run("-A", "context", "missing"); r.code == 0 {
		t.Errorf("context of an unknown note should fail:\n%s", r)
	}
}

func TestRename(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("me"), "--name", "me")
	e.write("me/projects/noot.md", "---\ntitle: noot\n---\n# noot\n\nSee [vaults](../decisions/vaults.md).\n")
	e.write("me/decisions/vaults.md", "# Vaults\n\nFor [[projects/noot|noot]] and [main](../projects/noot.md).\n")

	dry := e.ok("rename", "noot", "projects/noot/noot", "--dry-run")
	contains(t, "dry run", dry, "would rename projects/noot.md → projects/noot/noot.md, 3 links in 2 files", "decisions/vaults.md")
	contains(t, "dry run leaves files", e.read("me/decisions/vaults.md"), "[[projects/noot|noot]]")

	contains(t, "rename", e.ok("rename", "noot", "projects/noot/noot"), "renamed projects/noot.md → projects/noot/noot.md")
	contains(t, "rewritten", e.read("me/decisions/vaults.md"), "[[projects/noot/noot|noot]] and [main](../projects/noot/noot.md)")
	contains(t, "moved", e.read("me/projects/noot/noot.md"), "[vaults](../../decisions/vaults.md)")
	if out := e.ok("broken"); strings.TrimSpace(out) != "" {
		t.Errorf("broken links after rename:\n%s", out)
	}

	var res struct {
		To    string
		Links int
	}
	if err := json.Unmarshal([]byte(e.ok("rename", "--json", "noot", "--title", "noot CLI")), &res); err != nil ||
		res.To != "projects/noot/noot.md" || res.Links != 0 {
		t.Errorf("rename --json = %+v, %v", res, err)
	}
	contains(t, "title", e.read("me/projects/noot/noot.md"), "title: noot CLI", "# noot CLI")

	if r := e.run("rename", "vaults", "projects/noot/noot"); r.code == 0 {
		t.Errorf("renaming onto an existing note should fail:\n%s", r)
	}
}

func TestStatus(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")
	e.ok("init", e.path("bot"), "--name", "bot", "--agent")
	e.gitRepo(e.path("v"))
	e.gitRepo(e.path("bot"))

	e.write("v/zettels/a.md", "# A")
	out := e.ok("-V", "v", "status")
	contains(t, "before first push", out, "A  zettels/a.md", "no upstream yet")
	lacks(t, "index is never pushed", out, ".noot")

	e.ok("-V", "v", "push")
	contains(t, "after push", e.ok("-V", "v", "status"), "nothing to push")

	e.write("v/zettels/a.md", "# A changed")
	e.git(e.path("v"), "commit", "-q", "-am", "local only")
	e.write("v/zettels/b.md", "# B")
	contains(t, "unpushed commit", e.ok("-V", "v", "status"), "A  zettels/b.md", "1 commit not pushed yet")

	// The agent vault, and --json.
	e.write("bot/logs/2026-10-25.md", "# Log")
	contains(t, "agent vault", e.ok("-A", "status"), "logs/2026-10-25.md")
	var st struct {
		Changes []struct{ Status, Path string }
		Ahead   int
	}
	if err := json.Unmarshal([]byte(e.ok("-A", "status", "--json")), &st); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, c := range st.Changes {
		paths = append(paths, c.Status+" "+c.Path)
	}
	contains(t, "status --json", strings.Join(paths, "\n"), "A logs/2026-10-25.md", "A AGENTS.md")
}

func TestAgenda(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("me"), "--name", "me")

	y, w := time.Now().ISOWeek()
	rel := strings.TrimSpace(e.ok("agenda", "--print"))
	if want := fmt.Sprintf("agenda/%d-W%02d.md", y, w); rel != want {
		t.Fatalf("agenda --print = %q, want %q", rel, want)
	}
	contains(t, "agenda", e.read("me/"+rel), "tags: [agenda]", fmt.Sprintf("# Week %d, %d", w, y), "## Monday ", "## Sunday ", "### Planned\n\n### Doing\n\n### Done\n")

	ny, nw := time.Now().AddDate(0, 0, 7).ISOWeek()
	if got := strings.TrimSpace(e.ok("agenda", "--print", "--offset", "1")); got != fmt.Sprintf("agenda/%d-W%02d.md", ny, nw) {
		t.Errorf("agenda --offset 1 = %q", got)
	}
	contains(t, "agenda is a note", e.ok("tags", "agenda"), rel)
}

func TestReminders(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("me"), "--name", "me")

	contains(t, "empty list", e.ok("reminder"), "no open reminders")
	contains(t, "add", e.ok("reminder", "Call the dentist", "--due", "2020-01-01"), "added 1: Call the dentist")
	contains(t, "add via subcommand", e.ok("rem", "add", "Buy", "milk"), "added 2: Buy milk")
	contains(t, "add from stdin", e.runIn("Water the plants\n", "reminder", "add").stdout, "added 3: Water the plants")
	contains(t, "reminders.md", e.read("me/reminders.md"), "tags: [reminders]",
		"- [ ] Call the dentist · due 2020-01-01\n- [ ] Buy milk\n- [ ] Water the plants\n")

	list := e.ok("reminder", "list")
	contains(t, "list", list, "1  [ ] Call the dentist  due 2020-01-01 (overdue)", "2  [ ] Buy milk", "3  [ ] Water the plants")

	contains(t, "done", e.ok("reminder", "done", "1", "#3"), "done 1: Call the dentist", "done 3: Water the plants")
	list = e.ok("reminder")
	contains(t, "open after done", list, "Buy milk")
	lacks(t, "open after done", list, "dentist", "plants")
	contains(t, "--all", e.ok("reminder", "--all"), "[x] Call the dentist", "done "+time.Now().Format("2006-01-02"))
	if r := e.run("reminder", "done", "1"); r.code == 0 || !strings.Contains(r.stderr, "already done") {
		t.Errorf("done twice:\n%s", r)
	}
	if r := e.run("reminder", "done", "x"); r.code == 0 {
		t.Errorf("done x should fail:\n%s", r)
	}

	tomorrow := time.Now().AddDate(0, 0, 1)
	y, w := tomorrow.ISOWeek()
	agenda := fmt.Sprintf("agenda/%d-W%02d.md", y, w)
	contains(t, "plan", e.ok("reminder", "plan", "2", "tomorrow"), "planned 2 on "+tomorrow.Format("Monday 2 January"), agenda)
	contains(t, "agenda", e.read("me/"+agenda), "## "+tomorrow.Format("Monday 2 January")+"\n\n### Planned\n\n- [ ] Buy milk ([[reminders]])\n\n### Doing")
	contains(t, "planned in list", e.ok("reminder"), "planned "+tomorrow.Format("2006-01-02"))
	contains(t, "agenda links back", e.ok("backlinks", "reminders"), agenda)

	e.ok("reminder", "done", "2")
	contains(t, "agenda ticked", e.read("me/"+agenda), "- [x] Buy milk ([[reminders]])")

	var rs []struct {
		N    int    `json:"n"`
		Text string `json:"text"`
		Done bool   `json:"done"`
	}
	if err := json.Unmarshal([]byte(e.ok("reminder", "--all", "--json")), &rs); err != nil || len(rs) != 3 || !rs[1].Done {
		t.Errorf("reminder --json = %+v, %v", rs, err)
	}
}

func TestNewNotesGetUniqueIDs(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")

	// Notes created in the same second used to share a timestamp id, after
	// which the index kept the first and skipped the rest. Ids are ULIDs now.
	ids := map[string]string{}
	var inOrder []string
	for _, title := range []string{"Een", "Twee", "Drie", "Vier"} {
		rel := strings.TrimSpace(e.ok("new", title))
		id := frontmatterID(t, e.read("v/"+rel))
		if other, dup := ids[id]; dup {
			t.Fatalf("%s and %s share id %s", other, rel, id)
		}
		ids[id] = rel
		inOrder = append(inOrder, id)
	}
	if !sort.StringsAreSorted(inOrder) {
		t.Errorf("ids are not in creation order:\n%s", strings.Join(inOrder, "\n"))
	}
	if r := e.run("index"); r.code != 0 || strings.Contains(r.stderr, "already used") {
		t.Errorf("index after four new notes:\n%s", r)
	}

	// A note is still reachable by its id, whatever the id looks like.
	for id, rel := range ids {
		if got := strings.TrimSpace(e.ok("open", id, "--print")); got != e.path("v/"+rel) {
			t.Errorf("open %s --print = %q, want %q", id, got, e.path("v/"+rel))
		}
	}
}

// frontmatterID returns the id: value from a note's frontmatter.
func frontmatterID(t *testing.T, note string) string {
	t.Helper()
	for _, line := range strings.Split(note, "\n") {
		if rest, ok := strings.CutPrefix(line, "id:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	t.Fatalf("no id in frontmatter:\n%s", note)
	return ""
}

func TestServe(t *testing.T) {
	e := newEnv(t)
	e.ok("init", e.path("v"), "--name", "v")
	e.write("v/zettels/bestaand.md", "# Bestaand\nkeyword-heron, zie [[Nieuw idee]]")

	cmd := exec.Command(bin, "-V", "v", "serve", "--addr", "127.0.0.1:0")
	cmd.Dir, cmd.Env = e.cwd, e.environ()
	var out syncBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	addr := regexp.MustCompile(`http://127\.0\.0\.1:\d+`)
	waitFor(t, "server start", func() bool { return addr.MatchString(out.String()) })
	base := addr.FindString(out.String())

	get := func(path string) (int, string) {
		t.Helper()
		res, err := http.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}

	if code, body := get("/"); code != http.StatusOK || !strings.Contains(body, "noot") {
		t.Errorf("GET / = %d\n%s", code, body)
	}
	if code, body := get("/api/search?q=heron"); code != http.StatusOK || !strings.Contains(body, "zettels/bestaand.md") {
		t.Errorf("search existing = %d %s", code, body)
	}

	res, err := http.Post(base+"/api/notes", "application/json", strings.NewReader(`{"title":"Nieuw idee","body":"keyword-osprey"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated || !strings.Contains(string(b), `"path":"zettels/nieuw-idee.md"`) {
		t.Fatalf("create = %d %s", res.StatusCode, b)
	}
	contains(t, "created file", e.read("v/zettels/nieuw-idee.md"), "title: Nieuw idee", "keyword-osprey")

	if code, body := get("/api/search?q=osprey"); code != http.StatusOK || !strings.Contains(body, "zettels/nieuw-idee.md") {
		t.Errorf("search created = %d %s", code, body)
	}
	if code, body := get("/api/notes/Nieuw%20idee"); code != http.StatusOK || !strings.Contains(body, "keyword-osprey") {
		t.Errorf("read created = %d %s", code, body)
	}
	if code, body := get("/api/notes/zettels%2Fbestaand.md"); code != http.StatusOK || !strings.Contains(body, `href=\"#/note/Nieuw%20idee\"`) {
		t.Errorf("read rendered link = %d %s", code, body)
	}
	contains(t, "CLI sees the note", e.ok("-V", "v", "search", "osprey"), "zettels/nieuw-idee.md")

	// Files changed outside the server show up without a restart.
	e.write("v/zettels/extern.md", "# Extern\nkeyword-kestrel")
	waitFor(t, "watched file searchable", func() bool {
		_, body := get("/api/search?q=kestrel")
		return strings.Contains(body, "zettels/extern.md")
	})

	// A foreign Host header (DNS rebinding) is refused.
	req, _ := http.NewRequest("GET", base+"/api/search?q=heron", nil)
	req.Host = "evil.example"
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != http.StatusForbidden {
		t.Errorf("foreign host: %v %v", res, err)
	} else {
		res.Body.Close()
	}
}

// syncBuffer is a bytes.Buffer that a running process and the test may use at once.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
