import { useState } from "react";
import { ApiError, errorMessage, saveAgendaDay, type AgendaDay } from "../../shared/api";

export interface DaySave {
  /** Saves the day's text; resolves to whether it worked. */
  save: (content: string) => Promise<boolean>;
  saving: boolean;
  error: string | null;
  /** The day changed elsewhere after it was loaded; save refuses until the week reloads. */
  conflict: boolean;
}

/** Saves one day of the agenda; onSaved runs after a save that worked. */
export function useDaySave(day: AgendaDay, onSaved: () => Promise<void>): DaySave {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);

  async function save(content: string): Promise<boolean> {
    setSaving(true);
    setError(null);
    try {
      await saveAgendaDay(day.date, content, day.version);
      await onSaved();
      return true;
    } catch (err) {
      setConflict(err instanceof ApiError && err.status === 409);
      setError(errorMessage(err));
      return false;
    } finally {
      setSaving(false);
    }
  }

  return { save, saving, error, conflict };
}
