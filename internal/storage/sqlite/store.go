// Package sqlite implements app.Store on top of SQLite + FTS5, using a
// cgo-free driver so `go build` works the same on Windows and Linux.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/amavis442/noot/internal/domain"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

const schemaVersion = 2 // 2: link context keeps inline code

const schema = `
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);
CREATE TABLE IF NOT EXISTS notes (
	id         TEXT PRIMARY KEY,
	path       TEXT NOT NULL UNIQUE,
	path_norm  TEXT NOT NULL,
	stem_norm  TEXT NOT NULL,
	title      TEXT NOT NULL,
	title_norm TEXT NOT NULL,
	created    TEXT,
	mtime      INTEGER NOT NULL,
	size       INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS notes_stem  ON notes(stem_norm);
CREATE INDEX IF NOT EXISTS notes_title ON notes(title_norm);
CREATE TABLE IF NOT EXISTS aliases (note_id TEXT NOT NULL, alias_norm TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS aliases_alias ON aliases(alias_norm);
CREATE INDEX IF NOT EXISTS aliases_note  ON aliases(note_id);
CREATE TABLE IF NOT EXISTS tags (note_id TEXT NOT NULL, tag TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS tags_tag  ON tags(tag);
CREATE INDEX IF NOT EXISTS tags_note ON tags(note_id);
CREATE TABLE IF NOT EXISTS links (
	src_id      TEXT NOT NULL,
	target      TEXT NOT NULL,
	target_norm TEXT NOT NULL,
	kind        TEXT NOT NULL,
	context     TEXT,
	dst_id      TEXT
);
CREATE INDEX IF NOT EXISTS links_src ON links(src_id);
CREATE INDEX IF NOT EXISTS links_dst ON links(dst_id);
CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
	id UNINDEXED, title, body, tags,
	tokenize = 'unicode61 remove_diacritics 2'
);
`

// Store is a SQLite-backed index.
type Store struct{ db *sql.DB }

