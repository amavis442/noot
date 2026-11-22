# noot 🐿️

*A second brain made of plain Markdown files.* *Noot* is Dutch for "nut": like a squirrel you hide your nuts, and `noot` helps you find them again.

The files are the source of truth. `noot` builds an index next to them (SQLite + FTS5 in `.noot/index.db`). The index can always be thrown away and rebuilt. You write in your own editor, sync with git, and `noot` takes care of search, backlinks, tags and daily notes.

## Installing

```bash
go mod tidy                       # first time: fetches the missing checksums
make                              # → bin/noot (Linux) or bin\noot.exe (Windows)
make install                      # go install with version info → $GOPATH/bin
```

| Make target | What |
|---|---|
| `make` / `make build` | Build for this system; `.exe` is added automatically on Windows |
| `make build-linux` / `make build-windows` | Cross-compile into `bin/` |
| `make dist` | Release binaries in `dist/` for linux and windows (amd64 + arm64), with the web frontend built in (runs `make web` first) |
| `make test` | `go vet` + all tests: unit, integration (real SQLite) and end-to-end (`e2e/`: builds the binary and runs every command against temporary vaults) |
| `make web` | Build the web frontend into `web/dist/`; the next `make` embeds it in the binary (needs Node.js) |
| `make web-test` | Frontend tests (Vitest; needs Node.js) |
| `make run ARGS="search sqlite"` | Run without installing (`go run`) |
| `make clean` / `fmt` / `tidy` / `help` | |

The Makefile works with any GNU make, on Linux and on Windows (native, MSYS2 or Git Bash), and from cmd, PowerShell or bash. The recipes only call `go`, `make` and `echo`, so it does not matter which shell make uses. `noot --version` shows the git commit Go embeds in the binary; for a release pass `make dist VERSION=v0.2.0`.

`make` and `make test` do not need Node.js. Without `make web`, `noot serve` only shows a notice on `/` that the frontend is not built in; the JSON API still works.

There is no cgo: the SQLite driver (`ncruces/go-sqlite3`) runs as WebAssembly through wazero. So you need no C compiler, and all binaries are static.

The first time noot runs it compiles SQLite (about 1 second) and keeps the result in `%LocalAppData%\noot\wazero` (Windows) or `~/.cache/noot/wazero` (Linux). After that every command starts within a few milliseconds. Set another location with `$NOOT_CACHE_DIR`.

## Quick start

```bash
noot init D:\Noot\Patrick         # creates journal/ zettels/ projects/ attachments/ .noot/ + .gitignore, registered as "patrick"
cd D:\Noot\Patrick
noot today                        # opens journal/2026-10-24.md in $EDITOR
noot agenda                       # opens agenda/2026-W43.md, this week's agenda
noot reminder "Call the dentist" --due friday
noot new "Clean architecture in Go" -t go -t architecture
noot search clean arch            # prefix match, accents are ignored (cafe = café)
noot backlinks "clean architecture in go"
```

## Several vaults: you and your agent

You can give several vaults a name. The idea is one vault for yourself and a separate vault for your AI agent. The agent keeps its logs, memory and decisions there, without writing in your notes.

```bash
noot init D:\Noot\Patrick --name patrick           # personal: journal/ zettels/ projects/ attachments/
noot init D:\Noot\Claude  --name claude --agent    # agent: AGENTS.md logs/ memory/ decisions/ projects/ scratch/
noot vault list
#   A claude   agent     D:\Noot\Claude
#   * patrick  personal  D:\Noot\Patrick
```

`init` always registers a vault. Without `--name` the folder name becomes the name (`D:\Noot\Claude` becomes `claude`), and a folder that is already registered keeps its name. If a name is already used for another folder, `init` stops before anything is created; pick a name with `--name`. That way a vault never silently overwrites another.

