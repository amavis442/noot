package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/vault"
)

// fakeGit records calls and returns canned answers.
type fakeGit struct {
	repo     bool
	staged   []string
	changes  []string
	tree     string
	upstream bool
	remote   bool
	ahead    int
	pullErr  error

	head     string
	pullHead string // HEAD after a pull; "" = unchanged

	calls   []string
	subject string
	body    string
}

func (f *fakeGit) record(c string) { f.calls = append(f.calls, c) }

func (f *fakeGit) IsRepo(context.Context) (bool, error) { f.record("is-repo"); return f.repo, nil }
func (f *fakeGit) AddAll(context.Context) error         { f.record("add"); return nil }
func (f *fakeGit) StagedChanges(context.Context) ([]string, error) {
	f.record("staged")
	return f.staged, nil
}
func (f *fakeGit) Changes(context.Context) ([]string, error) {
	f.record("changes")
	return f.changes, nil
}
func (f *fakeGit) WriteTree(context.Context) (string, error) {
	f.record("write-tree")
	return f.tree, nil
}
func (f *fakeGit) Commit(_ context.Context, subject, body string) (string, error) {
	f.record("commit")
	f.subject, f.body = subject, body
	f.ahead++
	return "abc1234", nil
}
func (f *fakeGit) HasUpstream(context.Context) (bool, error) {
	f.record("has-upstream")
	return f.upstream, nil
}
func (f *fakeGit) HasRemote(_ context.Context, name string) (bool, error) {
	f.record("has-remote " + name)
	return f.remote, nil
}
func (f *fakeGit) PullRebase(context.Context) error {
	f.record("pull")
	if f.pullErr == nil && f.pullHead != "" {
		f.head = f.pullHead
	}
	return f.pullErr
}
func (f *fakeGit) Head(context.Context) (string, error) { f.record("head"); return f.head, nil }
func (f *fakeGit) Ahead(context.Context) (int, error) {
	f.record("ahead")
	return f.ahead, nil
}
func (f *fakeGit) Push(context.Context) error { f.record("push"); f.ahead = 0; return nil }
func (f *fakeGit) PushSetUpstream(_ context.Context, remote string) error {
	f.record("push -u " + remote)
	f.ahead = 0
	return nil
}

func publishSetup(t *testing.T, g *fakeGit) *app.Service {
	t.Helper()
	svc, _ := setup(t)
	svc.Git = g
	return svc
}

func TestPublishCommitsWithTreeHashAndPushes(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, staged: []string{"M\tzettels/a.md", "A\tlogs/2026-10-24.md"},
		tree: "3f9a1c2e4b7d0011223344556677889900aabbcc"}
	res, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if g.subject != "noot: 3f9a1c2e4b7d" {
		t.Errorf("subject = %q, want the short tree hash", g.subject)
	}
	if !strings.Contains(g.body, "M\tzettels/a.md") || !strings.Contains(g.body, "A\tlogs/2026-10-24.md") {
		t.Errorf("body should list the changed files, got %q", g.body)
	}
	want := []string{"is-repo", "add", "staged", "write-tree", "commit", "has-upstream", "ahead", "pull", "push"}
	if !slices.Equal(g.calls, want) {
		t.Errorf("calls = %v\nwant    %v", g.calls, want)
	}
	if res.Commit != "abc1234" || res.Files != 2 || !res.Pushed || !res.Pulled {
		t.Errorf("result = %+v", res)
	}
}

func TestPublishUsesCustomMessage(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, staged: []string{"M\ta.md"}, tree: "ffff"}
	if _, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{Message: "Weekly notes"}); err != nil {
		t.Fatal(err)
	}
	if g.subject != "Weekly notes" || slices.Contains(g.calls, "write-tree") {
		t.Errorf("subject = %q, calls = %v", g.subject, g.calls)
	}
}

func TestPublishNothingToDo(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true}
	res, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Commit != "" || res.Pushed || slices.Contains(g.calls, "commit") || slices.Contains(g.calls, "push") {
		t.Errorf("nothing to do, but result = %+v calls = %v", res, g.calls)
	}
	// Nothing to push means no network round trip at all.
	if slices.Contains(g.calls, "pull") || res.Pulled {
		t.Errorf("nothing to push must not pull: calls = %v", g.calls)
	}
}

func TestPublishPushesEarlierUnpushedCommits(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, ahead: 2}
	res, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{})
	if err != nil || !res.Pushed || !res.Pulled {
		t.Errorf("unpushed commits should be pulled over and pushed: %+v %v", res, err)
	}
}

func TestPublishWithoutUpstreamSetsIt(t *testing.T) {
	g := &fakeGit{repo: true, remote: true, staged: []string{"A\ta.md"}, tree: "abcdef1234567890"}
	res, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(g.calls, "push -u origin") || slices.Contains(g.calls, "pull") || !res.Pushed {
		t.Errorf("calls = %v result = %+v", g.calls, res)
	}
}

