# noot 🐿️

*Een second brain van gewone Markdown-bestanden.* Net als een eekhoorn verstop je je nootjes, en `noot` helpt je ze terug te vinden.

De bestanden zijn de bron van waarheid. `noot` bouwt er een index naast (SQLite + FTS5 in `.noot/index.db`). Die index is altijd weg te gooien en opnieuw op te bouwen. Je schrijft in je eigen editor, synct met git, en `noot` regelt zoeken, backlinks, tags en dagnotities.

## Installeren

```bash
go mod tidy                       # eerste keer: haalt de ontbrekende checksums op
go install ./cmd/noot             # → $GOPATH/bin/noot(.exe)
```

Er is geen cgo nodig: de SQLite-driver (`ncruces/go-sqlite3`) draait als WebAssembly via wazero. Daardoor werkt `go build` hetzelfde op Windows, Linux en in Docker. Cross-compilen kan gewoon:

```bash
GOOS=linux GOARCH=amd64 go build -o dist/noot ./cmd/noot
```

## Snel starten

```bash
noot init D:\Notes                # maakt journal/ zettels/ projects/ attachments/ .noot/ + .gitignore
cd D:\Notes
noot today                        # opent journal/2026-10-24.md in $EDITOR
noot new "Clean architecture in Go" -t go -t architectuur
noot search clean arch            # prefix-match, accenten worden genegeerd (cafe = café)
noot backlinks "clean architecture in go"
```

## Meerdere vaults: jij en je agent

Je kunt meerdere vaults een naam geven. Het idee is een vault voor jezelf en een aparte vault voor je AI-agent. Daarin houdt de agent zijn logs, geheugen en beslissingen bij, zonder in jouw notities te schrijven.

```bash
noot init D:\Notes        --name patrick            # persoonlijk: journal/ zettels/ projects/ attachments/
noot init D:\Notes-claude --name claude --agent     # agent: AGENTS.md logs/ memory/ decisions/ projects/ scratch/
noot vault list
#   A claude   agent     D:\Notes-claude
#   * patrick  personal  D:\Notes
```

De eerste persoonlijke vault wordt de **default** (`*`) en de eerste agent-vault de **agent-vault** (`A`). Dat kun je later veranderen met `noot vault default <naam>` en `noot vault agent <naam>`. De namen staan in `%AppData%\noot\config.yaml` (Windows) of `~/.config/noot/config.yaml` (Linux), of op de plek die je met `$NOOT_CONFIG` opgeeft.

**Welke vault wordt gebruikt?** De eerste regel die past, wint:

1. `-A` / `--agent`: de agent-vault
2. `-V <naam|pad>`: een geregistreerde naam of een map
3. `$NOOT_VAULT`: hetzelfde, maar dan uit de omgeving
4. de huidige map, als die in een vault met `.noot/` ligt
5. de default-vault

Speciaal voor de agent-vault zijn er een paar commando's, die je overigens in elke vault kunt gebruiken:

```bash
noot -A log -p noot -t release "v0.2.0 getagd"        # → logs/2026-10-24.md, "## 13:25 · [[noot]] #release"
git log -1 --oneline | noot -A log -p noot             # tekst via stdin
noot -A append --create projects/noot "- [ ] serve"    # aan een notitie toevoegen
noot -A cat memory/index                               # notitie printen (--json voor agents)
noot search --all sqlite                               # zoeken in álle vaults, met [vault]-prefix
```

`AGENTS.md` in de agent-vault legt elke agent uit hoe de vault werkt: wat hij aan het begin van een sessie leest, waar hij wat wegschrijft, en dat hij in jouw vault alleen leest en niet schrijft.

Elk leescommando werkt de index eerst incrementeel bij (op basis van mtime en grootte, en het duurt milliseconden). Een aparte `index`-stap is dus niet nodig. Met `--no-sync` sla je dat over, en met `noot watch` houd je de index live bij.

## Commando's

