// Package domain holds the core types of noot. It has no dependencies on
// storage, parsing or the CLI, so those layers can be swapped independently.
package domain

// LinkKind tells how a link was written in the source file.
type LinkKind string

const (
	// LinkWiki is a [[wikilink]] (optionally [[target|label]] or [[target#heading]]).
	LinkWiki LinkKind = "wiki"
	// LinkMarkdown is a regular [label](relative/path.md) link.
	LinkMarkdown LinkKind = "md"
)

// Note is a single Markdown file in the vault, as understood by noot.
type Note struct {
	ID      string   // frontmatter `id`, or the vault-relative path without .md
	Path    string   // vault-relative path with forward slashes, e.g. "zettels/foo.md"
	Title   string   // frontmatter `title`, first H1, or file name
	Created string   // frontmatter `created`, free-form (usually YYYY-MM-DD)
	Tags    []string // lower-cased, without '#'; from frontmatter and #inline tags
	Aliases []string // frontmatter `aliases`
	Links   []Link   // outgoing links in document order
	Body    string   // Markdown body without frontmatter (used for full-text search)
	ModTime int64    // file mtime, unix nanoseconds
	Size    int64    // file size in bytes
}

// Link is an outgoing reference from one note to another.
type Link struct {
	Target  string   // normalised target: wiki → name/id, md → vault-relative path
	Kind    LinkKind //
	Context string   // the paragraph the link appears in (trimmed), for backlink previews
}

// NoteRef is a lightweight pointer to a note, used in query results.
type NoteRef struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Title string `json:"title"`
}

// SearchQuery describes a full-text search.
type SearchQuery struct {
	Text  string   // user input; converted to an FTS5 prefix query unless Raw is set
	Raw   bool     // pass Text to FTS5 MATCH unmodified
	Tags  []string // every tag must match (hierarchical: "go" also matches "go/generics")
	Limit int
}

// SearchHit is one result of a full-text search.
type SearchHit struct {
	NoteRef
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"` // bm25, lower is better
}

// Backlink is a note that links to the note being queried.
type Backlink struct {
	From    NoteRef `json:"from"`
	Context string  `json:"context"`
}

// OutLink is an outgoing link, with the note it resolves to (nil when broken).
type OutLink struct {
	Target   string   `json:"target"`
	Kind     LinkKind `json:"kind"`
	Resolved *NoteRef `json:"resolved,omitempty"`
}

// BrokenLink is a link whose target does not resolve to any note.
type BrokenLink struct {
	From   NoteRef  `json:"from"`
	Target string   `json:"target"`
	Kind   LinkKind `json:"kind"`
}

// TagCount is a tag with the number of notes carrying it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// FileState is what the indexer remembers about a file to detect changes.
type FileState struct {
	ModTime int64
	Size    int64
}

// LogExcerpt is one entry of a daily log.
type LogExcerpt struct {
	Path    string `json:"path"`    // the log file, e.g. "logs/2026-10-25.md"
	Heading string `json:"heading"` // e.g. "00:47 · [[noot]]"
	Text    string `json:"text"`
}

// Document is a note with its full file content, frontmatter included.
type Document struct {
	NoteRef
	Content string `json:"content"`
}

// NoteContext is what is worth reading before working on a note: the note
// itself, the notes around it and the recent log entries that mention it.
type NoteContext struct {
	Note    NoteRef      `json:"note"`
	Content string       `json:"content"`
	Related []Backlink   `json:"related"` // notes linking to it, then the other notes in its folder
	Logs    []LogExcerpt `json:"logs"`    // oldest first
}