// Open opens (or creates) the index database at file.
func Open(file string) (*Store, error) {
	configureRuntime()
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // CLI tool: one connection keeps pragmas simple
	for _, p := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = OFF",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	var v int
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema'`).Scan(&v)
	if err == nil && v == schemaVersion {
		return nil
	}
	// Unknown or older schema: the index is derived data, so just rebuild it.
	for _, t := range []string{"notes", "aliases", "tags", "links", "notes_fts", "meta"} {
		if _, err := s.db.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	_, err = s.db.Exec(`INSERT INTO meta(key, value) VALUES ('schema', ?)`, schemaVersion)
	return err
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Reset removes everything from the index.
func (s *Store) Reset(ctx context.Context) error {
	for _, t := range []string{"notes", "aliases", "tags", "links", "notes_fts"} {
		if _, err := s.db.ExecContext(ctx, "DELETE FROM "+t); err != nil {
			return err
		}
	}
	return nil
}

// FileStates returns path → state for every indexed file.
func (s *Store) FileStates(ctx context.Context) (map[string]domain.FileState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, mtime, size FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]domain.FileState{}
	for rows.Next() {
		var p string
		var st domain.FileState
		if err := rows.Scan(&p, &st.ModTime, &st.Size); err != nil {
			return nil, err
		}
		out[p] = st
	}
	return out, rows.Err()
}

// Upsert writes the given notes in one transaction. A note whose id is
// already used by another file is skipped and reported in skipped.
func (s *Store) Upsert(ctx context.Context, notes []domain.Note) (skipped []error, err error) {
	if len(notes) == 0 {
		return nil, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, n := range notes {
		// Replace whatever was stored for this file before.
		if err := deleteWhere(ctx, tx, `SELECT id FROM notes WHERE path = ?`, n.Path); err != nil {
			return nil, err
		}
		var other string
		switch err := tx.QueryRowContext(ctx, `SELECT path FROM notes WHERE id = ?`, n.ID).Scan(&other); {
		case err == nil:
			skipped = append(skipped, fmt.Errorf("%s: id %q is already used by %s, skipped", n.Path, n.ID, other))
			continue
		case !errors.Is(err, sql.ErrNoRows):
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notes(id, path, path_norm, stem_norm, title, title_norm, created, mtime, size)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n.ID, n.Path, norm(n.Path), norm(strings.TrimSuffix(path.Base(n.Path), path.Ext(n.Path))),
			n.Title, norm(n.Title), n.Created, n.ModTime, n.Size); err != nil {
			return nil, fmt.Errorf("insert %s: %w", n.Path, err)
		}
		for _, a := range n.Aliases {
			if _, err := tx.ExecContext(ctx, `INSERT INTO aliases(note_id, alias_norm) VALUES (?, ?)`, n.ID, norm(a)); err != nil {
				return nil, err
			}
		}
		for _, t := range n.Tags {
			if _, err := tx.ExecContext(ctx, `INSERT INTO tags(note_id, tag) VALUES (?, ?)`, n.ID, t); err != nil {
				return nil, err
			}
		}
		for _, l := range n.Links {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO links(src_id, target, target_norm, kind, context) VALUES (?, ?, ?, ?, ?)`,
				n.ID, l.Target, norm(l.Target), string(l.Kind), l.Context); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notes_fts(id, title, body, tags) VALUES (?, ?, ?, ?)`,
			n.ID, n.Title, n.Body, strings.Join(n.Tags, " ")); err != nil {
			return nil, err
		}
	}
	return skipped, tx.Commit()
}

// DeletePaths removes notes by vault-relative path.
func (s *Store) DeletePaths(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range paths {
		if err := deleteWhere(ctx, tx, `SELECT id FROM notes WHERE path = ?`, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func deleteWhere(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		for _, q := range []string{
			`DELETE FROM notes WHERE id = ?`,
			`DELETE FROM aliases WHERE note_id = ?`,
			`DELETE FROM tags WHERE note_id = ?`,
			`DELETE FROM links WHERE src_id = ?`,
			`DELETE FROM notes_fts WHERE id = ?`,
		} {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveExpr resolves a normalised name (?1) to a note id. Order:
// id → path (+.md) → file name → title → alias.
const resolveExpr = `COALESCE(
	(SELECT id FROM notes WHERE lower(id) = ?1),
	(SELECT id FROM notes WHERE path_norm = ?1 OR path_norm = ?1 || '.md'),
	(SELECT id FROM notes WHERE stem_norm = ?1 ORDER BY length(path) LIMIT 1),
	(SELECT id FROM notes WHERE title_norm = ?1 ORDER BY length(path) LIMIT 1),
	(SELECT note_id FROM aliases WHERE alias_norm = ?1 LIMIT 1)
)`

// ResolveLinks recomputes dst_id for every link. Cheap enough to run after each sync.
func (s *Store) ResolveLinks(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE links SET dst_id = CASE kind
			WHEN 'md' THEN (SELECT id FROM notes WHERE path_norm = links.target_norm)
			ELSE `+strings.ReplaceAll(resolveExpr, "?1", "links.target_norm")+`
		END`)
	return err
}

// Resolve finds a note by id, path, file name, title or alias.
func (s *Store) Resolve(ctx context.Context, ref string) (*domain.NoteRef, error) {
	key := norm(strings.TrimSuffix(strings.ReplaceAll(strings.TrimSpace(ref), `\`, "/"), ".md"))
	var r domain.NoteRef
	err := s.db.QueryRowContext(ctx,
		`SELECT id, path, title FROM notes WHERE id = `+resolveExpr, key).Scan(&r.ID, &r.Path, &r.Title)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

// Search runs a full-text query.
func (s *Store) Search(ctx context.Context, q domain.SearchQuery) ([]domain.SearchHit, error) {
	match := q.Text
	if !q.Raw {
		match = ftsQuery(q.Text)
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}

	var where []string
	var args []any
	if match != "" {
		where = append(where, "notes_fts MATCH ?")
		args = append(args, match)
	}
	for _, t := range q.Tags {
		t = strings.ToLower(strings.TrimPrefix(t, "#"))
		where = append(where, "n.id IN (SELECT note_id FROM tags WHERE tag = ? OR tag LIKE ? || '/%')")
		args = append(args, t, t)
	}
	if len(where) == 0 {
		return nil, errors.New("empty query")
	}

	snippet, order := `''`, `n.path`
	if match != "" {
		// columns: 0 id, 1 title, 2 body, 3 tags. Weight title and tags above body.
		snippet = `snippet(notes_fts, 2, '«', '»', '…', 14)`
		order = `bm25(notes_fts, 0, 10.0, 1.0, 5.0)`
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT n.id, n.path, n.title, `+snippet+`, `+order+`
		FROM notes_fts JOIN notes n ON n.id = notes_fts.id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY 5 LIMIT ?`, append(args, q.Limit)...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	var out []domain.SearchHit
	for rows.Next() {
		var h domain.SearchHit
		var score any
		if err := rows.Scan(&h.ID, &h.Path, &h.Title, &h.Snippet, &score); err != nil {
			return nil, err
		}
		h.Snippet = strings.Join(strings.Fields(h.Snippet), " ")
		if f, ok := score.(float64); ok {
			h.Score = f
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ftsQuery turns `clean arch` into `"clean"* "arch"*` (AND, prefix match).
func ftsQuery(s string) string {
	var parts []string
	for _, f := range strings.Fields(s) {
		parts = append(parts, `"`+strings.ReplaceAll(f, `"`, `""`)+`"*`)
	}
	return strings.Join(parts, " ")
}

// Backlinks returns notes linking to id.
func (s *Store) Backlinks(ctx context.Context, id string) ([]domain.Backlink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT n.id, n.path, n.title, COALESCE(l.context, '')
		FROM links l JOIN notes n ON n.id = l.src_id
		WHERE l.dst_id = ? AND l.src_id <> l.dst_id
		ORDER BY n.path`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Backlink
	for rows.Next() {
		var b domain.Backlink
		if err := rows.Scan(&b.From.ID, &b.From.Path, &b.From.Title, &b.Context); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// OutLinks returns the outgoing links of id.
func (s *Store) OutLinks(ctx context.Context, id string) ([]domain.OutLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.target, l.kind, n.id, n.path, n.title
		FROM links l LEFT JOIN notes n ON n.id = l.dst_id
		WHERE l.src_id = ? ORDER BY l.rowid`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OutLink
	for rows.Next() {
		var o domain.OutLink
		var kind string
		var rid, rpath, rtitle sql.NullString
		if err := rows.Scan(&o.Target, &kind, &rid, &rpath, &rtitle); err != nil {
			return nil, err
		}
		o.Kind = domain.LinkKind(kind)
		if rid.Valid {
			o.Resolved = &domain.NoteRef{ID: rid.String, Path: rpath.String, Title: rtitle.String}
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Broken returns links that point to nothing.
func (s *Store) Broken(ctx context.Context) ([]domain.BrokenLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT n.id, n.path, n.title, l.target, l.kind
		FROM links l JOIN notes n ON n.id = l.src_id
		WHERE l.dst_id IS NULL ORDER BY n.path, l.rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.BrokenLink
	for rows.Next() {
		var b domain.BrokenLink
		var kind string
		if err := rows.Scan(&b.From.ID, &b.From.Path, &b.From.Title, &b.Target, &kind); err != nil {
			return nil, err
		}
		b.Kind = domain.LinkKind(kind)
		out = append(out, b)
	}
	return out, rows.Err()
}

// Orphans returns notes without incoming or outgoing (resolved) links.
func (s *Store) Orphans(ctx context.Context) ([]domain.NoteRef, error) {
	return s.refs(ctx, `
		SELECT id, path, title FROM notes n
		WHERE NOT EXISTS (SELECT 1 FROM links WHERE src_id = n.id AND dst_id IS NOT NULL AND dst_id <> n.id)
		  AND NOT EXISTS (SELECT 1 FROM links WHERE dst_id = n.id AND src_id <> n.id)
		ORDER BY path`)
}

// Tags returns all tags with counts.
func (s *Store) Tags(ctx context.Context) ([]domain.TagCount, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tag, COUNT(DISTINCT note_id) FROM tags GROUP BY tag ORDER BY 2 DESC, 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.TagCount
	for rows.Next() {
		var t domain.TagCount
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// NotesByTag returns notes carrying tag (or a child tag).
func (s *Store) NotesByTag(ctx context.Context, tag string) ([]domain.NoteRef, error) {
	tag = strings.ToLower(strings.TrimPrefix(tag, "#"))
	return s.refs(ctx, `
		SELECT DISTINCT n.id, n.path, n.title FROM notes n JOIN tags t ON t.note_id = n.id
		WHERE t.tag = ?1 OR t.tag LIKE ?1 || '/%' ORDER BY n.path`, tag)
}

// NotesInDir returns the notes directly inside dir, not those in its subfolders.
func (s *Store) NotesInDir(ctx context.Context, dir string) ([]domain.NoteRef, error) {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	return s.refs(ctx, `
		SELECT id, path, title FROM notes
		WHERE substr(path, 1, length(?1)) = ?1 AND instr(substr(path, length(?1) + 1), '/') = 0
		ORDER BY path`, prefix)
}

// Count returns the number of indexed notes.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notes`).Scan(&n)
	return n, err
}

func (s *Store) refs(ctx context.Context, q string, args ...any) ([]domain.NoteRef, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.NoteRef
	for rows.Next() {
		var r domain.NoteRef
		if err := rows.Scan(&r.ID, &r.Path, &r.Title); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
