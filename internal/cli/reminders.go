package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/amavis442/noot/internal/vault"
	"github.com/spf13/cobra"
)

func cmdReminder(opts *options) *cobra.Command {
	var due string
	var all bool
	c := &cobra.Command{
		Use:     "reminder [text...]",
		Aliases: []string{"rem"},
		Short:   "Keep reminders in reminders.md: add, list, mark done or plan them in the agenda",
		Long: `Reminders are Markdown tasks in reminders.md at the root of the vault, one
per line, numbered in file order. Done reminders stay in the file, so the
numbers do not shift.

With text, "noot reminder" adds a reminder; without, it lists the open ones.
Quote text that starts with a subcommand name (add, list, done, plan).`,
		Example: `  noot reminder "Call the dentist" --due friday
  noot reminder                     # open reminders, earliest date first
  noot reminder done 2
  noot reminder plan 1 tomorrow     # under Planned in this week's agenda`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return listReminders(cmd, opts, all)
			}
			return addReminder(cmd, opts, strings.Join(args, " "), due)
		},
	}
	c.Flags().StringVarP(&due, "due", "d", "", "due date: YYYY-MM-DD, today, tomorrow, +N or a weekday")
	c.Flags().BoolVarP(&all, "all", "a", false, "list done reminders too")
	c.AddCommand(cmdReminderAdd(opts), cmdReminderList(opts), cmdReminderDone(opts), cmdReminderPlan(opts))
	return c
}

func cmdReminderAdd(opts *options) *cobra.Command {
	var due string
	c := &cobra.Command{
		Use:   "add [text...]",
		Short: "Add a reminder (text from args or stdin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := readText(cmd, args)
			if err != nil {
				return err
			}
			return addReminder(cmd, opts, text, due)
		},
	}
	c.Flags().StringVarP(&due, "due", "d", "", "due date: YYYY-MM-DD, today, tomorrow, +N or a weekday")
	return c
}

func cmdReminderList(opts *options) *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Open reminders, earliest due or planned date first",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return listReminders(cmd, opts, all)
		},
	}
	c.Flags().BoolVarP(&all, "all", "a", false, "list done reminders too")
	return c
}

func cmdReminderDone(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "done <n>...",
		Short: "Mark reminders done (and tick them in the agenda when planned)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, err := reminderNumbers(args)
			if err != nil {
				return err
			}
			return run(cmd, opts, false, func(e *env) error {
				var done []vault.Reminder
				for _, n := range ns {
					r, err := e.svc.CompleteReminder(n, time.Now())
					if err != nil {
						return err
					}
					done = append(done, r)
				}
				if opts.asJSON {
					return e.json(done)
				}
				for _, r := range done {
					fmt.Fprintf(e.out, "done %d: %s\n", r.N, r.Text)
				}
				return nil
			})
		},
	}
}

func cmdReminderPlan(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "plan <n> <day>",
		Short: "Put a reminder under Planned of <day> in that week's agenda",
		Long: `Adds the reminder as "- [ ] text ([[reminders]])" under "### Planned" of
<day> in agenda/YYYY-Www.md (created when missing) and notes the day on the
reminder. <day> is YYYY-MM-DD, today, tomorrow, +N or a weekday (the next one).`,
		Example: `  noot reminder plan 3 thursday
  noot reminder plan 3 2026-11-04`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, err := reminderNumbers(args[:1])
			if err != nil {
				return err
			}
			day, err := vault.ParseDay(args[1], time.Now())
			if err != nil {
				return err
			}
			return run(cmd, opts, false, func(e *env) error {
				r, rel, err := e.svc.PlanReminder(ns[0], day)
				if err != nil {
					return err
				}
				if opts.asJSON {
					return e.json(map[string]any{"reminder": r, "agenda": rel})
				}
				fmt.Fprintf(e.out, "planned %d on %s: %s\n", r.N, day.Format("Monday 2 January"), rel)
				return nil
			})
		},
	}
}

func addReminder(cmd *cobra.Command, opts *options, text, due string) error {
	var day time.Time
	if due != "" {
		var err error
		if day, err = vault.ParseDay(due, time.Now()); err != nil {
			return err
		}
	}
	return run(cmd, opts, false, func(e *env) error {
		r, err := e.svc.AddReminder(text, day, time.Now())
		if err != nil {
			return err
		}
		if opts.asJSON {
			return e.json(r)
		}
		fmt.Fprintf(e.out, "added %d: %s\n", r.N, r.Text)
		return nil
	})
}

func listReminders(cmd *cobra.Command, opts *options, all bool) error {
	return run(cmd, opts, false, func(e *env) error {
		rs, err := e.svc.Reminders(all)
		if err != nil {
			return err
		}
		if opts.asJSON {
			return e.json(rs)
		}
		return printReminders(e, rs, time.Now().Format("2006-01-02"))
	})
}

// printReminders lists rs in three columns: number, box and text, dates.
// The columns are padded as text/tabwriter did before the output had
// colours, by the width of the plain text, so colour codes do not shift them.
func printReminders(e *env, rs []vault.Reminder, today string) error {
	p := e.paint
	if len(rs) == 0 {
		_, err := fmt.Fprintln(e.out, p.muted("no open reminders"))
		return err
	}
	type row struct{ num, text, meta, textStyled, metaStyled string }
	rows := make([]row, 0, len(rs))
	numW, textW := 0, 0
	for _, r := range rs {
		box := "[ ]"
		if r.Done {
			box = "[x]"
		}
		var meta, styled []string
		if r.Due != "" {
			due := "due " + r.Due
			if !r.Done && r.Due < today {
				due += " (overdue)"
				styled = append(styled, p.danger(due))
			} else {
				styled = append(styled, p.muted(due))
			}
			meta = append(meta, due)
		}
		if r.Planned != "" {
			meta = append(meta, "planned "+r.Planned)
			styled = append(styled, p.muted("planned "+r.Planned))
		}
		if r.DoneOn != "" {
			meta = append(meta, "done "+r.DoneOn)
			styled = append(styled, p.muted("done "+r.DoneOn))
		}
		text := box + " " + r.Text
		textStyled := text
		if r.Done {
			textStyled = p.muted(text)
		}
		rw := row{num: fmt.Sprintf("%3d", r.N), text: text, meta: strings.Join(meta, ", "), textStyled: textStyled, metaStyled: strings.Join(styled, ", ")}
		numW, textW = max(numW, utf8.RuneCountInString(rw.num)), max(textW, utf8.RuneCountInString(rw.text))
		rows = append(rows, rw)
	}
	const gap = 2 // tabwriter padding
	for _, rw := range rows {
		line := padRight(p.muted(rw.num), rw.num, numW+gap) + padRight(rw.textStyled, rw.text, textW+gap) + rw.metaStyled
		if _, err := fmt.Fprintln(e.out, line); err != nil {
			return err
		}
	}
	return nil
}

// padRight pads styled, whose plain text is plain, with spaces to width runes.
func padRight(styled, plain string, width int) string {
	return styled + strings.Repeat(" ", max(width-utf8.RuneCountInString(plain), 0))
}
func reminderNumbers(args []string) ([]int, error) {
	var ns []int
	for _, a := range args {
		n, err := strconv.Atoi(strings.TrimPrefix(a, "#"))
		if err != nil || n < 1 {
			return nil, fmt.Errorf("%q is not a reminder number (see noot reminder list)", a)
		}
		ns = append(ns, n)
	}
	return ns, nil
}
