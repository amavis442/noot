import { useEffect, useState } from "react";
import { ApiError, errorMessage, saveNote, type Note } from "../api";

/** Loads a note by key, such as a ref or a day, for useEditableNote. */
export type NoteLoader<T extends Note> = (key: string, signal: AbortSignal) => Promise<T>;

export interface EditableNoteState<T extends Note> {
  note: T | null;
  error: string | null;
  /** The file changed on disk after it was loaded; save refuses until reload. */
  conflict: boolean;
  saving: boolean;
  /** Saves the whole note; resolves to whether it worked. */
  save: (content: string) => Promise<boolean>;
  /** Loads the note again, dropping a conflict. */
  reload: () => void;
  /** Loads the note again while the current one stays on screen. */
  refresh: () => Promise<void>;
}

/**
 * Loads the note load(key) returns and saves edits to it with the version it
 * was loaded with. load must be the same function on every render.
 */
export function useEditableNote<T extends Note>(key: string, load: NoteLoader<T>): EditableNoteState<T> {
  const [note, setNote] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loads, setLoads] = useState(0);

  useEffect(() => {
    const ctrl = new AbortController();
    setNote(null);
    setError(null);
    setConflict(false);
    load(key, ctrl.signal)
      .then(setNote)
      .catch((err: unknown) => {
        if (!ctrl.signal.aborted) {
          setError(errorMessage(err));
        }
      });
    return () => ctrl.abort();
  }, [key, load, loads]);

  async function save(content: string): Promise<boolean> {
    if (!note) {
      return false;
    }
    setSaving(true);
    setError(null);
    try {
      const saved = await saveNote(note.path, content, note.version);
      // Keep what only the loader knows, such as the day of a journal note.
      setNote({ ...note, ...saved });
      return true;
    } catch (err) {
      setConflict(err instanceof ApiError && err.status === 409);
      setError(errorMessage(err));
      return false;
    } finally {
      setSaving(false);
    }
  }

  async function refresh() {
    try {
      setNote(await load(key, new AbortController().signal));
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  return { note, error, conflict, saving, save, reload: () => setLoads((n) => n + 1), refresh };
}
