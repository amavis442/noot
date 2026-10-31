# CLAUDE.md — noot

Instructions for Claude sessions in this repo, both local (Windows/Linux) and cloud.

## Agent memory: read this first

My working memory is the **agent vault**, managed with noot itself. Patrick uses noot on several machines (home and work), each with **its own vaults that must not mix**. So never assume a path, repo or machine: the vaults are whatever is registered on the machine you run on.

- `noot vault list` shows them: `A` is the agent vault (use `noot -A …`), `*` is Patrick's own vault.
- Never write into, clone, pull or push another machine's vault, and never copy notes from one machine's vault into another's.
- No agent vault registered (e.g. a cloud session): skip the vault steps below unless Patrick names one.

At the start of every session:
1. If the agent vault is a git repo with a remote, `noot -A pull`. Then read its `AGENTS.md` and `memory/index.md`.
2. Run `noot -A context noot`: the project note (status, gotchas, open threads for this repo), related notes and recent log entries.
3. Log while working: `noot -A log -p noot "<what, why, what's next>"`. When done and the vault has a remote: `noot -A push` (the vault may be pushed straight to `main`).

Patrick's own vault (`*`) is **read-only** for agents unless he asks for a specific change.

## Working with Patrick

Patrick's global `~/.claude/CLAUDE.md` applies; the points that matter most here:

- Converse in Dutch; code, comments, commit messages, PRs and tags in English. The README is Dutch.
- Commit as `Amavis442 <patteun8@gmail.com>`. Never add Co-Authored-By or other AI attribution trailers.
- **`main` is protected on GitHub: never push to it.** Every change starts with a work item (a GitHub issue, `gh issue create`) and a branch named after it: `<type>/<issue-nr>-<short-slug>`, e.g. `feat/12-vault-export` or `fix/13-watch-crash` (`chore/` for housekeeping). Commit there, push the branch and open a PR that references the issue (`Closes #<nr>`). Patrick merges and tags. This applies to the noot repo only. Every other code repo: `feat/*` / `fix/*` branches + PR, never push `main` or tags.
- TDD; unit, integration **and** end-to-end tests (`make test` runs all three). No exceptions.

**Accepted exception:** noot uses idiomatic Go layering (`internal/domain`, `app`, `storage`, `cli`) instead of feature-first folders. It is a small CLI with one domain; the dependency rule still holds (domain has no project imports, storage implements an `app` port).

## This repo

```bash
make          # bin/noot or bin\noot.exe
make test     # go vet + go test
make dist VERSION=v0.x.y
```

- Layers: `internal/domain` ← `internal/app` (Store port, Sync, Watch) ← `internal/storage/sqlite`; `parser`, `vault`, `config`, `cli` around it. Keep dependencies pointing inward.
- The Makefile must stay **shell-agnostic** (only `go`, `make`, `echo`, `&&`). Never set `SHELL` for Windows: Patrick's PATH has MSYS2 first, and MSYS2 make turns `/c` into `C:/`.
- `ncruces/go-sqlite3` is pinned to v0.30.5 (newer needs Go ≥ 1.25). No cgo.
- Bump `schemaVersion` in `internal/storage/sqlite/store.go` when parsed output changes; indexes then rebuild themselves.
- Cloud sessions only: `proxy.golang.org` and vanity module hosts are blocked; see `memory/environment.md` in the vault for the `GOPROXY=direct` + temporary `replace` workaround. Never commit those replaces.
