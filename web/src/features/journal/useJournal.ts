import { useEffect, useState } from "react";
import { ApiError, errorMessage, getJournal, saveJournal, type JournalDay } from "../../shared/api";

export interface JournalState {
  journal: JournalDay | null;
  error: string | null;
  /** The file changed on disk after it was loaded; save refuses until reload. */
  conflict: boolean;
  saving: boolean;
  /** Saves the whole note; resolves to whether it worked. */
  save: (content: string) => Promise<boolean>;
  /** Loads the note again, dropping a conflict. */
  reload: () => void;
}

/** Loads the journal note of day and saves edits to it. */
export function useJournal(day: string): JournalState {
  const [journal, setJournal] = useState<JournalDay | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loads, setLoads] = useState(0);

  useEffect(() => {
    const ctrl = new AbortController();
    setJournal(null);
    setError(null);
    setConflict(false);
    getJournal(day, ctrl.signal)
      .then(setJournal)
      .catch((err: unknown) => {
        if (!ctrl.signal.aborted) {
          setError(errorMessage(err));
        }
      });
    return () => ctrl.abort();
  }, [day, loads]);

  async function save(content: string): Promise<boolean> {
    if (!journal) {
      return false;
    }
    setSaving(true);
    setError(null);
    try {
      setJournal(await saveJournal(journal.date, content, journal.version));
      return true;
    } catch (err) {
      setConflict(err instanceof ApiError && err.status === 409);
      setError(errorMessage(err));
      return false;
    } finally {
      setSaving(false);
    }
  }

  return { journal, error, conflict, saving, save, reload: () => setLoads((n) => n + 1) };
}
