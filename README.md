# noot 🐿️

*Een second brain van gewone Markdown-bestanden.* Net als een eekhoorn verstop je je nootjes, en `noot` helpt je ze terug te vinden.

De bestanden zijn de bron van waarheid. `noot` bouwt er een index naast (SQLite + FTS5 in `.noot/index.db`). Die index is altijd weg te gooien en opnieuw op te bouwen. Je schrijft in je eigen editor, synct met git, en `noot` regelt zoeken, backlinks, tags en dagnotities.

## Installeren

```bash
go mod tidy                       # eerste keer: haalt de ontbrekende checksums op
make                              # → bin/noot (Linux) of bin\noot.exe (Windows)
make install                      # go install met versie-info → $GOPATH/bin
```

| Make-target | Wat |
|---|---|
| `make` / `make build` | Build voor dit systeem; `.exe` wordt automatisch toegevoegd op Windows |
| `make build-linux` / `make build-windows` | Cross-compile naar `bin/` |
| `make dist` | Release-binaries in `dist/` voor linux en windows (amd64 + arm64), met de web-frontend erin (bouwt eerst `make web`) |
| `make test` | `go vet` + alle tests: unit, integratie (echte SQLite) en end-to-end (`e2e/`: bouwt de binary en draait elk commando tegen tijdelijke vaults) |
| `make web` | Web-frontend bouwen naar `web/dist/`; de volgende `make` neemt hem op in de binary (Node.js nodig) |
| `make web-test` | Frontend-tests (Vitest; Node.js nodig) |
| `make run ARGS="search sqlite"` | Draaien zonder te installeren (`go run`) |
| `make clean` / `fmt` / `tidy` / `help` | |

De Makefile werkt met elke GNU make, op Linux en op Windows (native, MSYS2 of Git Bash), en vanuit cmd, PowerShell of bash. De recipes roepen alleen `go`, `make` en `echo` aan, dus het maakt niet uit welke shell make gebruikt. `noot --version` toont de git-commit die Go in de binary zet; voor een release geef je `make dist VERSION=v0.2.0` mee.

Voor `make` en `make test` heb je geen Node.js nodig. Zonder `make web` toont `noot serve` op `/` alleen een melding dat de frontend niet is ingebouwd; de JSON-API werkt dan wel.

Er is geen cgo nodig: de SQLite-driver (`ncruces/go-sqlite3`) draait als WebAssembly via wazero. Je hebt dus geen C-compiler nodig, en alle binaries zijn statisch.

De eerste keer dat noot draait, compileert hij SQLite (ongeveer 1 seconde) en bewaart het resultaat in `%LocalAppData%\noot\wazero` (Windows) of `~/.cache/noot/wazero` (Linux). Daarna start elk commando binnen enkele milliseconden. Een andere plek stel je in met `$NOOT_CACHE_DIR`.

## Snel starten

```bash
noot init D:\Noot\Patrick         # maakt journal/ zettels/ projects/ attachments/ .noot/ + .gitignore, geregistreerd als "patrick"
cd D:\Noot\Patrick
noot today                        # opent journal/2026-10-24.md in $EDITOR
noot agenda                       # opent agenda/2026-W43.md, de agenda van deze week
noot reminder "Tandarts bellen" --due vrijdag
noot new "Clean architecture in Go" -t go -t architectuur
noot search clean arch            # prefix-match, accenten worden genegeerd (cafe = café)
noot backlinks "clean architecture in go"
```

## Meerdere vaults: jij en je agent

Je kunt meerdere vaults een naam geven. Het idee is een vault voor jezelf en een aparte vault voor je AI-agent. Daarin houdt de agent zijn logs, geheugen en beslissingen bij, zonder in jouw notities te schrijven.

```bash
noot init D:\Noot\Patrick --name patrick           # persoonlijk: journal/ zettels/ projects/ attachments/
noot init D:\Noot\Claude  --name claude --agent    # agent: AGENTS.md logs/ memory/ decisions/ projects/ scratch/
noot vault list
#   A claude   agent     D:\Noot\Claude
#   * patrick  personal  D:\Noot\Patrick
```

`init` registreert een vault altijd. Zonder `--name` wordt de mapnaam de naam (`D:\Noot\Claude` wordt `claude`), en een map die al geregistreerd is, houdt zijn naam. Wordt een naam al gebruikt voor een andere map, dan stopt `init` voordat er iets wordt aangemaakt. Kies dan zelf een naam met `--name`. Zo overschrijft een vault nooit ongemerkt een andere.

