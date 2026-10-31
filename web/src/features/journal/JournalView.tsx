import { getJournal, type JournalDay } from "../../shared/api";
import { journalHref, shiftDay } from "../../shared/days";
import { EditableNote } from "../../shared/editing/EditableNote";
import { useEditableNote } from "../../shared/editing/useEditableNote";

/** The journal note of day (YYYY-MM-DD or a word such as today), to read or edit. */
export function JournalView({ day }: { day: string }) {
  const state = useEditableNote(day, getJournal);
  return <EditableNote state={state} header={(j) => <DayNav journal={j} />} />;
}

function DayNav({ journal }: { journal: JournalDay }) {
  return (
    <nav className="day-nav">
      <a href={journalHref(shiftDay(journal.date, -1))}>← Vorige dag</a>
      <span className="path">{journal.path}</span>
      <a href={journalHref(shiftDay(journal.date, 1))}>Volgende dag →</a>
    </nav>
  );
}
