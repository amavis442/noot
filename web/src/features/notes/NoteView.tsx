import { getNote } from "../../shared/api";
import { EditableNote } from "../../shared/editing/EditableNote";
import { useEditableNote } from "../../shared/editing/useEditableNote";

export function NoteView({ noteRef }: { noteRef: string }) {
  const state = useEditableNote(noteRef, getNote);
  return <EditableNote state={state} header={(note) => <p className="path">{note.path}</p>} />;
}
