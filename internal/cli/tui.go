package cli

import (
	"errors"
	"os"
	"time"

	"github.com/amavis442/noot/internal/tui"
	"github.com/amavis442/noot/internal/tui/kit"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func cmdTUI(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Interactive terminal UI: search, journal, agenda and reminders",
		Long: `Opens the selected vault in a full-screen terminal UI with four tabs:

  1 Search      results while you type ("#tag" filters), the note rendered beside them
  2 Today       the journal note of a day; [ and ] browse days
  3 Agenda      the weekly agenda, day by day, opened at today; [ and ] browse weeks
  4 Reminders   add (ctrl+n), tick off (x), plan into the agenda (p), show done ones (.)

The keys follow superfile's defaults: j/k or arrows move, enter/l/right opens,
h/left/backspace goes back, tab/L and H switch panels, / searches, f toggles the
preview, e opens what is shown in $VISUAL / $EDITOR, ? lists every key, q/esc
quits. While typing, enter confirms and esc/ctrl+c cancels. 1-4 switch tabs.

Needs an interactive terminal; scripts and agents use the other commands.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
				return errors.New("noot tui needs an interactive terminal; in scripts use search, today, agenda or reminder")
			}
			return run(cmd, opts, true, func(e *env) error {
				ports := tui.Ports{Notes: e.svc, Journal: e.svc, Agenda: e.svc, Reminders: e.svc}
				views := tui.Views(e.ctx, ports, tui.Options{
					Editor:   kit.External{Abs: e.svc.Vault.Abs},
					Markdown: kit.NewGlamour(markdownStyle()),
					Now:      time.Now,
				})
				_, err := tea.NewProgram(tui.New(e.name, views...), tea.WithAltScreen(), tea.WithContext(e.ctx)).Run()
				return err
			})
		},
	}
}

func isTerminal(f *os.File) bool { return isatty.IsTerminal(f.Fd()) }

// markdownStyle picks the glamour style for the terminal: none with NO_COLOR,
// else one for its background.
func markdownStyle() string {
	switch {
	case os.Getenv("NO_COLOR") != "":
		return "notty"
	case lipgloss.HasDarkBackground():
		return "dark"
	}
	return "light"
}
