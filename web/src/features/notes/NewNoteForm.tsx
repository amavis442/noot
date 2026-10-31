import { useState, type FormEvent } from "react";
import { parseTags, useCreateNote } from "./useCreateNote";

export function NewNoteForm() {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [tags, setTags] = useState("");
  const { create, saving, error } = useCreateNote();
  const canSave = title.trim() !== "" && !saving;

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!canSave) {
      return;
    }
    const tagList = parseTags(tags);
    void create({
      title: title.trim(),
      ...(body.trim() !== "" && { body }),
      ...(tagList.length > 0 && { tags: tagList }),
    });
  }

  return (
    <form className="new-note" onSubmit={onSubmit}>
      <h2>Nieuwe notitie</h2>
      <label>
        Titel
        <input value={title} onChange={(e) => setTitle(e.target.value)} required autoFocus />
      </label>
      <label>
        Tekst (Markdown, optioneel)
        <textarea value={body} onChange={(e) => setBody(e.target.value)} rows={12} />
      </label>
      <label>
        Tags (optioneel, gescheiden door spatie of komma)
        <input value={tags} onChange={(e) => setTags(e.target.value)} />
      </label>
      {error && <p role="alert" className="error">{error}</p>}
      <button type="submit" disabled={!canSave}>
        {saving ? "Bezig…" : "Aanmaken"}
      </button>
    </form>
  );
}
