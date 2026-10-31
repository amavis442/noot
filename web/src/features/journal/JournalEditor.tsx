import { useState, type FormEvent } from "react";

interface Props {
  initial: string;
  saving: boolean;
  onSave: (content: string) => void;
  onCancel: () => void;
}

/** A textarea for the whole note, frontmatter included. */
export function JournalEditor({ initial, saving, onSave, onCancel }: Props) {
  const [content, setContent] = useState(initial);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    onSave(content);
  }

  return (
    <form className="editor" onSubmit={onSubmit}>
      <label>
        Inhoud (Markdown)
        <textarea value={content} onChange={(e) => setContent(e.target.value)} rows={20} autoFocus />
      </label>
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
