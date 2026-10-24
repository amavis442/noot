// Package git implements app.Git by running the git command-line tool, so
// noot uses the user's own git config, credentials and SSH setup.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Repo runs git in one directory.
type Repo struct {
	dir string
	bin string
}

// New returns a Repo for dir, using `git` from PATH.
func New(dir string) *Repo { return &Repo{dir: dir, bin: "git"} }

func (r *Repo) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Dir = r.dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			msg := strings.TrimSpace(errb.String())
			if msg == "" {
				msg = strings.TrimSpace(out.String())
			}
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
		}
		return "", fmt.Errorf("running git: %w", err)
	}
	return strings.TrimRight(out.String(), "\r\n"), nil
}

// IsRepo reports whether dir is the top level of a git work tree.
func (r *Repo) IsRepo(ctx context.Context) (bool, error) {
	top, err := r.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
			return false, nil
		}
		return false, err
	}
	return samePath(top, r.dir), nil
}

// AddAll stages every change (respecting .gitignore).
func (r *Repo) AddAll(ctx context.Context) error {
	_, err := r.run(ctx, "add", "-A")
	return err
}

// StagedChanges lists staged changes as "<status>\t<path>", sorted by path.
func (r *Repo) StagedChanges(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "-c", "core.quotepath=off", "diff", "--cached", "--name-status", "--no-renames")
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// WriteTree returns the hash of the staged tree.
func (r *Repo) WriteTree(ctx context.Context) (string, error) {
	return r.run(ctx, "write-tree")
}

// Commit commits the staged changes and returns the short commit hash.
func (r *Repo) Commit(ctx context.Context, subject, body string) (string, error) {
	msg := subject
	if body != "" {
		msg += "\n\n" + body
	}
	if _, err := r.run(ctx, "commit", "-q", "-m", msg); err != nil {
		return "", err
	}
	return r.run(ctx, "rev-parse", "--short", "HEAD")
}

// HasUpstream reports whether the current branch tracks a remote branch.
func (r *Repo) HasUpstream(ctx context.Context) (bool, error) {
	_, err := r.run(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	return err == nil, nil
}

// HasRemote reports whether a remote with that name exists.
func (r *Repo) HasRemote(ctx context.Context, name string) (bool, error) {
	out, err := r.run(ctx, "remote")
	if err != nil {
		return false, err
	}
	for _, rm := range strings.Fields(out) {
		if rm == name {
			return true, nil
		}
	}
	return false, nil
}

// PullRebase fetches and rebases local commits on top of the upstream.
func (r *Repo) PullRebase(ctx context.Context) error {
	_, err := r.run(ctx, "pull", "--rebase", "-q")
	return err
}

// Ahead returns the number of local commits not on the upstream.
func (r *Repo) Ahead(ctx context.Context) (int, error) {
	out, err := r.run(ctx, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// Push pushes the current branch to its upstream.
func (r *Repo) Push(ctx context.Context) error {
	_, err := r.run(ctx, "push", "-q")
	return err
}

// PushSetUpstream pushes the current branch to remote and starts tracking it.
func (r *Repo) PushSetUpstream(ctx context.Context, remote string) error {
	_, err := r.run(ctx, "push", "-q", "-u", remote, "HEAD")
	return err
}

// samePath compares paths as git prints them (forward slashes, possibly a
// different drive-letter case or an MSYS-style /d/... form on Windows).
func samePath(gitPath, dir string) bool {
	a := filepath.Clean(filepath.FromSlash(gitPath))
	b := filepath.Clean(dir)
	if real, err := filepath.EvalSymlinks(b); err == nil {
		b = real
	}
	if real, err := filepath.EvalSymlinks(a); err == nil {
		a = real
	}
	if filepath.Separator == '\\' {
		a, b = strings.ToLower(msysToWindows(a)), strings.ToLower(b)
	}
	return a == b
}

// msysToWindows turns \d\Noot\Patrick (MSYS2 git output) into d:\Noot\Patrick.
func msysToWindows(p string) string {
	if len(p) >= 3 && p[0] == '\\' && p[2] == '\\' && ('a' <= p[1] && p[1] <= 'z' || 'A' <= p[1] && p[1] <= 'Z') {
		return p[1:2] + ":" + p[2:]
	}
	return p
}
