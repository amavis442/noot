package tui

import (
	"context"
	"time"

	"github.com/amavis442/noot/internal/tui/agenda"
	"github.com/amavis442/noot/internal/tui/journal"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/tui/reminders"
	"github.com/amavis442/noot/internal/tui/search"
)

// Ports are the app use cases the views call; *app.Service implements each.
type Ports struct {
	Notes     search.Notes
	Journal   journal.Journal
	Agenda    agenda.Agenda
	Reminders reminders.Reminders
}

// Options are what the views share besides the use cases.
type Options struct {
	Editor   kit.Editor
	Markdown kit.Markdown
	Now      func() time.Time
}

// Views returns the tabs in their order: search, today, agenda, reminders.
func Views(ctx context.Context, p Ports, o Options) []kit.View {
	return []kit.View{
		search.New(ctx, p.Notes, o.Markdown, o.Editor),
		journal.New(ctx, p.Journal, o.Markdown, o.Editor, o.Now),
		agenda.New(ctx, p.Agenda, o.Markdown, o.Editor, o.Now),
		reminders.New(p.Reminders, o.Editor, o.Now),
	}
}
