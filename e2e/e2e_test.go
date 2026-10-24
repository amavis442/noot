// Package e2e builds the real noot binary once and drives it as a user would:
// separate processes, real files, a throwaway config. Runs as part of `go test ./...`.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
