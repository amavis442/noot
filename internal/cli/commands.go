package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/config"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/vault"
	"github.com/spf13/cobra"
)

func cmdInit(opts *options) *cobra.Command {
	var name string
	var agent bool
	c := &cobra.Command{
		Use:   "init [dir]",
		Short: "Create a vault (folder layout, .noot/, .gitignore) and register it",
		Long: `Creates the folder layout and registers the vault, so it can be selected by
name (-V) and, for the first personal and agent vault, as the default and -A.
Without --name the vault is registered under its folder name. A name that is
already used for another folder is refused before anything is created.`,
		Example: `  noot init D:\Noot\Patrick              # registered as "patrick"
  noot init ~/notes/work --name work
  noot init D:\Noot\Claude --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			kind := config.KindPersonal
			if agent {
				kind = config.KindAgent
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			name, err := cfg.Register(name, vault.Slugify(filepath.Base(abs)), abs, kind)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(abs, 0o755); err != nil {
				return err
			}
			v, err := vault.Open(abs)
			if err != nil {
				return err
			}
			if err := v.Init(kind); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			return runAt(cmd, opts, v.Root, name, true, func(e *env) error {
				n, _ := e.store.Count(e.ctx)
				fmt.Fprintf(e.out, "%s vault ready: %q (%s), %d notes indexed\n", kind, name, v.Root, n)
				return nil
			})
		},
	}
	c.Flags().StringVar(&name, "name", "", "register the vault under this name (default: the folder name)")
	c.Flags().BoolVar(&agent, "agent", false, "agent layout: AGENTS.md, logs/, memory/, decisions/, projects/, scratch/")
	return c
}

func cmdIndex(opts *options) *cobra.Command {
	var full bool
	c := &cobra.Command{
		Use:   "index",
		Short: "Update the index (incremental; --full rebuilds)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts, false, func(e *env) error {
				start := time.Now()
				st, err := e.svc.Sync(e.ctx, full)
				if err != nil {
					return err
				}
				for _, w := range st.Errors {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
				}
				fmt.Fprintf(e.out, "%s in %s\n", st, time.Since(start).Round(time.Millisecond))
				return nil
			})
		},
	}
	c.Flags().BoolVar(&full, "full", false, "drop the index and re-parse every file")
	return c
}

func cmdSearch(opts *options) *cobra.Command {
	var q domain.SearchQuery
	var all bool
	c := &cobra.Command{
		Use:   "search [terms...]",
		Short: "Full-text search (prefix match on every term; --raw for FTS5 syntax)",
		Example: `  noot search clean arch
  noot search --tag go repository
  noot search --all sqlite          # every registered vault
  noot search --raw 'golang NEAR/5 sqlite'`,
		Aliases: []string{"s"},
		RunE: func(cmd *cobra.Command, args []string) error {
			q.Text = strings.Join(args, " ")
			if !all {
				return run(cmd, opts, true, func(e *env) error {
					hits, err := e.store.Search(e.ctx, q)
					if err != nil {
						return err
					}
					if opts.asJSON {
						return e.json(hits)
					}
					printHits(e, "", hits)
					return nil
				})
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Vaults) == 0 {
				return fmt.Errorf("no vaults registered (noot vault add <name> <path>)")
			}
			type vaultHits struct {
				Vault string             `json:"vault"`
				Hits  []domain.SearchHit `json:"hits"`
			}
			var res []vaultHits
			for _, n := range cfg.Names() {
				vc, _ := cfg.Get(n)
				err := runAt(cmd, opts, vc.Path, n, true, func(e *env) error {
					hits, err := e.store.Search(e.ctx, q)
					res = append(res, vaultHits{n, hits})
					return err
				})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: vault %s: %v\n", n, err)
				}
			}
			if opts.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			e := &env{out: cmd.OutOrStdout()}
			total := 0
			for _, r := range res {
				total += len(r.Hits)
				printHits(e, r.Vault, r.Hits)
			}
			if total == 0 {
				fmt.Fprintln(e.out, "no matches")
			}
			return nil
		},
	}
	c.Flags().StringSliceVarP(&q.Tags, "tag", "t", nil, "only notes with this tag (repeatable)")
	c.Flags().IntVarP(&q.Limit, "limit", "n", 20, "max results (per vault with --all)")
	c.Flags().BoolVar(&q.Raw, "raw", false, "pass the query to FTS5 unmodified")
	c.Flags().BoolVarP(&all, "all", "a", false, "search every registered vault")
	return c
}

func printHits(e *env, vaultName string, hits []domain.SearchHit) {
	if vaultName == "" && len(hits) == 0 {
		fmt.Fprintln(e.out, "no matches")
	}
	for _, h := range hits {
		prefix := ""
		if vaultName != "" {
			prefix = "[" + vaultName + "] "
		}
		fmt.Fprintf(e.out, "%s%s  —  %s\n", prefix, h.Path, h.Title)
		if h.Snippet != "" {
			fmt.Fprintf(e.out, "    %s\n", h.Snippet)
		}
	}
}

func cmdBacklinks(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:     "backlinks <note>",
		Short:   "Notes that link to <note> (id, path, file name, title or alias)",
		Aliases: []string{"bl"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, opts, true, func(e *env) error {
				ref, err := e.svc.MustResolve(e.ctx, args[0])
				if err != nil {
					return err
				}
				bls, err := e.store.Backlinks(e.ctx, ref.ID)
				if err != nil {
					return err
				}
				if opts.asJSON {
					return e.json(bls)
				}
				fmt.Fprintf(e.out, "%d backlinks to %s\n", len(bls), ref.Path)
				for _, b := range bls {
					fmt.Fprintf(e.out, "  %s  —  %s\n", b.From.Path, b.From.Title)
					if b.Context != "" {
						fmt.Fprintf(e.out, "      %s\n", b.Context)
					}
				}
				return nil
			})
		},
	}
}

func cmdLinks(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "links <note>",
		Short: "Outgoing links of <note>; broken ones are marked with ✗",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, opts, true, func(e *env) error {
				ref, err := e.svc.MustResolve(e.ctx, args[0])
				if err != nil {
					return err
				}
				links, err := e.store.OutLinks(e.ctx, ref.ID)
				if err != nil {
					return err
				}
				if opts.asJSON {
					return e.json(links)
				}
				for _, l := range links {
					if l.Resolved != nil {
						fmt.Fprintf(e.out, "  ✓ %-30s → %s\n", l.Target, l.Resolved.Path)
					} else {
						fmt.Fprintf(e.out, "  ✗ %s\n", l.Target)
					}
				}
				return nil
			})
		},
	}
}

func cmdContext(opts *options) *cobra.Command {
	var co app.ContextOptions
	c := &cobra.Command{
		Use:   "context <note>",
		Short: "Print a note with the notes around it and the recent log entries that link to it",
		Long: `Prints everything worth reading before working on a note, in one go:
the note itself, the notes that link to it (daily logs excluded), the other
notes in its folder when it is the folder's main note (projects/x/x.md), and
the entries of recent daily logs that link to it.`,
		Example: `  noot -A context trading212-go-api
  noot -A context noot --days 14 -n 50
  noot -A context --json noot`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, opts, true, func(e *env) error {
				co.Now = time.Now()
				nc, err := e.svc.Context(e.ctx, args[0], co)
				if err != nil {
					return err
				}
				if opts.asJSON {
					return e.json(nc)
				}
				printContext(e, nc, co.Days)
				return nil
			})
		},
	}
	c.Flags().IntVar(&co.Days, "days", 7, "log entries of the last N days (0 = all)")
	c.Flags().IntVarP(&co.Limit, "limit", "n", 20, "at most N log entries, the newest (0 = all)")
	return c
}

func printContext(e *env, nc *domain.NoteContext, days int) {
	fmt.Fprintf(e.out, "==> %s (%s)\n%s", nc.Note.Path, nc.Note.Title, nc.Content)
	if !strings.HasSuffix(nc.Content, "\n") {
		fmt.Fprintln(e.out)
	}

	fmt.Fprintln(e.out, "\n==> related")
	if len(nc.Related) == 0 {
		fmt.Fprintln(e.out, "  (none)")
	}
	for _, r := range nc.Related {
		fmt.Fprintf(e.out, "  %s  —  %s\n", r.From.Path, r.From.Title)
		if r.Context != "" {
			fmt.Fprintf(e.out, "      %s\n", r.Context)
		}
	}

	if days > 0 {
		fmt.Fprintf(e.out, "\n==> log, last %d days\n", days)
	} else {
		fmt.Fprintln(e.out, "\n==> log")
	}
	if len(nc.Logs) == 0 {
		fmt.Fprintln(e.out, "  (none)")
	}
	for _, l := range nc.Logs {
		day := l.Path
		if d, ok := vault.LogDate(l.Path); ok {
			day = d.Format("2006-01-02")
		}
		fmt.Fprintf(e.out, "-- %s %s\n%s\n", day, l.Heading, l.Text)
	}
}

func cmdRename(opts *options) *cobra.Command {
	var ro app.RenameOptions
	c := &cobra.Command{
		Use:   "rename <note> [new-path]",
		Short: "Move a note and/or change its title, rewriting every link to it",
		Long: `Moves a note to new-path (vault-relative, ".md" optional; a bare name stays
in the same folder) and/or changes its title with --title. Every [[wikilink]]
and Markdown link that pointed to the note is rewritten in the form it was
written in (path, file name or title); links by id or alias keep working and
are left alone. The note's own relative Markdown links follow it to its new
folder. Links inside code are not touched.`,
		Example: `  noot -A rename noot projects/noot/noot
  noot rename "Clean architecture" --title "Clean architecture in Go"
  noot rename old-name new-name --dry-run`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				ro.To = args[1]
			}
			return run(cmd, opts, true, func(e *env) error {
				res, err := e.svc.Rename(e.ctx, args[0], ro)
				if err != nil {
					return err
				}
				for _, b := range res.Broken {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: broken link %s → %s\n", b.From.Path, b.Target)
				}
				if opts.asJSON {
					return e.json(res)
				}
				verb := "renamed"
				if res.DryRun {
					verb = "would rename"
				}
				fmt.Fprintf(e.out, "%s %s → %s, %d links in %d files\n", verb, res.From, res.To, res.Links, len(res.Changed))
				for _, p := range res.Changed {
					fmt.Fprintf(e.out, "  %s\n", p)
				}
				return nil
			})
		},
	}
	c.Flags().StringVar(&ro.Title, "title", "", "new title (frontmatter title and the matching H1)")
	c.Flags().BoolVarP(&ro.DryRun, "dry-run", "n", false, "show what would change without writing")
	return c
}

func cmdBroken(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "broken",
		Short: "All links that point to a non-existing note",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts, true, func(e *env) error {
				bs, err := e.store.Broken(e.ctx)
				if err != nil {
					return err
				}
				if opts.asJSON {
					return e.json(bs)
				}
				tw := tabwriter.NewWriter(e.out, 0, 4, 2, ' ', 0)
				for _, b := range bs {
					fmt.Fprintf(tw, "%s\t→ %s\n", b.From.Path, b.Target)
				}
				return tw.Flush()
			})
		},
	}
}

func cmdOrphans(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "orphans",
		Short: "Notes without any incoming or outgoing links",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts, true, func(e *env) error {
				refs, err := e.store.Orphans(e.ctx)
				if err != nil {
					return err
				}
				return printRefs(e, refs)
			})
		},
	}
}

func cmdTags(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "tags [tag]",
		Short: "List tags with counts, or the notes carrying [tag]",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, opts, true, func(e *env) error {
				if len(args) == 1 {
					refs, err := e.store.NotesByTag(e.ctx, args[0])
					if err != nil {
						return err
					}
					return printRefs(e, refs)
				}
				tags, err := e.store.Tags(e.ctx)
				if err != nil {
					return err
				}
				if opts.asJSON {
					return e.json(tags)
				}
				for _, t := range tags {
					fmt.Fprintf(e.out, "%5d  #%s\n", t.Count, t.Tag)
				}
				return nil
			})
		},
	}
}

func cmdNew(opts *options) *cobra.Command {
	var dir string
	var tags []string
	var open bool
	c := &cobra.Command{
		Use:   "new <title...>",
		Short: "Create a note with frontmatter (id, title, created, tags)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, opts, false, func(e *env) error {
				rel, err := e.svc.Vault.NewNote(strings.Join(args, " "), dir, tags, time.Now())
				if err != nil {
					return err
				}
				fmt.Fprintln(e.out, rel)
				if open {
					return openInEditor(e.svc.Vault.Abs(rel))
				}
				return nil
			})
		},
	}
	c.Flags().StringVarP(&dir, "dir", "d", "zettels", "folder inside the vault")
	c.Flags().StringSliceVarP(&tags, "tag", "t", nil, "tags (repeatable)")
	c.Flags().BoolVarP(&open, "open", "o", false, "open in $EDITOR afterwards")
	return c
}

func cmdToday(opts *options) *cobra.Command {
	var noOpen bool
	var offset int
	c := &cobra.Command{
		Use:   "today",
		Short: "Open (and create if needed) today's journal note",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts, false, func(e *env) error {
				rel, _, err := e.svc.Vault.Journal(time.Now().AddDate(0, 0, offset))
				if err != nil {
					return err
				}
				if noOpen {
					fmt.Fprintln(e.out, rel)
					return nil
				}
				return openInEditor(e.svc.Vault.Abs(rel))
			})
		},
	}
	c.Flags().BoolVar(&noOpen, "print", false, "print the path instead of opening the editor")
	c.Flags().IntVar(&offset, "offset", 0, "days from today, e.g. -1 for yesterday")
	return c
}

func cmdAgenda(opts *options) *cobra.Command {
	var noOpen bool
	var offset int
	c := &cobra.Command{
		Use:   "agenda",
		Short: "Open (and create if needed) this week's agenda: agenda/YYYY-Www.md, a heading per day",
		Example: `  noot agenda
  noot agenda --offset 1        # next week
  noot agenda --print`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts, false, func(e *env) error {
				rel, _, err := e.svc.Vault.Agenda(time.Now().AddDate(0, 0, 7*offset))
				if err != nil {
					return err
				}
				if noOpen {
					fmt.Fprintln(e.out, rel)
					return nil
				}
				return openInEditor(e.svc.Vault.Abs(rel))
			})
		},
	}
	c.Flags().BoolVar(&noOpen, "print", false, "print the path instead of opening the editor")
	c.Flags().IntVar(&offset, "offset", 0, "weeks from this week, e.g. 1 for next week")
	return c
}

func cmdOpen(opts *options) *cobra.Command {
	var printOnly bool
	c := &cobra.Command{
		Use:   "open <note>",
		Short: "Open a note by id, path, file name, title or alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, opts, true, func(e *env) error {
				ref, err := e.svc.MustResolve(e.ctx, args[0])
				if err != nil {
					return err
				}
				if printOnly {
					fmt.Fprintln(e.out, e.svc.Vault.Abs(ref.Path))
					return nil
				}
				return openInEditor(e.svc.Vault.Abs(ref.Path))
			})
		},
	}
	c.Flags().BoolVar(&printOnly, "print", false, "print the absolute path instead of opening it")
	return c
}

func cmdWatch(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Keep the index up to date while files change (Ctrl+C to stop)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opts, false, func(e *env) error {
				ctx, stop := signal.NotifyContext(e.ctx, os.Interrupt)
				defer stop()
				fmt.Fprintf(e.out, "watching %s\n", e.svc.Vault.Root)
				return e.svc.Watch(ctx, 300*time.Millisecond, func(f string, a ...any) {
					fmt.Fprintf(e.out, "%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(f, a...))
				})
			})
		},
	}
}

func printRefs(e *env, refs []domain.NoteRef) error {
	if e.opts.asJSON {
		return e.json(refs)
	}
	tw := tabwriter.NewWriter(e.out, 0, 4, 2, ' ', 0)
	for _, r := range refs {
		fmt.Fprintf(tw, "%s\t%s\n", r.Path, r.Title)
	}
	return tw.Flush()
}
