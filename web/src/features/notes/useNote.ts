import { useEffect, useState } from "react";
import { errorMessage, getNote, type Note } from "../../shared/api";

export interface NoteState {
  note: Note | null;
  error: string | null;
}

/** Loads the note ref points to, again whenever ref changes. */
export function useNote(ref: string): NoteState {
  const [state, setState] = useState<NoteState>({ note: null, error: null });

  useEffect(() => {
    const ctrl = new AbortController();
    setState({ note: null, error: null });
    getNote(ref, ctrl.signal)
      .then((note) => setState({ note, error: null }))
      .catch((err: unknown) => {
        if (!ctrl.signal.aborted) {
          setState({ note: null, error: errorMessage(err) });
        }
      });
    return () => ctrl.abort();
  }, [ref]);

  return state;
}
