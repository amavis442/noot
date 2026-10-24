// Package cli is the cobra command tree. It only parses flags, calls the app
// layer and formats output.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/config"
	"github.com/amavis442/noot/internal/storage/sqlite"
	"github.com/amavis442/noot/internal/vault"
	"github.com/spf13/cobra"
)

// Version can be set at build time: -ldflags "-X github.com/amavis442/noot/internal/cli.Version=v0.2.0".
// When it is not, version() falls back to the module version or the git commit
// that Go embeds in the binary.
var Version = ""

var pseudoVersion = regexp.MustCompile(`\d{14}-[0-9a-f]{12}`)

func version() string {
	if Version != "" {
		return Version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	// A real tag (go install …/cmd/noot@v0.2.0) wins; pseudo-versions such as
	// v0.0.0-20261024124715-110c15d0fbf3+dirty are replaced by the short commit.
	if v := bi.Main.Version; v != "" && v != "(devel)" && !pseudoVersion.MatchString(v) {
		return v
	}
	rev, dirty := "", false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "dev"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		rev += "-dirty"
	}
	return rev
}

type options struct {
	vaultRef string // name from the config, or a path
	agent    bool   // use the configured agent vault
	noSync   bool
	asJSON   bool
}

// env is what every command gets after setup.
type env struct {
	ctx   context.Context
	svc   *app.Service
	store *sqlite.Store
	name  string // registered vault name, "" when unregistered
	out   io.Writer
	opts  *options
}

// NewRoot builds the command tree.
func NewRoot() *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:   "noot",
		Short: "noot — a second brain made of plain Markdown files",
		Long: `noot indexes folders of Markdown notes (frontmatter, #tags, [[wikilinks]])
so you can search, follow backlinks and spot orphans from the terminal.

Vault selection, first match wins:
  -A/--agent             the configured agent vault
  -V/--vault <name|path> a registered vault name or a directory
  $NOOT_VAULT            same, from the environment
  current directory      when it is (inside) a vault with .noot/
  default vault          from the config (noot vault default <name>)`,
		Version:       version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&opts.vaultRef, "vault", "V", "", "vault name or path")
	pf.BoolVarP(&opts.agent, "agent", "A", false, "use the agent vault")
	pf.BoolVar(&opts.noSync, "no-sync", false, "skip the automatic incremental index update")
	pf.BoolVar(&opts.asJSON, "json", false, "output JSON (for scripts and agents)")

	root.AddCommand(
		cmdInit(opts),
		cmdVault(opts),
		cmdIndex(opts),
		cmdSearch(opts),
		cmdBacklinks(opts),
		cmdLinks(opts),
		cmdBroken(opts),
		cmdOrphans(opts),
		cmdTags(opts),
		cmdNew(opts),
		cmdToday(opts),
		cmdLog(opts),
		cmdCat(opts),
		cmdAppend(opts),
		cmdOpen(opts),
		cmdPush(opts),
		cmdWatch(opts),
	)
	return root
}

// resolveVault applies the selection order documented on the root command.
func resolveVault(opts *options, cfg *config.Config) (root, name string, err error) {
	lookup := func(ref string) (string, string, error) {
		if v, ok := cfg.Get(ref); ok {
			return v.Path, ref, nil
		}
		if fi, err := os.Stat(ref); err == nil && fi.IsDir() {
			return ref, cfg.NameFor(ref), nil
		}
		return "", "", fmt.Errorf("%q is neither a registered vault (%s) nor a directory", ref, strings.Join(cfg.Names(), ", "))
	}
	switch {
	case opts.agent:
		if cfg.Agent == "" {
			return "", "", errors.New("no agent vault configured: noot init <dir> --agent --name <name>")
		}
		return lookup(cfg.Agent)
	case opts.vaultRef != "":
		return lookup(opts.vaultRef)
	case os.Getenv("NOOT_VAULT") != "":
		return lookup(os.Getenv("NOOT_VAULT"))
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	if dir, ok := vault.FindUp(wd); ok {
		return dir, cfg.NameFor(dir), nil
	}
	if cfg.Default != "" {
		return lookup(cfg.Default)
	}
	return wd, "", nil
}

// run opens the selected vault + index, optionally syncs, then calls fn.
func run(cmd *cobra.Command, opts *options, sync bool, fn func(e *env) error) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	root, name, err := resolveVault(opts, cfg)
	if err != nil {
		return err
	}
	return runAt(cmd, opts, root, name, sync, fn)
}

func runAt(cmd *cobra.Command, opts *options, root, name string, sync bool, fn func(e *env) error) error {
	v, err := vault.Open(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(v.Abs(vault.MetaDir), 0o755); err != nil {
		return err
	}
	store, err := sqlite.Open(v.IndexFile())
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer store.Close()

	e := &env{ctx: cmd.Context(), svc: &app.Service{Vault: v, Store: store}, store: store, name: name, out: cmd.OutOrStdout(), opts: opts}
	if sync && !opts.noSync {
		st, err := e.svc.Sync(e.ctx, false)
		if err != nil {
			return err
		}
		for _, w := range st.Errors {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
		}
	}
	return fn(e)
}

func (e *env) json(v any) error {
	enc := json.NewEncoder(e.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// openInEditor opens a file in $VISUAL / $EDITOR (falls back to notepad / vi).
func openInEditor(file string) error {
	ed := os.Getenv("VISUAL")
	if ed == "" {
		ed = os.Getenv("EDITOR")
	}
	if ed == "" {
		if runtime.GOOS == "windows" {
			ed = "notepad"
		} else {
			ed = "vi"
		}
	}
	parts := strings.Fields(ed) // allows EDITOR="code -w"
	c := exec.Command(parts[0], append(parts[1:], file)...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

// readText returns args joined, or stdin when args is empty or "-".
func readText(cmd *cobra.Command, args []string) (string, error) {
	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		b, err := io.ReadAll(cmd.InOrStdin())
		return string(b), err
	}
	return strings.Join(args, " "), nil
}
