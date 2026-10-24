package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

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
		Short: "Create a vault (folder layout, .noot/, .gitignore) and optionally register it",
		Example: `  noot init D:\Notes --name patrick
  noot init D:\Notes-claude --name claude --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			v, err := vault.Open(dir)
			if err != nil {
				return err
			}
			kind := config.KindPersonal
			if agent {
				kind = config.KindAgent
			}
			if err := v.Init(kind); err != nil {
				return err
			}
			if name != "" {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				if err := cfg.Add(name, v.Root, kind); err != nil {
					return err
				}
				if err := cfg.Save(); err != nil {
					return err
				}
			}
			return runAt(cmd, opts, v.Root, name, true, func(e *env) error {
				n, _ := e.store.Count(e.ctx)
				label := v.Root
				if name != "" {
					label = fmt.Sprintf("%q (%s)", name, v.Root)
				}
				fmt.Fprintf(e.out, "%s vault ready: %s, %d notes indexed\n", kind, label, n)
				return nil
			})
		},
	}
	c.Flags().StringVar(&name, "name", "", "register the vault under this name")
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
