package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
)

func TestContext(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.Local)

	write(t, root, "projects/noot/noot.md", "---\ntitle: noot\naliases: [amavis442/noot]\n---\n# noot\n\nStatus: green.")
	write(t, root, "projects/noot/serve-plan.md", "# Serve plan\n\nNo links back.")
	write(t, root, "projects/noot/sub/deep.md", "# Deep\n\nNot a sibling.")
	write(t, root, "decisions/2026-10-24-vaults.md", "# Separate vaults\n\nApplies to [[noot]].")
	write(t, root, "logs/2026-10-25.md", "# Log 2026-10-25\n\n"+
		"## 00:47 · [[noot]]\n\nTemplate updated.\n\n"+
		"## 00:51 · [[trading212]]\n\nNothing about the project.\n\n"+
		"## 01:10\n\nMentions [[amavis442/noot]] by alias.\n")
	write(t, root, "logs/2026-10-24.md", "# Log 2026-10-24\n\n## 09:00 · [[noot]]\n\nYesterday.\n")
	write(t, root, "logs/2026-10-01.md", "# Log 2026-10-01\n\n## 09:00 · [[noot]]\n\nToo old.\n")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}

	nc, err := svc.Context(ctx, "noot", app.ContextOptions{Days: 7, Limit: 20, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if nc.Note.Path != "projects/noot/noot.md" || !strings.Contains(nc.Content, "Status: green.") {
		t.Errorf("note = %+v, content %q", nc.Note, nc.Content)
	}

	var related []string
	for _, r := range nc.Related {
		related = append(related, r.From.Path)
	}
	if strings.Join(related, ",") != "decisions/2026-10-24-vaults.md,projects/noot/serve-plan.md" {
		t.Errorf("related = %v", related)
	}
	if nc.Related[0].Context != "Applies to [[noot]]." {
		t.Errorf("related context = %q", nc.Related[0].Context)
	}

	var logs []string
	for _, l := range nc.Logs {
		logs = append(logs, l.Path+" "+l.Heading+": "+l.Text)
	}
	want := []string{
		"logs/2026-10-24.md 09:00 · [[noot]]: Yesterday.",
		"logs/2026-10-25.md 00:47 · [[noot]]: Template updated.",
		"logs/2026-10-25.md 01:10: Mentions [[amavis442/noot]] by alias.",
	}
	if strings.Join(logs, "\n") != strings.Join(want, "\n") {
		t.Errorf("logs =\n%s\nwant\n%s", strings.Join(logs, "\n"), strings.Join(want, "\n"))
	}

	// Limit keeps the newest entries.
	nc, _ = svc.Context(ctx, "noot", app.ContextOptions{Days: 7, Limit: 1, Now: now})
	if len(nc.Logs) != 1 || nc.Logs[0].Heading != "01:10" {
		t.Errorf("limited logs = %+v", nc.Logs)
	}

	if _, err := svc.Context(ctx, "nope", app.ContextOptions{Days: 7, Limit: 20, Now: now}); err == nil {
		t.Error("unknown note should fail")
	}
}

func TestContextSiblingsOnlyForFolderNotes(t *testing.T) {
	ctx := context.Background()
	svc, root := setup(t)
	write(t, root, "memory/owner.md", "# Owner")
	write(t, root, "memory/environment.md", "# Environment")
	if _, err := svc.Sync(ctx, false); err != nil {
		t.Fatal(err)
	}
	nc, err := svc.Context(ctx, "owner", app.ContextOptions{Days: 7, Limit: 20, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(nc.Related) != 0 {
		t.Errorf("a note that is not its folder's main note has no siblings: %+v", nc.Related)
	}
}
