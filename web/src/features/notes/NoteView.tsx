import { useNote } from "./useNote";

export function NoteView({ noteRef }: { noteRef: string }) {
  const { note, error } = useNote(noteRef);

  if (error) {
    return <p role="alert" className="error">{error}</p>;
  }
  if (!note) {
    return <p className="muted">Laden…</p>;
  }
  return (
    <article className="note">
      <p className="path">{note.path}</p>
      {/* The server renders the Markdown without raw HTML, so this is safe. */}
      <div className="markdown" dangerouslySetInnerHTML={{ __html: note.html }} />
    </article>
  );
}
