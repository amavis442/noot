import { useState, type ReactNode } from "react";
import type { Note } from "../api";
import { NoteEditor } from "./NoteEditor";
import type { EditableNoteState } from "./useEditableNote";

interface Props<T extends Note> {
  state: EditableNoteState<T>;
  /** Shown above the note, such as its path or a day navigation. */
  header: (note: T) => ReactNode;
  /** Extra class for the note, so a kind of note can have its own style. */
  className?: string;
  /** The text of the button that edits the whole note. */
  editLabel?: string;
  /** Shows the note while it is not edited as a whole; the rendered HTML by default. */
  view?: (note: T) => ReactNode;
}

/** A note to read, with a button to edit the whole file. */
export function EditableNote<T extends Note>({ state, header, className, editLabel = "Bewerken", view }: Props<T>) {
  const { note, error, conflict, saving, save, reload } = state;
  const [editing, setEditing] = useState(false);

  async function onSave(content: string) {
    if (await save(content)) {
      setEditing(false);
    }
  }

  function onReload() {
    setEditing(false);
    reload();
  }

  if (!note) {
    return error ? <p role="alert" className="error">{error}</p> : <p className="muted">Laden…</p>;
  }
  return (
    <article className={["note", className, editing && "editing"].filter(Boolean).join(" ")}>
      {header(note)}
      {error && (
        <div role="alert" className="error">
          <p>{error}</p>
          {conflict && (
            <button type="button" onClick={onReload}>
              Herladen (jouw wijzigingen gaan verloren)
            </button>
          )}
        </div>
      )}
      {editing ? (
        <NoteEditor
          path={note.path}
          initial={note.content}
          initialHtml={note.html}
          saving={saving}
          onSave={onSave}
          onCancel={() => setEditing(false)}
        />
      ) : (
        <>
          <button type="button" onClick={() => setEditing(true)}>
            {editLabel}
          </button>
          {view ? (
            view(note)
          ) : (
            /* The server renders the Markdown without raw HTML, so this is safe. */
            <div className="markdown" dangerouslySetInnerHTML={{ __html: note.html }} />
          )}
        </>
      )}
    </article>
  );
}
