# CLAUDE.md — noot

Instructions for Claude sessions in this repo, both local (Windows/Linux) and cloud.

## Agent memory: read this first

My working memory is the **agent vault** (repo `amavis442/noot-claude`), managed with noot itself.

| Session | Where the vault is | How to use it |
|---|---|---|
| Local on Patrick's PC | `D:\Noot\Claude` (registered as `claude`, `-A`) | `noot -A …`; commit and push from that folder |
| Cloud | not present | `git clone https://github.com/amavis442/noot-claude ~/noot-claude`, then `noot vault add claude ~/noot-claude --kind agent` |

At the start of every session:
1. Pull the vault (`git -C <vault> pull`), then read its `AGENTS.md` and `memory/index.md`.
2. Read `projects/noot.md` in the vault: status, gotchas, open threads for this repo.
3. Log while working: `noot -A log -p noot "<what, why, what's next>"`. Commit and push the vault when done: the vault is the one repo that may be pushed straight to `main`.

Patrick's own vault (`D:\Noot\Patrick`, repo `amavis442/noot-patrick`) is **read-only** for agents unless he asks for a specific change.

## Working with Patrick

Patrick's global `~/.claude/CLAUDE.md` applies; the points that matter most here:

- Converse in Dutch; code, comments, commit messages, PRs and tags in English. The README is Dutch.
- Commit as `Amavis442 <patteun8@gmail.com>`. Never add Co-Authored-By or other AI attribution trailers.
- Work on `feat/*`, `fix/*` or `docs/*` branches and open a PR. Never push `main` or tags; Patrick merges and tags.
- TDD; unit, integration and end-to-end tests. Every Go file starts with a 2-line `// ABOUTME:` comment.

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