func TestPublishWithoutRemoteCommitsLocally(t *testing.T) {
	g := &fakeGit{repo: true, staged: []string{"A\ta.md"}, tree: "abcdef1234567890"}
	res, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Commit == "" || res.Pushed || res.Note == "" {
		t.Errorf("expected a local commit with a note about the missing remote, got %+v", res)
	}
}

func TestPublishNoPull(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, staged: []string{"M\ta.md"}, tree: "abcdef1234567890"}
	if _, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{NoPull: true}); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(g.calls, "pull") {
		t.Errorf("NoPull still pulled: %v", g.calls)
	}
}

func TestPublishPullFailureStopsBeforePush(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, staged: []string{"M\ta.md"}, tree: "abcdef1234567890",
		pullErr: errors.New("conflict in a.md")}
	res, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{})
	if err == nil || !strings.Contains(err.Error(), "conflict in a.md") {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(g.calls, "push") || res.Commit != "abc1234" {
		t.Errorf("must commit, then stop before push: calls = %v result = %+v", g.calls, res)
	}
}

func TestPublishRequiresRepo(t *testing.T) {
	g := &fakeGit{}
	if _, err := publishSetup(t, g).Publish(context.Background(), app.PublishOptions{}); err == nil ||
		!strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("err = %v", err)
	}
}

func TestPublishEnsuresIndexIsIgnored(t *testing.T) {
	g := &fakeGit{repo: true}
	svc := publishSetup(t, g)
	gi := filepath.Join(svc.Vault.Root, ".gitignore")
	if err := os.WriteFile(gi, []byte("*.tmp"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Publish(context.Background(), app.PublishOptions{}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(gi)
	if !strings.Contains(string(b), vault.MetaDir+"/") || !strings.HasPrefix(string(b), "*.tmp\n") {
		t.Errorf(".gitignore = %q", b)
	}
	if g.calls[1] != "add" {
		t.Errorf(".gitignore must be fixed before git add, calls = %v", g.calls)
	}
}

func TestPullUpdatesFromRemote(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, head: "aaa", pullHead: "bbb"}
	res, err := publishSetup(t, g).Pull(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.From != "aaa" || res.To != "bbb" {
		t.Errorf("result = %+v", res)
	}
	if want := []string{"is-repo", "has-upstream", "head", "pull", "head"}; !slices.Equal(g.calls, want) {
		t.Errorf("calls = %v\nwant    %v", g.calls, want)
	}
}

func TestPullAlreadyUpToDate(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, head: "aaa"}
	res, err := publishSetup(t, g).Pull(context.Background())
	if err != nil || res.Changed {
		t.Errorf("result = %+v, %v", res, err)
	}
}

func TestPullNeedsRepoAndUpstream(t *testing.T) {
	if _, err := publishSetup(t, &fakeGit{}).Pull(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("no repo: %v", err)
	}
	if _, err := publishSetup(t, &fakeGit{repo: true}).Pull(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "no upstream") {
		t.Errorf("no upstream: %v", err)
	}
}

func TestPullFailureIsReported(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, head: "aaa", pullErr: errors.New("conflict in a.md")}
	if _, err := publishSetup(t, g).Pull(context.Background()); err == nil || !strings.Contains(err.Error(), "conflict in a.md") {
		t.Errorf("err = %v", err)
	}
}

func TestStatusListsChangesAndUnpushedCommits(t *testing.T) {
	g := &fakeGit{repo: true, upstream: true, remote: true, ahead: 1,
		changes: []string{"M	logs/2026-10-25.md", "A	projects/noot/plan.md"}}
	st, err := publishSetup(t, g).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []app.FileChange{{Status: "M", Path: "logs/2026-10-25.md"}, {Status: "A", Path: "projects/noot/plan.md"}}
	if !slices.Equal(st.Changes, want) || st.Ahead != 1 || !st.Upstream || !st.Remote {
		t.Errorf("status = %+v", st)
	}
	for _, c := range []string{"add", "commit", "pull", "push"} {
		if slices.Contains(g.calls, c) {
			t.Errorf("status must only look, but called %q: %v", c, g.calls)
		}
	}
}

func TestStatusWithoutUpstream(t *testing.T) {
	g := &fakeGit{repo: true, remote: true}
	st, err := publishSetup(t, g).Status(context.Background())
	if err != nil || st.Upstream || !st.Remote || st.Ahead != 0 || slices.Contains(g.calls, "ahead") {
		t.Errorf("status = %+v, %v, calls %v", st, err, g.calls)
	}
}

func TestStatusRequiresRepo(t *testing.T) {
	if _, err := publishSetup(t, &fakeGit{}).Status(context.Background()); err == nil {
		t.Error("expected an error outside a git repository")
	}
}