You can put all vaults under one folder, for example `D:\Noot\`. A vault always skips subfolders with their own `.noot/`, so vaults cannot index each other's notes by accident.

The first personal vault becomes the **default** (`*`) and the first agent vault the **agent vault** (`A`). You can change that later with `noot vault default <name>` and `noot vault agent <name>`. The names are stored in `%AppData%\noot\config.yaml` (Windows) or `~/.config/noot/config.yaml` (Linux), or in the place you give with `$NOOT_CONFIG`.

**Which vault is used?** The first rule that applies wins:

1. `-A` / `--agent`: the agent vault
2. `-V <name|path>`: a registered name or a folder
3. `$NOOT_VAULT`: the same, but from the environment
4. the current folder, if it is inside a vault with a `.noot/`
5. the default vault

A few commands are made for the agent vault, though you can use them in any vault:

```bash
noot -A context noot                                  # project note + related notes + recent log entries
noot -A log -p noot -t release "v0.2.0 tagged"        # → logs/2026-10-24.md, "## 13:25 · [[noot]] #release"
git log -1 --oneline | noot -A log -p noot             # text via stdin
noot -A append --create projects/noot "- [ ] serve"    # append to a note
noot -A cat memory/index                               # print a note (--json for agents)
noot search --all sqlite                               # search in all vaults, with a [vault] prefix
```

`AGENTS.md` in the agent vault tells every agent how the vault works: what to read at the start of a session, where to write what, and that it only reads your vault and never writes to it.

Every read command first updates the index incrementally (based on mtime and size; it takes milliseconds). A separate `index` step is therefore not needed. `--no-sync` skips it, and `noot watch` keeps the index live.

## Commands

| Command | What |
|---|---|
| `init [dir] [--name n] [--agent]` | Creates the folder structure, `.noot/` and `.gitignore` and registers the vault, under `--name` or else the folder name |
| `vault list\|add\|remove\|default\|agent` | Manage named vaults |
| `log [-p project] [-t tag] <text>` | Append a timestamped entry to `logs/YYYY-MM-DD.md` (also via stdin) |
| `cat <note>` | Print a note |
| `context <note> [--days 7] [-n 20]` | Print a note with the notes that link to it or sit next to it (for `projects/x/x.md`), and the log entries of the last days that link to it. One command to pick up a project |
| `rename <note> [new-path] [--title t] [--dry-run]` | Move a note and/or change its title; all links to it are updated in the form they were written in (path, file name or title). Links by id or alias keep working as they are, links inside code stay untouched |
| `append [--create] <note> <text>` | Append text to a note (also via stdin) |
| `index [--full]` | Update the index; `--full` rebuilds everything |
| `search <terms> [-t tag] [-n 20] [--raw] [--all]` | Full-text search; `--all` searches all vaults at once. With `--raw` you use FTS5 syntax (`NEAR`, `OR`, `"exact phrase"`) |
| `backlinks <note>` (`bl`) | What links here, with the paragraph as context |
| `links <note>` | Outgoing links, broken links marked with ✗ |
| `broken` | All links that go nowhere |
| `orphans` | Notes without incoming or outgoing links |
| `tags [tag]` | Tag overview with counts, or the notes with a tag (`go` also matches `go/generics`) |
| `new <title> [-d zettels] [-t tag] [-o]` | New note with frontmatter |
| `today [--offset -1] [--print]` | Open or create the daily note |
| `agenda [--offset 1] [--print]` | Open or create the weekly agenda: `agenda/YYYY-Www.md` (ISO week, Monday to Sunday) with a heading per day and under it Planned, Doing and Done; `--offset` counts in weeks |
| `reminder [text] [--due day] [--all]` (`rem`) | With text: add a reminder to `reminders.md`. Without text: the open reminders, earliest date first (`--all` also shows the done ones) |
| `reminder done <n>...` | Tick reminders off; a planned reminder is ticked in the agenda too |
| `reminder plan <n> <day>` | Put a reminder under Planned of that day in the weekly agenda (the agenda is created when needed) |
| `todo [--today\|--week\|--all]` | Every task in one list: reminders, the backlog (`agenda/agenda.md`) and the weekly agendas, grouped as overdue, today, this week, later, backlog and done (see [Todo](#todo)) |
| `open <note> [--print]` | Open a note by id, path, file name, title or alias |
| `watch` | Keep the index live (Ctrl+C to stop) |
| `serve [--addr 127.0.0.1:4242]` | Web UI to search, create and edit notes (also the daily note and the weekly agenda) and to manage reminders, plus a JSON API (see [Web UI](#web-ui)). Keeps the index live like `watch` |
| `tui` | Interactive terminal UI with search, daily note, weekly agenda, reminders and todo (see [Terminal UI](#terminal-ui)) |
| `push [-m message] [--no-pull]` | Commit and push everything: `git add -A`, commit, `pull --rebase`, `push` |
| `pull` | Fetch changes from elsewhere (`pull --rebase --autostash`) and update the index |
| `status` | Show what the next `push` will take along: changed files (A/M/D) and commits not pushed yet. Only looks, no network |

In a terminal, `search`, `reminder`, `todo` and `status` get colour, in the same style as `noot tui`: paths and matches in the accent colour, overdue reminders in red, `A`/`M`/`D` in green/yellow/red. The text itself stays exactly the same. In a pipe or script, with `--json` or with `NO_COLOR` set there is no colour. In Git Bash/MSYS2 (mintty) noot does not see a terminal and the output stays plain there too.

You can add `--json` everywhere, for scripts, fzf or an AI agent. For example:

```bash
noot search --json repository | jq -r '.[].path' | fzf --preview 'bat --color=always {}'
```

`<note>` is looked up in this order: **id → path → file name → title → alias**, case-insensitively. The same order is used to resolve `[[wikilinks]]`.

## Todo

`noot todo` puts all tasks of your vault in one list. It reads three sources and keeps no administration of its own; no index is needed either, the files are read on every call:

| Source | Which lines |
|---|---|
| `reminders.md` | all reminders |
| `agenda/agenda.md` | the backlog without a day: `- [ ]`/`- [x]` lines under `## Planned`, `## Doing` and `## Done`. An item may carry `· due YYYY-MM-DD` or `· planned YYYY-MM-DD`, like a reminder; without a date it is in the backlog. Indented lines below an item belong to it |
| `agenda/YYYY-Www.md` | only lines with a checkbox under `### Planned` and `### Doing`. Plain bullets such as `- Holiday 16-27` do not count. The day a line stands under is its planned day |

