package app_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/app"
	"github.com/amavis442/noot/internal/domain"
)

func TestCreateNoteIsSearchableAndReadable(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)

	ref, err := svc.CreateNote(ctx, app.NoteDraft{Title: "Koffie zetten", Body: "Met een **V60** filter.", Tags: []string{"thuis"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Path != "zettels/koffie-zetten.md" || ref.Title != "Koffie zetten" || ref.ID == "" {
		t.Fatalf("ref = %+v", ref)
	}
	if _, err := os.Stat(svc.Vault.Abs(ref.Path)); err != nil {
		t.Fatal(err)
	}

	hits, err := svc.Search(ctx, domain.SearchQuery{Text: "v60"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != ref.ID {
		t.Fatalf("search v60 = %+v", hits)
	}
	hits, err = svc.Search(ctx, domain.SearchQuery{Tags: []string{"thuis"}})
	if err != nil || len(hits) != 1 {
		t.Fatalf("search by tag = %+v, %v", hits, err)
	}

	doc, err := svc.ReadNote(ctx, "Koffie zetten")
	if err != nil {
		t.Fatal(err)
	}
	if doc.NoteRef != *ref || !strings.Contains(doc.Content, "Met een **V60** filter.") {
		t.Errorf("doc = %+v", doc)
	}
}

func TestCreateNoteRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)
	now := time.Now()

	for name, d := range map[string]app.NoteDraft{
		"blank title":     {Title: "  "},
		"newline title":   {Title: "one\ntwo"},
		"tag with spaces": {Title: "x", Tags: []string{"two words"}},
		"tag with yaml":   {Title: "x", Tags: []string{"a]\nid: hack"}},
	} {
		if _, err := svc.CreateNote(ctx, d, now); !errors.Is(err, app.ErrInvalidNote) {
			t.Errorf("%s: err = %v, want ErrInvalidNote", name, err)
		}
	}
	if n, _ := svc.Store.Count(ctx); n != 0 {
		t.Errorf("invalid drafts created %d notes", n)
	}
}

func TestReadNoteUnknownIsNotFound(t *testing.T) {
	svc, _ := setup(t)
	_, err := svc.ReadNote(context.Background(), "nope")
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), `no note matches "nope"`) {
		t.Errorf("message = %q", err)
	}
}

func TestCreateNoteConcurrently(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t)

	// noot serve handles requests in parallel while Watch syncs in the background.
	const n = 8
	errs := make(chan error, n+1)
	for i := 0; i < n; i++ {
		go func(i int) {
			_, err := svc.CreateNote(ctx, app.NoteDraft{Title: fmt.Sprintf("Note %d", i)}, time.Now())
			errs <- err
		}(i)
	}
	go func() { _, err := svc.Sync(ctx, false); errs <- err }()
	for i := 0; i < n+1; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	if got, _ := svc.Store.Count(ctx); got != n {
		t.Errorf("indexed %d notes, want %d", got, n)
	}
}
