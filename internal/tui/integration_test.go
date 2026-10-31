package tui_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/config"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/storage/sqlite"
	"github.com/amavis442/noot/internal/tui"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/tui/kit/kittest"
	"github.com/amavis442/noot/internal/vault"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

var now = time.Date(2026, 10, 31, 14, 0, 0, 0, time.Local)

// newVault returns a service over a fresh vault and index with one note.
func newVault(t *testing.T) *app.Service {
	t.Helper()
	v := &vault.Vault{Root: t.TempDir()}
	if err := v.Init(config.KindPersonal); err != nil {
		t.Fatal(err)
	}
	note := "---\ntitle: Zuurdesem\ntags: [bakken]\n---\n# Zuurdesem\n\nVoeden met rogge.\n"
	if err := os.WriteFile(filepath.Join(v.Root, "zuurdesem.md"), []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := sqlite.Open(v.IndexFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc := &app.Service{Vault: v, Store: st}
	if _, err := svc.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	return svc
}

func start(t *testing.T, svc *app.Service, ed kit.Editor) *teatest.TestModel {
	t.Helper()
	ports := tui.Ports{Notes: svc, Journal: svc, Agenda: svc, Reminders: svc}
	views := tui.Views(context.Background(), ports, tui.Options{Editor: ed, Markdown: kit.NewGlamour("notty"), Now: func() time.Time { return now }})
	return teatest.NewTestModel(t, tui.New("test", views...), teatest.WithInitialTermSize(110, 30))
}

func waitFor(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(text)) },
		teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func finalView(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	return tm.FinalModel(t, teatest.WithFinalTimeout(10*time.Second)).View()
}

func TestSearchShowsTheNoteFromTheIndex(t *testing.T) {
	tm := start(t, newVault(t), &kittest.Editor{})
	tm.Type("rogge")
	waitFor(t, tm, "Voeden met rogge")
	out := finalView(t, tm)
	if !strings.Contains(out, "zuurdesem.md") || strings.Contains(out, "title: Zuurdesem") {
		t.Fatalf("view:\n%s", out)
	}
}

func TestTabPicksASearchResult(t *testing.T) {
	svc := newVault(t)
	if err := os.WriteFile(svc.Vault.Abs("brood.md"), []byte("# Brood\n\nOok met rogge, 24 uur rijzen.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	hits, err := svc.Search(context.Background(), domain.SearchQuery{Text: "rogge", Limit: 50})
	if err != nil || len(hits) != 2 {
		t.Fatalf("hits = %+v, %v", hits, err)
	}
	ed := &kittest.Editor{}
	tm := start(t, svc, ed)
	tm.Type("rogge")
	waitFor(t, tm, "brood.md")
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	tm.Type("je")
	waitFor(t, tm, "▸ "+hits[1].Title)
	finalView(t, tm)
	if len(ed.Opened) != 1 || ed.Opened[0] != hits[1].Path {
		t.Fatalf("tab, j, e should open the second result %s, opened %v", hits[1].Path, ed.Opened)
	}
}
func TestJournalAndAgendaAreCreatedOnOpening(t *testing.T) {
	svc := newVault(t)
	tm := start(t, svc, &kittest.Editor{})
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc}) // stop typing in the search bar, so 2 switches tabs
	tm.Type("2")
	waitFor(t, tm, "zaterdag 31 oktober 2026")
	tm.Type("3")
	waitFor(t, tm, "Week 44")
	finalView(t, tm)
	for _, rel := range []string{"journal/2026-10-31.md", vault.AgendaPath(now)} {
		if _, err := os.Stat(svc.Vault.Abs(rel)); err != nil {
			t.Errorf("%s was not created: %v", rel, err)
		}
	}
}

func TestRemindersAreAddedAndPlannedInTheVault(t *testing.T) {
	svc := newVault(t)
	tm := start(t, svc, &kittest.Editor{})
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Type("4")
	waitFor(t, tm, "Geen open herinneringen")
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlN})
	tm.Type("Kaas kopen")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Type("monday")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitFor(t, tm, "Toegevoegd")
	tm.Type("p")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitFor(t, tm, "Gepland op")
	finalView(t, tm)

	rs, err := svc.Reminders(false)
	if err != nil || len(rs) != 1 || rs[0].Text != "Kaas kopen" || rs[0].Due != "2026-11-02" || rs[0].Planned != "2026-10-31" {
		t.Fatalf("reminders = %+v, %v", rs, err)
	}
	agenda, err := os.ReadFile(svc.Vault.Abs(vault.AgendaPath(now)))
	if err != nil || !strings.Contains(string(agenda), "Kaas kopen") {
		t.Fatalf("agenda:\n%s\n%v", agenda, err)
	}
}
