package search_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
	"github.com/amavis442/noot/internal/tui/kit"
	"github.com/amavis442/noot/internal/tui/kit/kittest"
	"github.com/amavis442/noot/internal/tui/search"
)

// fakeNotes answers searches from a fixed set of notes.
type fakeNotes struct {
	notes   map[string]string // path -> content
	queries []domain.SearchQuery
	syncs   int
	fail    error
}

func (f *fakeNotes) Search(_ context.Context, q domain.SearchQuery) ([]domain.SearchHit, error) {
	f.queries = append(f.queries, q)
	if f.fail != nil {
		return nil, f.fail
	}
	var hits []domain.SearchHit
	for _, p := range sortedKeys(f.notes) {
		if strings.Contains(strings.ToLower(f.notes[p]), strings.ToLower(q.Text)) {
			title := strings.TrimSuffix(p, ".md")
			hits = append(hits, domain.SearchHit{NoteRef: domain.NoteRef{ID: title, Path: p, Title: title}})
		}
	}
	return hits, nil
}

func (f *fakeNotes) ReadNote(_ context.Context, ref string) (*domain.Document, error) {
	c, ok := f.notes[ref]
	if !ok {
		return nil, app.ErrNotFound
	}
	return &domain.Document{NoteRef: domain.NoteRef{Path: ref}, Content: c}, nil
}

func (f *fakeNotes) Sync(context.Context, bool) (app.SyncStats, error) {
	f.syncs++
	return app.SyncStats{}, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func newModel() (*search.Model, *fakeNotes, *kittest.Editor) {
	notes := &fakeNotes{notes: map[string]string{
		"brood.md":     "# Brood\n\nVoeden met rogge.",
		"zuurdesem.md": "# Zuurdesem\n\nOok rogge, en spelt.",
		"koffie.md":    "# Koffie\n\nBonen.",
	}}
	ed := &kittest.Editor{}
	m := search.New(context.Background(), notes, kit.Plain{}, ed)
	m.SetSize(100, 20)
	return m, notes, ed
}

func TestTypingSearchesLiveAndPreviewsTheFirstHit(t *testing.T) {
	m, _, _ := newModel()
	kittest.Run(t, m, m.Init())
	if !m.Capturing() {
		t.Fatal("the search field has the keyboard at first")
	}
	kittest.Type(t, m, "rogge")
	out := m.View()
	for _, want := range []string{"brood", "zuurdesem", "Voeden met rogge"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "koffie") {
		t.Fatalf("koffie does not match:\n%s", out)
	}
}

func TestMovingTheSelectionChangesThePreview(t *testing.T) {
	m, _, _ := newModel()
	kittest.Run(t, m, m.Init())
	kittest.Type(t, m, "rogge", "down")
	if out := m.View(); !strings.Contains(out, "Ook rogge, en spelt") {
		t.Fatalf("down previews the second hit:\n%s", out)
	}
	kittest.Type(t, m, "enter", "k")
	if out := m.View(); !strings.Contains(out, "Voeden met rogge") {
		t.Fatalf("k in the list goes back up:\n%s", out)
	}
}

func TestHashWordsBecomeTags(t *testing.T) {
	m, notes, _ := newModel()
	kittest.Type(t, m, "rogge #bakken")
	q := notes.queries[len(notes.queries)-1]
	if q.Text != "rogge" || !slices.Equal(q.Tags, []string{"bakken"}) {
		t.Fatalf("query = %+v", q)
	}
}

func TestStaleResultsAreDropped(t *testing.T) {
	m, _, _ := newModel()
	// Typing "ko" fires two searches; only the answer for "ko" may show.
	first := m.Update(kittest.Key("k"))
	second := m.Update(kittest.Key("o"))
	kittest.Run(t, m, second)
	kittest.Run(t, m, first) // the answer for "k" arrives late
	if out := m.View(); !strings.Contains(out, "koffie") || strings.Contains(out, "zuurdesem") {
		t.Fatalf("a stale result replaced the current one:\n%s", out)
	}
}

func TestEditOpensTheSelectedNoteAndRefreshes(t *testing.T) {
	m, notes, ed := newModel()
	kittest.Type(t, m, "Bonen", "enter")
	if m.Capturing() {
		t.Fatal("enter moves the keyboard to the results")
	}
	notes.notes["koffie.md"] = "# Koffie\n\nBonen, vers gemalen."
	kittest.Type(t, m, "e")
	if !slices.Equal(ed.Opened, []string{"koffie.md"}) {
		t.Fatalf("opened %v", ed.Opened)
	}
	if notes.syncs != 1 {
		t.Fatalf("the index is synced after editing, syncs = %d", notes.syncs)
	}
	if out := m.View(); !strings.Contains(out, "vers gemalen") {
		t.Fatalf("the preview shows the edited note:\n%s", out)
	}
	kittest.Type(t, m, "/")
	if !m.Capturing() {
		t.Fatal("/ goes back to the search field")
	}
}

func TestRefreshKeepsTheSelection(t *testing.T) {
	m, _, ed := newModel()
	kittest.Type(t, m, "rogge", "down", "enter", "e")
	if !slices.Equal(ed.Opened, []string{"zuurdesem.md"}) {
		t.Fatalf("opened %v", ed.Opened)
	}
	kittest.Type(t, m, "e")
	if !slices.Equal(ed.Opened, []string{"zuurdesem.md", "zuurdesem.md"}) {
		t.Fatalf("the selection moved after refreshing: opened %v", ed.Opened)
	}
}

func TestEmptyQueryAndErrorsAreShown(t *testing.T) {
	m, notes, _ := newModel()
	kittest.Run(t, m, m.Init())
	if out := m.View(); !strings.Contains(out, "Typ om te zoeken") {
		t.Fatalf("hint missing:\n%s", out)
	}
	kittest.Type(t, m, "xyz")
	if out := m.View(); !strings.Contains(out, "Geen resultaten") {
		t.Fatalf("no-results message missing:\n%s", out)
	}
	notes.fail = errors.New("index kapot")
	kittest.Type(t, m, "backspace")
	if out := m.View(); !strings.Contains(out, "index kapot") {
		t.Fatalf("error missing:\n%s", out)
	}
	kittest.Type(t, m, "backspace", "backspace")
	if out := m.View(); !strings.Contains(out, "Typ om te zoeken") {
		t.Fatalf("clearing the query clears the results:\n%s", out)
	}
}