Je kunt alle vaults onder één map zetten, bijvoorbeeld `D:\Noot\`. Een vault slaat submappen met een eigen `.noot/` altijd over, dus vaults kunnen niet per ongeluk elkaars notities indexeren.

De eerste persoonlijke vault wordt de **default** (`*`) en de eerste agent-vault de **agent-vault** (`A`). Dat kun je later veranderen met `noot vault default <naam>` en `noot vault agent <naam>`. De namen staan in `%AppData%\noot\config.yaml` (Windows) of `~/.config/noot/config.yaml` (Linux), of op de plek die je met `$NOOT_CONFIG` opgeeft.

**Welke vault wordt gebruikt?** De eerste regel die past, wint:

1. `-A` / `--agent`: de agent-vault
2. `-V <naam|pad>`: een geregistreerde naam of een map
3. `$NOOT_VAULT`: hetzelfde, maar dan uit de omgeving
4. de huidige map, als die in een vault met `.noot/` ligt
5. de default-vault

Speciaal voor de agent-vault zijn er een paar commando's, die je overigens in elke vault kunt gebruiken:

```bash
noot -A context noot                                  # projectnotitie + verwante notities + recente logregels
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
| `init [dir] [--name n] [--agent]` | Maakt de mappenstructuur, `.noot/` en `.gitignore` en registreert de vault, onder `--name` of anders onder de mapnaam |
| `vault list\|add\|remove\|default\|agent` | Vaults met een naam beheren |
| `log [-p project] [-t tag] <tekst>` | Regel met tijdstempel toevoegen aan `logs/JJJJ-MM-DD.md` (ook via stdin) |
| `cat <note>` | Notitie printen |
| `context <note> [--days 7] [-n 20]` | Notitie printen met de notities die ernaar linken of ernaast staan (bij `projects/x/x.md`), en de logregels van de laatste dagen die ernaar linken. Eén commando om een project op te pakken |
| `rename <note> [nieuw-pad] [--title t] [--dry-run]` | Notitie verplaatsen en/of de titel wijzigen; alle links ernaar worden bijgewerkt in de vorm waarin ze geschreven zijn (pad, bestandsnaam of titel). Links via id of alias blijven gewoon werken, links in code blijven staan |
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
| `agenda [--offset 1] [--print]` | Weekagenda openen of aanmaken: `agenda/JJJJ-Www.md` (ISO-week, maandag t/m zondag) met een kop per dag en daaronder Planned, Doing en Done; `--offset` telt in weken |
| `reminder [tekst] [--due dag] [--all]` (`rem`) | Met tekst: reminder toevoegen aan `reminders.md`. Zonder tekst: de open reminders, vroegste datum eerst (`--all` toont ook de afgeronde) |
| `reminder done <n>...` | Reminders afvinken; is de reminder ingepland, dan wordt hij ook in de agenda afgevinkt |
| `reminder plan <n> <dag>` | Reminder onder Planned van die dag in de weekagenda zetten (de agenda wordt zo nodig aangemaakt) |
| `open <note> [--print]` | Notitie openen via id, pad, bestandsnaam, titel of alias |
| `watch` | Index live bijhouden (Ctrl+C om te stoppen) |
| `serve [--addr 127.0.0.1:4242]` | Web-UI om te zoeken, notities te lezen en nieuwe te maken, plus een JSON-API (zie [Web-UI](#web-ui)). Houdt de index live bij zoals `watch` |
| `push [-m melding] [--no-pull]` | Alles committen en pushen: `git add -A`, commit, `pull --rebase`, `push` |
| `pull` | Wijzigingen van elders binnenhalen (`pull --rebase --autostash`) en de index bijwerken |
| `status` | Laten zien wat de volgende `push` meeneemt: gewijzigde bestanden (A/M/D) en commits die nog niet gepusht zijn. Kijkt alleen, zonder netwerk |

Overal kun je `--json` toevoegen voor scripts, fzf of een AI-agent. Bijvoorbeeld:

```bash
noot search --json repository | jq -r '.[].path' | fzf --preview 'bat --color=always {}'
```

`<note>` wordt in deze volgorde opgezocht: **id → pad → bestandsnaam → titel → alias**, zonder onderscheid tussen hoofd- en kleine letters. Diezelfde volgorde wordt gebruikt om `[[wikilinks]]` op te lossen.

## Web-UI

```bash
make web && make                  # eenmalig: frontend bouwen en in de binary zetten
noot serve                        # → http://127.0.0.1:4242
noot -A serve --addr 127.0.0.1:8080
```

`noot serve` serveert de vault die je ook met elk ander commando kiest (`-V`, `-A`, de huidige map of de default). In de UI zoek je met dezelfde zoekopdracht als `noot search`, open je een notitie (wikilinks en Markdown-links zijn klikbaar) en maak je een nieuwe notitie met een titel, optioneel tekst en tags. Nieuwe notities komen in `zettels/`.

De API, voor scripts:

| Endpoint | Wat |
|---|---|
| `GET /api/search?q=…&tag=…&limit=…` | Zoekresultaten, net als `noot search --json` |
| `GET /api/notes/{ref}` | Notitie via id, pad, bestandsnaam, titel of alias: `content` (het bestand) en `html` (gerenderd) |
| `POST /api/notes` | Nieuwe notitie; body `{"title": "…", "body": "…", "tags": ["…"]}` als `application/json` |

Op een loopback-adres (de standaard) beantwoordt de server alleen verzoeken voor `localhost`/`127.0.0.1`. Dat beschermt tegen DNS-rebinding. `POST` accepteert alleen JSON, zodat andere sites geen notities kunnen aanmaken. Er is geen login: bind niet op een openbaar adres.

### Frontend ontwikkelen

De frontend staat in `web/` (React + TypeScript, Vite, Vitest). Met Node.js:

```bash
cd web
npm ci
npm run dev                       # Vite op http://localhost:5173; /api gaat naar noot serve op :4242
npm test                          # Vitest + Testing Library
npm run build                     # typecheck + build naar web/dist/
```

Draai daarnaast `noot serve` voor de API. De code is per feature opgedeeld: `src/features/search` en `src/features/notes` (hooks voor de logica, componenten voor de weergave) en `src/shared` (API-client en routing).

## Synchroniseren met git

Een vault is gewoon een git-repo. `noot push` doet de hele ronde in één keer:

```bash
noot push                     # commit "noot: 3f9a1c2e4b7d" + lijst met gewijzigde bestanden, pull --rebase, push
noot push -m "Weekly review"  # eigen melding
noot -A push                  # de agent-vault
noot status                   # wat gaat er met de volgende push mee?
noot pull                     # binnenhalen wat elders is gepusht
```

- De standaardmelding is `noot: <hash>`: de eerste 12 tekens van de hash van de inhoud van de vault (`git write-tree`). Identieke inhoud geeft dus dezelfde hash. In de body staan de gewijzigde bestanden.
- Is er iets te pushen, dan eerst `git pull --rebase`, zodat een push vanaf een andere machine (of van je agent) wordt meegenomen in plaats van geweigerd. Daarna werkt noot de index bij.
- Is er niets te pushen, dan zegt noot meteen `nothing to push`, zonder contact met de remote. Dat scheelt de netwerktijd (bij GitHub al gauw 1 tot 2 seconden).
- Wijzigingen van elders binnenhalen doe je met `noot pull`. Lokale, nog niet gecommitte wijzigingen blijven daarbij staan.
- Heeft de branch nog geen upstream maar wel een `origin`, dan doet noot `git push -u origin HEAD`. Zonder remote wordt er alleen lokaal gecommit.
- `.noot/` (de index) komt nooit in git; noot zet het zo nodig in `.gitignore`.
- noot gebruikt `git` uit je PATH, dus je eigen config, credentials en SSH-sleutels.

## Conventies

```markdown
---
id: 01M5Q07ST0D58RJ8G68GBWRR3S   # optioneel; zonder id is het pad (zonder .md) de id
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
- `noot new` geeft elke notitie een ULID: 26 tekens, uniek zonder in de rest van de vault te kijken, en
  toch op aanmaaktijd gesorteerd. Een eigen id mag ook; elke tekst is goed, ook de tijdstempels
  (`20261024130512`) uit oudere notities.
- Een bestaande Obsidian-vault werkt gewoon. Logseq-graphs met Markdown ook, al zijn blok-referenties (`((uuid))`) nog niet ondersteund.

## Architectuur

```
cmd/noot/                 main
internal/domain/          Note, Link, NoteRef … (geen dependencies)
internal/parser/          Markdown → domain.Note (goldmark AST + frontmatter)
internal/vault/           Bestandslayout: root vinden, scannen, notities/journal aanmaken
internal/app/             Use cases: Sync, Watch, Resolve, CreateNote, Search, ReadNote; definieert de Store-port
internal/storage/sqlite/  Store-implementatie (SQLite + FTS5)
internal/cli/             cobra-commando's en output
internal/httpapi/         HTTP-adapter voor noot serve: JSON-API over de use cases + statische bestanden
web/                      React-frontend; web/dist wordt via go:embed in de binary gezet
```

Afhankelijkheden lopen naar binnen: `cli`/`httpapi → app → domain`, en `storage` implementeert `app.Store`. Wil je een andere zoekmotor (bleve, Meilisearch) of semantisch zoeken met embeddings, dan schrijf je een nieuwe Store. De rest blijft hetzelfde.

## Ideeën voor later

- Web-UI uitbreiden: backlinks en een graph (`/graph.json` + cytoscape.js)
- Semantisch zoeken: embeddings via Ollama naast FTS5 (sqlite-vec of een eigen tabel)
- Auto-commit: `noot sync` die `git add/commit/pull --rebase/push` uitvoert
- Templates in `.noot/templates/` voor `new` en `today`
