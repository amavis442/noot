package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Git is the version-control port used by Publish. internal/vcs/git implements it.
type Git interface {
	IsRepo(ctx context.Context) (bool, error)
	AddAll(ctx context.Context) error
	StagedChanges(ctx context.Context) ([]string, error) // "M\tpath" lines, as git diff --name-status
	WriteTree(ctx context.Context) (string, error)       // hash of the staged tree
	Commit(ctx context.Context, subject, body string) (string, error)
	HasUpstream(ctx context.Context) (bool, error)
	HasRemote(ctx context.Context, name string) (bool, error)
	PullRebase(ctx context.Context) error     // also keeps uncommitted changes (autostash)
	Head(ctx context.Context) (string, error) // full hash of HEAD
	Ahead(ctx context.Context) (int, error)   // commits not yet on the upstream
	Push(ctx context.Context) error
	PushSetUpstream(ctx context.Context, remote string) error
}

// PublishOptions configures Publish.
type PublishOptions struct {
	Message string // commit subject; default "noot: <short tree hash>"
	NoPull  bool   // skip git pull --rebase before pushing
}

// PublishResult reports what Publish did.
type PublishResult struct {
	Commit string // short hash of the new commit, "" when there was nothing to commit
	Files  int    // number of changed files in that commit
	Pulled bool
	Pushed bool
	Note   string // explanation when nothing could be pushed
}

const (
	defaultRemote = "origin"
	treeHashLen   = 12
	maxBodyLines  = 50
)

// Publish commits every change in the vault and pushes it: add, commit,
// pull --rebase, push. The pull only happens when there is something to push
// (it exists to keep the push from being rejected), so "nothing to push" needs
// no network at all.
func (s *Service) Publish(ctx context.Context, opts PublishOptions) (PublishResult, error) {
	var res PublishResult
	if s.Git == nil {
		return res, errors.New("no git backend configured")
	}
	ok, err := s.Git.IsRepo(ctx)
	if err != nil {
		return res, err
	}
	if !ok {
		return res, fmt.Errorf("%s is not a git repository: run `git init` and add a remote first", s.Vault.Root)
	}
	// Never commit the generated index.
	if err := s.Vault.EnsureGitignore(); err != nil {
		return res, err
	}
	if err := s.Git.AddAll(ctx); err != nil {
		return res, err
	}
	changes, err := s.Git.StagedChanges(ctx)
	if err != nil {
		return res, err
	}
	if len(changes) > 0 {
		subject := strings.TrimSpace(opts.Message)
		if subject == "" {
			tree, err := s.Git.WriteTree(ctx)
			if err != nil {
				return res, err
			}
			subject = "noot: " + shorten(tree, treeHashLen)
		}
		if res.Commit, err = s.Git.Commit(ctx, subject, commitBody(changes)); err != nil {
			return res, err
		}
		res.Files = len(changes)
	}

	upstream, err := s.Git.HasUpstream(ctx)
	if err != nil {
		return res, err
	}
	if !upstream {
		remote, err := s.Git.HasRemote(ctx, defaultRemote)
		if err != nil {
			return res, err
		}
		if !remote {
			res.Note = "no remote configured: committed locally (git remote add origin <url> to enable pushing)"
			return res, nil
		}
		if err := s.Git.PushSetUpstream(ctx, defaultRemote); err != nil {
			return res, err
		}
		res.Pushed = true
		return res, nil
	}

	ahead, err := s.Git.Ahead(ctx) // against the remote-tracking ref: local, no network
	if err != nil {
		return res, err
	}
	if ahead == 0 {
		return res, nil
	}
	if !opts.NoPull {
		if err := s.Git.PullRebase(ctx); err != nil {
			return res, fmt.Errorf("git pull --rebase failed; resolve it in %s, then run noot push again: %w", s.Vault.Root, err)
		}
		res.Pulled = true
	}
	if err := s.Git.Push(ctx); err != nil {
		return res, err
	}
	res.Pushed = true
	return res, nil
}

// PullResult reports what Pull did.
type PullResult struct {
	From, To string // HEAD before and after
	Changed  bool
}

// Pull brings in commits from the upstream (git pull --rebase --autostash).
// Uncommitted local changes are kept.
func (s *Service) Pull(ctx context.Context) (PullResult, error) {
	var res PullResult
	if s.Git == nil {
		return res, errors.New("no git backend configured")
	}
	ok, err := s.Git.IsRepo(ctx)
	if err != nil {
		return res, err
	}
	if !ok {
		return res, fmt.Errorf("%s is not a git repository", s.Vault.Root)
	}
	up, err := s.Git.HasUpstream(ctx)
	if err != nil {
		return res, err
	}
	if !up {
		return res, errors.New("current branch has no upstream: run noot push once to set it")
	}
	if res.From, err = s.Git.Head(ctx); err != nil {
		return res, err
	}
	if err := s.Git.PullRebase(ctx); err != nil {
		return res, fmt.Errorf("git pull --rebase failed; resolve it in %s: %w", s.Vault.Root, err)
	}
	if res.To, err = s.Git.Head(ctx); err != nil {
		return res, err
	}
	res.Changed = res.From != res.To
	return res, nil
}

func commitBody(changes []string) string {
	lines := changes
	if len(lines) > maxBodyLines {
		lines = append(lines[:maxBodyLines:maxBodyLines], fmt.Sprintf("… and %d more", len(changes)-maxBodyLines))
	}
	return strings.Join(lines, "\n")
}

func shorten(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
