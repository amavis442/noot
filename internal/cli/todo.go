package cli

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/spf13/cobra"
)

// groupLabels are the headings of the groups in noot todo.
var groupLabels = map[domain.TaskGroup]string{
	domain.GroupOverdue:  "Overdue",
	domain.GroupToday:    "Today",
	domain.GroupThisWeek: "This week",
	domain.GroupLater:    "Later",
	domain.GroupBacklog:  "Backlog",
	domain.GroupDone:     "Done",
}

func cmdTodo(opts *options) *cobra.Command {
	var today, week, all bool
	c := &cobra.Command{
		Use:   "todo",
		Short: "Every task in one list: reminders, the backlog and the weekly agendas",
		Long: `Lists the checkbox tasks of reminders.md, agenda/agenda.md (the backlog
without a day) and the weekly agendas (under Planned and Doing), grouped as
overdue, today, this week, later, backlog and done. A task is placed by its
earliest "due" or "planned" date; without one it is in the backlog.

A reminder that was planned into a week is listed once, as the reminder. Plain
bullets are not tasks. noot only reads the files here; tick tasks off with
"noot reminder done <n>" or in the todo tab of "noot tui".

Each line ends with the file and line number of the task.`,
		Example: `  noot todo             # every open task
  noot todo --today     # overdue and today
  noot todo --week      # ... and the rest of this week
  noot todo --all       # also the done ones
  noot todo --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope := app.TodoOpen
			switch {
			case today:
				scope = app.TodoToday
			case week:
				scope = app.TodoWeek
			case all:
				scope = app.TodoAll
			}
			return run(cmd, opts, false, func(e *env) error {
				now := time.Now()
				tasks, err := e.svc.Tasks(scope, now)
				if err != nil {
					return err
				}
				if opts.asJSON {
					type taskJSON struct {
						domain.Task
						Group domain.TaskGroup `json:"group"`
					}
					out := make([]taskJSON, 0, len(tasks))
					for _, t := range tasks {
						out = append(out, taskJSON{t, t.Group(now)})
					}
					return e.json(out)
				}
				return printTodo(e, tasks, now)
			})
		},
	}
	c.Flags().BoolVar(&today, "today", false, "only overdue tasks and those for today")
	c.Flags().BoolVar(&week, "week", false, "overdue, today and the rest of this week")
	c.Flags().BoolVarP(&all, "all", "a", false, "include done tasks")
	c.MarkFlagsMutuallyExclusive("today", "week", "all")
	return c
}

// printTodo lists tasks, which Tasks returns in group order, under a heading
// per group. Task and date columns are as wide as the widest entry, padded by
// the width of the plain text so colour codes do not shift them.
func printTodo(e *env, tasks []domain.Task, now time.Time) error {
	p := e.paint
	if len(tasks) == 0 {
		_, err := fmt.Fprintln(e.out, p.muted("nothing to do"))
		return err
	}
	type row struct{ text, meta, textStyled, metaStyled, src string }
	byGroup := map[domain.TaskGroup][]row{}
	textW, metaW := 0, 0
	for _, t := range tasks {
		g := t.Group(now)
		box := "[ ]"
		if t.Done {
			box = "[x]"
		}
		text := box + " " + t.Text
		textStyled := text
		if t.Done {
			textStyled = p.muted(text)
		}
		var meta, styled []string
		add := func(label, date string) {
			s := label + " " + date
			if g == domain.GroupOverdue && date == t.Date() {
				s += " (overdue)"
				meta, styled = append(meta, s), append(styled, p.danger(s))
				return
			}
			meta, styled = append(meta, s), append(styled, p.muted(s))
		}
		if t.Due != "" {
			add("due", t.Due)
		}
		if t.Planned != "" {
			add("planned", t.Planned)
		}
		if t.DoneOn != "" {
			add("done", t.DoneOn)
		}
		r := row{text: text, meta: strings.Join(meta, ", "), textStyled: textStyled, metaStyled: strings.Join(styled, ", "), src: fmt.Sprintf("%s:%d", t.File, t.Line)}
		textW, metaW = max(textW, utf8.RuneCountInString(r.text)), max(metaW, utf8.RuneCountInString(r.meta))
		byGroup[g] = append(byGroup[g], r)
	}
	first := true
	for _, g := range domain.TaskGroups {
		rows := byGroup[g]
		if len(rows) == 0 {
			continue
		}
		if !first {
			fmt.Fprintln(e.out)
		}
		first = false
		heading := fmt.Sprintf("%s (%d)", groupLabels[g], len(rows))
		if g == domain.GroupOverdue {
			heading = p.danger(heading)
		} else {
			heading = p.title(heading)
		}
		fmt.Fprintln(e.out, heading)
		for _, r := range rows {
			line := "  " + padRight(r.textStyled, r.text, textW+2) + padRight(r.metaStyled, r.meta, metaW+2) + p.muted(r.src)
			if _, err := fmt.Fprintln(e.out, line); err != nil {
				return err
			}
		}
	}
	return nil
}
