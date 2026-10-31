import { useState } from "react";
import type { AgendaDay } from "../../shared/api";
import { NoteEditor } from "../../shared/editing/NoteEditor";
import { useDaySave } from "./useDaySave";

interface Props {
  day: AgendaDay;
  /** The agenda note, so links in the preview resolve as they will once saved. */
  path: string;
  /** Loads the week again, after a save or to drop a conflict. */
  refresh: () => Promise<void>;
}

/** One day of the agenda, to read or to edit on its own. */
export function AgendaDayCard({ day, path, refresh }: Props) {
  const [editing, setEditing] = useState(false);
  const { save, saving, error, conflict } = useDaySave(day, refresh);

  async function onSave(content: string) {
    if (await save(content)) {
      setEditing(false);
    }
  }

  async function onReload() {
    await refresh();
    setEditing(false);
  }

  return (
    <section className="agenda-day" aria-labelledby={`day-${day.date}`}>
      <div className="agenda-day-head">
        <h2 id={`day-${day.date}`}>{day.heading}</h2>
        {!editing && (
          <button type="button" className="secondary" onClick={() => setEditing(true)}>
            Bewerken
          </button>
        )}
      </div>
      {error && (
        <div role="alert" className="error">
          <p>{error}</p>
          {conflict && (
            <button type="button" onClick={() => void onReload()}>
              Herladen (jouw wijzigingen gaan verloren)
            </button>
          )}
        </div>
      )}
      {editing ? (
        <NoteEditor
          path={path}
          initial={day.content}
          initialHtml={day.html}
          saving={saving}
          onSave={(content) => void onSave(content)}
          onCancel={() => setEditing(false)}
        />
      ) : (
        /* The server renders the Markdown without raw HTML, so this is safe. */
        <div className="markdown" dangerouslySetInnerHTML={{ __html: day.html }} />
      )}
    </section>
  );
}
