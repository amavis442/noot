---
title: AGENTS
created: {{date}}
tags: [agent, instructions]
---
# AGENTS.md

This vault belongs to the AI agent (Claude, Codex, …). It is the agent's
working memory: everything the agent learns, decides or does across sessions
is written here, in plain Markdown, so the next session can pick it up.

The owner has their **own, separate vault**. Read it when asked or when it
clearly helps (`noot -V <name> search …`), but **never write to it** unless
the owner explicitly asks for a specific change.

## Start of every session

1. Read this file.
2. Read `memory/index.md` and follow the links that matter for the task.
3. Check the latest logs: `noot -A cat logs/YYYY-MM-DD` for today and yesterday.
4. Search before asking: `noot -A search <terms>`; `noot search --all <terms>` covers every vault.

## Layout

| Folder | What goes there |
|---|---|
| `logs/` | One file per day (`logs/YYYY-MM-DD.md`). Append-only. Written with `noot -A log`. |
| `memory/` | Durable facts: the owner's preferences, environment, recurring patterns. One subject per file, linked from `memory/index.md`. Update in place; don't append duplicates. |
| `decisions/` | One note per decision (ADR style): context, options, decision, consequences. File name `YYYY-MM-DD-short-slug.md`. |
| `projects/` | One note per project the agent works on: status, open threads, how to build/test, gotchas. |
| `scratch/` | Temporary working notes. Safe to delete; nothing here is authoritative. |

## Writing rules

- **Log as you go**, not at the end: `noot -A log -p <project> -t <tag> "what happened, why, what's next"`.
  Keep entries short and factual: what was done, what was found, what is still open.
- Link generously with `[[wikilinks]]` (project notes, decisions, memory files) so backlinks work.
- Use frontmatter (`title`, `tags`, `created`) on every new note; `noot -A new "Title" -d decisions` does this for you.
- Facts in `memory/` must be things the owner stated or that were verified, not guesses. Note the source and date.
- Never store secrets (tokens, passwords, keys) in this vault.
- Prefer editing an existing note over creating a near-duplicate; check with `noot -A search` first.

## Useful commands

```bash
noot -A log -p noot "Added named vaults; tests green"     # append to today's log
noot -A new "Use SQLite FTS5 for search" -d decisions      # new decision note
noot -A cat memory/index                                    # print a note
noot -A append projects/noot "- [ ] add noot serve"         # append to a note
noot -A search --json fts5                                  # machine-readable search
noot -A backlinks noot                                      # what refers to the project
noot -A broken                                              # links to notes that don't exist yet
```
