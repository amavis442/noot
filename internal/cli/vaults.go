package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/amavis442/noot/internal/config"
	"github.com/amavis442/noot/internal/vault"
	"github.com/spf13/cobra"
)

func cmdVault(opts *options) *cobra.Command {
	c := &cobra.Command{
		Use:   "vault",
		Short: "Manage named vaults (list, add, remove, default, agent)",
	}

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List registered vaults (* = default, A = agent)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if opts.asJSON {
				e := &env{out: cmd.OutOrStdout()}
				return e.json(cfg)
			}
			out := cmd.OutOrStdout()
			if len(cfg.Vaults) == 0 {
				fmt.Fprintf(out, "no vaults registered yet (config: %s)\n", cfg.Path())
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			for _, n := range cfg.Names() {
				v, _ := cfg.Get(n)
				mark := " "
				switch n {
				case cfg.Default:
					mark = "*"
				case cfg.Agent:
					mark = "A"
				}
				kind := v.Kind
				if kind == "" {
					kind = config.KindPersonal
				}
				fmt.Fprintf(tw, "%s %s\t%s\t%s\n", mark, n, kind, v.Path)
			}
			fmt.Fprintf(tw, "\nconfig: %s\n", cfg.Path())
			return tw.Flush()
		},
	}

	var kindFlag string
	add := &cobra.Command{
		Use:   "add <name> <path>",
		Short: "Register an existing folder as a vault",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := config.ParseKind(kindFlag)
			if err != nil {
				return err
			}
			if fi, err := os.Stat(args[1]); err != nil || !fi.IsDir() {
				return fmt.Errorf("%s is not a directory (use noot init to create one)", args[1])
			}
			return editConfig(cmd, func(cfg *config.Config) (string, error) {
				return fmt.Sprintf("registered %s → %s", args[0], args[1]), cfg.Add(args[0], args[1], kind)
			})
		},
	}
	add.Flags().StringVar(&kindFlag, "kind", "personal", "personal or agent")

	remove := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Unregister a vault (files are left untouched)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return editConfig(cmd, func(cfg *config.Config) (string, error) {
				return "removed " + args[0], cfg.Remove(args[0])
			})
		},
	}

	setter := func(use, short string, set func(*config.Config, string)) *cobra.Command {
		return &cobra.Command{
			Use:   use + " <name>",
			Short: short,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return editConfig(cmd, func(cfg *config.Config) (string, error) {
					if _, ok := cfg.Get(args[0]); !ok {
						return "", fmt.Errorf("no vault named %q", args[0])
					}
					set(cfg, args[0])
					return fmt.Sprintf("%s vault is now %s", use, args[0]), nil
				})
			},
		}
	}

	c.AddCommand(list, add, remove,
		setter("default", "Vault used when nothing else selects one", func(c *config.Config, n string) { c.Default = n }),
		setter("agent", "Vault used by -A/--agent", func(c *config.Config, n string) { c.Agent = n }),
	)
	return c
}

func editConfig(cmd *cobra.Command, fn func(*config.Config) (string, error)) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	msg, err := fn(cfg)
	if err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	return nil
}

func cmdLog(opts *options) *cobra.Command {
	var entry vault.LogEntry
	c := &cobra.Command{
		Use:   "log [text...]",
		Short: "Append a timestamped entry to logs/YYYY-MM-DD.md (text from args or stdin)",
		Example: `  noot -A log -p noot -t release "Tagged v0.2.0"
  git log -1 --oneline | noot -A log -p noot`,
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := readText(cmd, args)
			if err != nil {
				return err
			}
			entry.Text = text
			return run(cmd, opts, false, func(e *env) error {
				rel, err := e.svc.Vault.AppendLog(entry, time.Now())
				if err != nil {
					return err
				}
				fmt.Fprintln(e.out, rel)
				return nil
			})
		},
	}
	c.Flags().SetInterspersed(false) // text like "- [ ] todo" is not parsed as flags
	c.Flags().StringVarP(&entry.Project, "project", "p", "", "project name, written as [[project]]")
	c.Flags().StringSliceVarP(&entry.Tags, "tag", "t", nil, "tags for this entry (repeatable)")
	return c
}

func cmdCat(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "cat <note>",
		Short: "Print a note (by id, path, file name, title or alias)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, opts, true, func(e *env) error {
				ref, err := e.svc.MustResolve(e.ctx, args[0])
				if err != nil {
					return err
				}
				b, err := os.ReadFile(e.svc.Vault.Abs(ref.Path))
				if err != nil {
					return err
				}
				if opts.asJSON {
					return e.json(map[string]string{"id": ref.ID, "path": ref.Path, "title": ref.Title, "content": string(b)})
				}
				_, err = e.out.Write(b)
				return err
			})
		},
	}
}

func cmdAppend(opts *options) *cobra.Command {
	var create bool
	c := &cobra.Command{
		Use:   "append <note> [text...]",
		Short: "Append text to a note (text from args or stdin)",
		Example: `  noot -A append projects/noot "- [ ] add noot serve"
  noot -A append --create scratch/ideas.md - < ideas.txt`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := readText(cmd, args[1:])
			if err != nil {
				return err
			}
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("nothing to append")
			}
			return run(cmd, opts, true, func(e *env) error {
				rel := ""
				ref, err := e.store.Resolve(e.ctx, args[0])
				switch {
				case err != nil:
					return err
				case ref != nil:
					rel = ref.Path
				case create:
					rel = filepath.ToSlash(strings.TrimSuffix(args[0], ".md") + ".md")
					if strings.HasPrefix(rel, "../") || filepath.IsAbs(args[0]) {
						return fmt.Errorf("%s is outside the vault", args[0])
					}
				default:
					return fmt.Errorf("no note matches %q (use --create to make %s.md)", args[0], strings.TrimSuffix(args[0], ".md"))
				}
				if err := e.svc.Vault.Append(rel, text); err != nil {
					return err
				}
				fmt.Fprintln(e.out, rel)
				return nil
			})
		},
	}
	c.Flags().SetInterspersed(false) // flags go before <note>; text like "- [ ] todo" stays text
	c.Flags().BoolVar(&create, "create", false, "create the note (as a vault-relative path) if it does not exist")
	return c
}
