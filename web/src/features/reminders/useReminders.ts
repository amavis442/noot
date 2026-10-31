import { useEffect, useState } from "react";
import {
  addReminder,
  completeReminder,
  errorMessage,
  listReminders,
  planReminder,
  type Reminder,
} from "../../shared/api";

export interface RemindersState {
  reminders: Reminder[] | null; // null while loading
  error: string | null;
  showDone: boolean;
  setShowDone: (show: boolean) => void;
  /** Each action reloads the list and resolves to whether it worked. */
  add: (text: string, due: string) => Promise<boolean>;
  complete: (n: number) => Promise<boolean>;
  plan: (n: number, day: string) => Promise<boolean>;
}

/** The reminders of the vault and what can be done with them. */
export function useReminders(): RemindersState {
  const [reminders, setReminders] = useState<Reminder[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showDone, setShowDone] = useState(false);
  const [loads, setLoads] = useState(0);

  useEffect(() => {
    const ctrl = new AbortController();
    listReminders(showDone, ctrl.signal)
      .then(setReminders)
      .catch((err: unknown) => {
        if (!ctrl.signal.aborted) {
          setError(errorMessage(err));
        }
      });
    return () => ctrl.abort();
  }, [showDone, loads]);

  async function act(action: () => Promise<unknown>): Promise<boolean> {
    setError(null);
    try {
      await action();
      setLoads((n) => n + 1);
      return true;
    } catch (err) {
      setError(errorMessage(err));
      return false;
    }
  }

  return {
    reminders,
    error,
    showDone,
    setShowDone,
    add: (text, due) => act(() => addReminder(text, due.trim() || undefined)),
    complete: (n) => act(() => completeReminder(n)),
    plan: (n, day) => act(() => planReminder(n, day)),
  };
}
