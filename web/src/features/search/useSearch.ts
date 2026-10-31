import { useEffect, useState } from "react";
import { errorMessage, searchNotes, type SearchHit } from "../../shared/api";

/** Wait this long after the last keystroke before searching. */
const debounceMs = 200;

export interface SearchState {
  hits: SearchHit[] | null; // null until a search has finished
  error: string | null;
  loading: boolean;
}

const idle: SearchState = { hits: null, error: null, loading: false };

/** Searches for query as it changes; an older search is cancelled. */
export function useSearch(query: string): SearchState {
  const [state, setState] = useState<SearchState>(idle);
  const text = query.trim();

  useEffect(() => {
    if (text === "") {
      setState(idle);
      return;
    }
    const ctrl = new AbortController();
    setState((s) => ({ ...s, loading: true }));
    const timer = setTimeout(() => {
      searchNotes(text, ctrl.signal)
        .then((hits) => setState({ hits, error: null, loading: false }))
        .catch((err: unknown) => {
          if (!ctrl.signal.aborted) {
            setState({ hits: null, error: errorMessage(err), loading: false });
          }
        });
    }, debounceMs);
    return () => {
      clearTimeout(timer);
      ctrl.abort();
    };
  }, [text]);

  return state;
}
