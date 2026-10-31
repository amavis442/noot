import { NewNoteForm } from "./features/notes/NewNoteForm";
import { NoteView } from "./features/notes/NoteView";
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
        <a href="#/new" className="button">
          Nieuwe notitie
        </a>
      </header>
      <aside>
        <SearchPanel />
      </aside>
      <main>
        {route.name === "note" && <NoteView key={route.ref} noteRef={route.ref} />}
        {route.name === "new" && <NewNoteForm />}
        {route.name === "home" && <p className="muted">Zoek een notitie of maak een nieuwe.</p>}
      </main>
    </div>
  );
}
