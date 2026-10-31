import { AgendaView } from "./features/agenda/AgendaView";
import { JournalView } from "./features/journal/JournalView";
import { NewNoteForm } from "./features/notes/NewNoteForm";
import { NoteView } from "./features/notes/NoteView";
import { RemindersPage } from "./features/reminders/RemindersPage";
import { SearchPanel } from "./features/search/SearchPanel";
import { useHashRoute } from "./shared/useHashRoute";

export function App() {
  const route = useHashRoute();

  return (
    <div className="layout">
      <header>
        <a href="#/" className="brand">
          noot
        </a>
        <nav className="main-nav">
          <a href="#/journal">Vandaag</a>
          <a href="#/agenda">Agenda</a>
          <a href="#/reminders">Reminders</a>
          <a href="#/new" className="button">
            Nieuwe notitie
          </a>
        </nav>
      </header>
      <aside>
        <SearchPanel />
      </aside>
      <main>
        {route.name === "note" && <NoteView key={route.ref} noteRef={route.ref} />}
        {route.name === "new" && <NewNoteForm />}
        {route.name === "journal" && <JournalView key={route.day} day={route.day} />}
        {route.name === "agenda" && <AgendaView key={route.day} day={route.day} />}
        {route.name === "reminders" && <RemindersPage />}
        {route.name === "home" && <p className="muted">Zoek een notitie of maak een nieuwe.</p>}
      </main>
    </div>
  );
}
