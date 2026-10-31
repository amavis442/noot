import { useState, type FormEvent } from "react";
import { usePreview } from "./usePreview";

interface Props {
  /** Where the note lives, so links in the preview resolve as they will once saved. */
  path: string;
  initial: string;
  /** The rendering of initial, shown until the text changes. */
  initialHtml: string;
  saving: boolean;
  onSave: (content: string) => void;
  onCancel: () => void;
}

/** A textarea for the whole note, frontmatter included, next to a live preview. */
export function NoteEditor({ path, initial, initialHtml, saving, onSave, onCancel }: Props) {
  const [content, setContent] = useState(initial);
  const preview = usePreview(path, content, initial, initialHtml);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    onSave(content);
  }

  return (
    <form className="editor" onSubmit={onSubmit}>
      <div className="editor-panes">
        <label>
          Inhoud (Markdown)
          <textarea value={content} onChange={(e) => setContent(e.target.value)} rows={20} autoFocus />
        </label>
        <section className="preview" aria-label="Voorbeeld">
          <span className="muted">Voorbeeld</span>
          {preview.error && <p className="error">{preview.error}</p>}
          {/* The server renders the Markdown without raw HTML, so this is safe. */}
          <div className="markdown" dangerouslySetInnerHTML={{ __html: preview.html }} />
        </section>
      </div>
      <div className="actions">
        <button type="submit" disabled={saving}>
          {saving ? "Bezig…" : "Opslaan"}
        </button>
        <button type="button" className="secondary" onClick={onCancel}>
          Annuleren
        </button>
      </div>
    </form>
  );
}
