import { getAgenda, type AgendaWeek } from "../../shared/api";
import { agendaHref, shiftDay } from "../../shared/days";
import { EditableNote } from "../../shared/editing/EditableNote";
import { useEditableNote } from "../../shared/editing/useEditableNote";
import { AgendaDayCard } from "./AgendaDayCard";

/**
 * The agenda of the week day falls in (YYYY-MM-DD or a word such as today):
 * every day to read or edit on its own, or the whole week at once.
 */
export function AgendaView({ day }: { day: string }) {
  const state = useEditableNote(day, getAgenda);
  const dayNames = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
  return (
    <EditableNote
      state={state}
      header={(w) => <WeekNav week={w} />}
      className="agenda"
      editLabel="Hele week bewerken"
      view={(w) => (
        <>
          {/* The server renders the Markdown without raw HTML, so this is safe. */}
          <div className="markdown" dangerouslySetInnerHTML={{ __html: w.intro_html }} />
          {w.days.map((d) => {
            const inputDate = new Date(d.date)
            const dayIndex = inputDate.getDay()
            const currentDay = dayNames[dayIndex]
            return <AgendaDayCard key={d.date} day={d} dayname={currentDay} path={w.path} refresh={state.refresh} />
          })}        
        </>
      )}
    />
  );
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