| Commando | Wat |
|---|---|
| `init [dir] [--name n] [--agent]` | Maakt de mappenstructuur, `.noot/` en `.gitignore`; met `--name` wordt de vault ook geregistreerd |
| `vault list\|add\|remove\|default\|agent` | Vaults met een naam beheren |
| `log [-p project] [-t tag] <tekst>` | Regel met tijdstempel toevoegen aan `logs/JJJJ-MM-DD.md` (ook via stdin) |
| `cat <note>` | Notitie printen |
| `append [--create] <note> <tekst>` | Tekst achter een notitie plakken (ook via stdin) |
| `index [--full]` | Index bijwerken; `--full` bouwt alles opnieuw op |
| `search <termen> [-t tag] [-n 20] [--raw] [--all]` | Full-text zoeken; met `--all` in alle vaults tegelijk. Met `--raw` gebruik je FTS5-syntax (`NEAR`, `OR`, `"exacte zin"`) |
| `backlinks <note>` (`bl`) | Wie linkt hiernaar, met de alinea als context |
| `links <note>` | Uitgaande links, gebroken links gemarkeerd met ✗ |
| `broken` | Alle links die nergens heen gaan |
| `orphans` | Notities zonder in- of uitgaande links |
| `tags [tag]` | Tag-overzicht met aantallen, of de notities met een tag (`go` matcht ook `go/generics`) |
| `new <titel> [-d zettels] [-t tag] [-o]` | Nieuwe notitie met frontmatter |
| `today [--offset -1] [--print]` | Dagnotitie openen of aanmaken |
| `open <note> [--print]` | Notitie openen via id, pad, bestandsnaam, titel of alias |
| `watch` | Index live bijhouden (Ctrl+C om te stoppen) |

Overal kun je `--json` toevoegen voor scripts, fzf of een AI-agent. Bijvoorbeeld:

```bash
noot search --json repository | jq -r '.[].path' | fzf --preview 'bat --color=always {}'
```

`<note>` wordt in deze volgorde opgezocht: **id → pad → bestandsnaam → titel → alias**, zonder onderscheid tussen hoofd- en kleine letters. Diezelfde volgorde wordt gebruikt om `[[wikilinks]]` op te lossen.

## Conventies

```markdown
---
id: 20261024130512          # optioneel; zonder id is het pad (zonder .md) de id
title: Clean architecture in Go
tags: [go, architectuur]    # of: tags: go, architectuur
aliases: [hexagonaal]
created: 2026-10-24
---
# Clean architecture in Go

Zie [[repository-pattern]], [[Repository pattern|label]], [[note#kop]]
of een gewone link: [tekst](../projects/benstartpagina.md). #refactoring #go/patterns
```

- Links en tags in codeblokken en `inline code` worden genegeerd.
- Mappen die met `.` beginnen (`.git`, `.obsidian`, `.noot`) en `node_modules` worden overgeslagen.
- Twee bestanden met dezelfde `id` krijg je als waarschuwing te zien; het tweede bestand wordt dan overgeslagen.
- Een bestaande Obsidian-vault werkt gewoon. Logseq-graphs met Markdown ook, al zijn blok-referenties (`((uuid))`) nog niet ondersteund.

## Architectuur

```
cmd/noot/                 main
internal/domain/          Note, Link, NoteRef … (geen dependencies)
internal/parser/          Markdown → domain.Note (goldmark AST + frontmatter)
internal/vault/           Bestandslayout: root vinden, scannen, notities/journal aanmaken
internal/app/             Use cases: Sync, Watch, Resolve; definieert de Store-port
internal/storage/sqlite/  Store-implementatie (SQLite + FTS5)
internal/cli/             cobra-commando's en output
```

Afhankelijkheden lopen naar binnen: `cli → app → domain`, en `storage` implementeert `app.Store`. Wil je een andere zoekmotor (bleve, Meilisearch) of semantisch zoeken met embeddings, dan schrijf je een nieuwe Store. De rest blijft hetzelfde.

## Ideeën voor later

- `noot serve`: web-UI met `net/http` + htmx, met zoeken, backlinks en een graph (`/graph.json` + cytoscape.js)
- Semantisch zoeken: embeddings via Ollama naast FTS5 (sqlite-vec of een eigen tabel)
- `noot rename <oud> <nieuw>`: bestand hernoemen en alle `[[links]]` meteen bijwerken
- Auto-commit: `noot sync` die `git add/commit/pull --rebase/push` uitvoert
- Templates in `.noot/templates/` voor `new` en `today`
