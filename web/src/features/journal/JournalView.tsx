import { useState } from "react";
import { journalHref, shiftDay } from "./days";
import { JournalEditor } from "./JournalEditor";
import { useJournal } from "./useJournal";

/** The journal note of day (YYYY-MM-DD or a word such as today), to read or edit. */
export function JournalView({ day }: { day: string }) {
  const { journal, error, conflict, saving, save, reload } = useJournal(day);
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

  if (!journal) {
    return error ? <p role="alert" className="error">{error}</p> : <p className="muted">Laden…</p>;
  }
  return (
    <article className="note journal">
      <nav className="day-nav">
        <a href={journalHref(shiftDay(journal.date, -1))}>← Vorige dag</a>
        <span className="path">{journal.path}</span>
        <a href={journalHref(shiftDay(journal.date, 1))}>Volgende dag →</a>
      </nav>
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
        <JournalEditor initial={journal.content} saving={saving} onSave={onSave} onCancel={() => setEditing(false)} />
      ) : (
        <>
          <button type="button" onClick={() => setEditing(true)}>
            Bewerken
          </button>
          {/* The server renders the Markdown without raw HTML, so this is safe. */}
          <div className="markdown" dangerouslySetInnerHTML={{ __html: journal.html }} />
        </>
      )}
    </article>
  );
}
