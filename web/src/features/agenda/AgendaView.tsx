import { getAgenda, type AgendaWeek } from "../../shared/api";
import { agendaHref, shiftDay } from "../../shared/days";
import { EditableNote } from "../../shared/editing/EditableNote";
import { useEditableNote } from "../../shared/editing/useEditableNote";

/** The agenda of the week day falls in (YYYY-MM-DD or a word such as today), to read or edit. */
export function AgendaView({ day }: { day: string }) {
  const state = useEditableNote(day, getAgenda);
  return <EditableNote state={state} header={(w) => <WeekNav week={w} />} className="agenda" />;
}

function WeekNav({ week }: { week: AgendaWeek }) {
  return (
    <nav className="day-nav">
      <a href={agendaHref(shiftDay(week.monday, -7))}>← Vorige week</a>
      <span className="path">{week.path}</span>
      <a href={agendaHref(shiftDay(week.monday, 7))}>Volgende week →</a>
    </nav>
  );
}