A reminder you put in the week with `noot reminder plan` is in both files. Then only the reminder counts; the line with `([[reminders]])` in the week is skipped.

A task goes in the group of its earliest date (`due` or `planned`): **Overdue** when that is before today, **Today**, **This week** (up to and including Sunday), **Later**, and without a date **Backlog**. Done tasks are under **Done**.

```bash
noot todo             # all open tasks
noot todo --today     # overdue and today
noot todo --week      # also the rest of this week
noot todo --all       # also the done ones
noot todo --json      # for scripts, with source, file, line and group per task
```

Every line ends with the file and line number. You tick tasks off in the [todo tab](#terminal-ui) of `noot tui`, or for reminders with `noot reminder done <n>`.

## Terminal UI

```bash
noot tui                          # the vault that -V, -A or the current folder selects
noot -A tui
```

`noot tui` opens the vault full-screen in the terminal (PowerShell, Windows Terminal and Linux terminals), with five tabs. Switch with `1`-`5`; quit with `q` or `esc`. `?` shows all keys of where you are.

The keys follow the defaults of [superfile](https://superfile.dev/list/hotkey-list) (`spf`):

| Key | What |
|---|---|
| `j`/`k`, `↓`/`↑`, `pgdn`/`pgup` | Down/up in a list or note |
| `enter`, `l`, `→` | Open: the selected note in your editor |
| `tab`/`L`, `H` (`shift+←`) | Next/previous panel |
| `/` | To the search bar |
| `f` | Preview on/off |
| `e` | Edit in your editor |
| `E` | Edit everything: in Agenda the whole week |
| `ctrl+n` | New (reminder) |
| `.` | Show/hide done reminders and tasks (in Todo also `a`) |
| `enter` / `esc` or `ctrl+c` | While typing: confirm / cancel |

On top of that, where superfile has nothing: `J`/`K` (`shift+↓`/`↑`) to scroll the preview, `[` and `]` for previous/next day or week, `t` for today, `x` to tick off (in Todo also the space bar) and `p` to plan.

The tab names are Dutch, like the rest of the UI text:

| Tab | What |
|---|---|
| **1 Zoeken** (search) | Press `/` to search; as in superfile the search bar has no focus at startup, so `1`-`5` switch tabs right away. A search bar with two panels under it: the results, which update while you type, and the selected note rendered. `#tag` in the query filters on tag. After typing, `tab`, `enter` or `esc` takes you to the results, which keep the arrows; the preview never gets the focus, as in superfile. The panel with the focus has a coloured border |
| **2 Vandaag** (today) | The daily note (created when missing) |
| **3 Agenda** | The weekly agenda day by day, opened at today. `↑`/`↓` (`j`/`k`) pick a day; `e` opens only that day in your editor and then saves it in the week file, like *Bewerken* (edit) per day in the web UI. If that day was changed elsewhere in the meantime, nothing is overwritten and the TUI keeps your text: `e` opens your version again. `E` edits the whole week |
| **4 Herinneringen** (reminders) | The open reminders, overdue in red. With `ctrl+n` a due date follows after the text (empty is none), with `p` the day (empty is today) |
| **5 Todo** | All tasks from reminders, backlog and weekly agendas (see [Todo](#todo)), grouped as Achterstallig (overdue), Vandaag (today), Deze week (this week), Later, Backlog and Klaar (done). `space` or `x` ticks off in the source file; if the line changed in the meantime nothing happens and the list reloads. A ticked backlog item moves to `## Done`; a reminder stays in `reminders.md`. `enter` or `e` opens the source file, `p` plans a reminder in the weekly agenda (reminders only; the other tasks are already in the agenda), `a` also shows the done ones |

With `e` you open what you see in `$VISUAL` or `$EDITOR` (without those variables Notepad on Windows and `vi` on Linux), like `noot open`. When you close the editor the UI comes back and the index is updated. You write a day as on the command line: `2026-11-06`, `today`, `tomorrow`, `+3` or `friday` (the Dutch words `vandaag`, `morgen` and `vrijdag` work too). With `NO_COLOR` set everything is shown without colour.

`noot tui` needs a real terminal. In a pipe or script it stops with an error message; use the ordinary commands there, which keep their plain output and `--json`.

## Web UI

```bash
make web && make                  # once: build the frontend and put it in the binary
noot serve                        # → http://127.0.0.1:4242
noot -A serve --addr 127.0.0.1:8080
```

`noot serve` serves the vault that every other command selects too (`-V`, `-A`, the current folder or the default). In the UI you can:

- **Search** with the same query as `noot search` and open a note. Wikilinks and Markdown links are clickable.
- **Edit any note** with *Bewerken* (edit): you edit the whole file, frontmatter included, with a preview next to it that follows as you type. If the file was changed outside the UI in the meantime (in your editor, or by `noot pull`), saving is refused and the UI keeps your text; *Herladen* (reload) fetches the version from disk.
- **Create a new note** with a title and optionally text and tags. New notes go in `zettels/`.
- **The daily note** under *Vandaag* (today), like `noot today`, with browsing to the previous and next day.
- **The weekly agenda** under *Agenda*, like `noot agenda`, with browsing per week. The week is created when it does not exist yet. Every day has its own *Bewerken*: you edit only that day, while it stays in the week file. Saving is only refused if exactly that day was changed elsewhere in the meantime; a change on another day (such as a planned reminder) simply stays. With *Hele week bewerken* (edit the whole week) you edit the whole file.
- **Manage reminders** under *Reminders*, like `noot reminder`: add with a due date, tick off and plan in the weekly agenda. You write a day as on the command line: `2026-11-06`, `today`, `tomorrow`, `+3` or `friday`.

The API, for scripts:

| Endpoint | What |
|---|---|
| `GET /api/search?q=…&tag=…&limit=…` | Search results, like `noot search --json` |
| `GET /api/notes/{ref}` | Note by id, path, file name, title or alias: `content` (the file), `html` (rendered) and `version` |
| `PUT /api/notes/{ref}` | Save a note; body `{"content": "…", "version": "…"}`. Gives `409` if the file changed after loading |
| `POST /api/notes` | New note; body `{"title": "…", "body": "…", "tags": ["…"]}` |
| `POST /api/preview` | Render unsaved text the way the note will look; body `{"path": "…", "content": "…"}`, answer `{"html": "…"}`. Reads and writes no files |
| `GET /api/journal/{day}` | Daily note (created when missing), like a note plus `date`; `{day}` may also be `today`. Saving goes through `PUT /api/notes/{path}` |
| `GET /api/agenda/{day}` | Weekly agenda of the week `{day}` falls in (created when missing), plus `week`, `monday`, `intro_html` and `days` (per day `date`, `heading`, `content`, `html` and `version`) |
| `PUT /api/agenda/{day}/day` | Save one day of the agenda; body `{"content": "…", "version": "…"}` with the `version` of that day. Gives `409` if that day changed after loading |
| `GET /api/reminders?all=true` | Reminders, like `noot reminder --json` (without `all` only the open ones) |
| `POST /api/reminders` | Add a reminder; body `{"text": "…", "due": "friday"}` (`due` is optional) |
| `POST /api/reminders/{n}/done` | Tick reminder `n` off (body `{}`) |
| `POST /api/reminders/{n}/plan` | Plan reminder `n`; body `{"day": "monday"}` |

Send bodies as `application/json`.

On a loopback address (the default) the server only answers requests for `localhost`/`127.0.0.1`. That protects against DNS rebinding. `POST` only accepts JSON, so other sites cannot create notes. There is no login: do not bind to a public address.

### Developing the frontend

The frontend is in `web/` (React + TypeScript, Vite, Vitest). With Node.js:

```bash
cd web
npm ci
npm run dev                       # Vite on http://localhost:5173; /api goes to noot serve on :4242
npm test                          # Vitest + Testing Library
npm run build                     # typecheck + build into web/dist/
```

Also run `noot serve` for the API. The code is split per feature: `src/features/search`, `notes`, `journal`, `agenda` and `reminders` (hooks for the logic, components for the display) and `src/shared` (API client, routing and note editing, which notes, journal and agenda share).

## Syncing with git

A vault is just a git repo. `noot push` does the whole round in one go:

```bash
noot push                     # commit "noot: 3f9a1c2e4b7d" + list of changed files, pull --rebase, push
noot push -m "Weekly review"  # your own message
noot -A push                  # the agent vault
noot status                   # what goes along with the next push?
noot pull                     # fetch what was pushed elsewhere
```

- The default message is `noot: <hash>`: the first 12 characters of the hash of the vault's content (`git write-tree`). Identical content therefore gives the same hash. The body lists the changed files.
- If there is something to push, `git pull --rebase` runs first, so a push from another machine (or from your agent) is taken along instead of being rejected. Then noot updates the index.
- If there is nothing to push, noot says `nothing to push` at once, without contacting the remote. That saves the network time (with GitHub easily 1 to 2 seconds).
- You fetch changes from elsewhere with `noot pull`. Local, not yet committed changes stay put.
- If the branch has no upstream yet but there is an `origin`, noot runs `git push -u origin HEAD`. Without a remote it only commits locally.
- `.noot/` (the index) never goes into git; noot adds it to `.gitignore` when needed.
- noot uses the `git` from your PATH, so your own config, credentials and SSH keys.

## Conventions

```markdown
---
id: 01M5Q07ST0D58RJ8G68GBWRR3S   # optional; without an id the path (without .md) is the id
title: Clean architecture in Go
tags: [go, architecture]    # or: tags: go, architecture
aliases: [hexagonal]
created: 2026-10-24
---
# Clean architecture in Go

See [[repository-pattern]], [[Repository pattern|label]], [[note#heading]]
or an ordinary link: [text](../projects/startpage.md). #refactoring #go/patterns
```

- Links and tags in code blocks and `inline code` are ignored.
- Folders that start with `.` (`.git`, `.obsidian`, `.noot`) and `node_modules` are skipped.
- Two files with the same `id` show up as a warning; the second file is then skipped.
- `noot new` gives every note a ULID: 26 characters, unique without looking at the rest of the vault, and still sorted by creation time. An id of your own is fine too; any text will do, also the timestamps (`20261024130512`) from older notes.
- An existing Obsidian vault just works. Logseq graphs with Markdown too, although block references (`((uuid))`) are not supported yet.

## Architecture

```
cmd/noot/                 main
internal/domain/          Note, Link, NoteRef, Task … (no dependencies)
internal/parser/          Markdown → domain.Note (goldmark AST + frontmatter)
internal/vault/           File layout: finding the root, scanning, creating notes/journal, reading and ticking tasks
internal/app/             Use cases: Sync, Watch, Resolve, notes, daily note, reminders and todo; defines the Store port
internal/storage/sqlite/  Store implementation (SQLite + FTS5)
internal/cli/             cobra commands and output
internal/httpapi/         HTTP adapter for noot serve: JSON API over the use cases + static files
internal/tui/             Terminal adapter for noot tui (Bubble Tea): one package per tab, shared style in tui/kit
internal/editor/          $VISUAL/$EDITOR command, shared by noot open and noot tui
internal/theme/           Colour palette, shared by noot tui and the coloured command output
web/                      React frontend; web/dist is embedded in the binary through go:embed
```

Dependencies point inward: `cli`/`httpapi`/`tui → app → domain`, and `storage` implements `app.Store`. If you want another search engine (bleve, Meilisearch) or semantic search with embeddings, you write a new Store. The rest stays the same.

## Ideas for later

- Extend the web UI: backlinks and a graph (`/graph.json` + cytoscape.js)
- Semantic search: embeddings through Ollama next to FTS5 (sqlite-vec or a table of its own)
- Auto-commit: `noot sync` that runs `git add/commit/pull --rebase/push`
- Templates in `.noot/templates/` for `new` and `today`
