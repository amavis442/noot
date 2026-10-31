import { useEffect, useState } from "react";
import { errorMessage, previewNote } from "../api";

/** Wait this long after the last keystroke before asking for a preview. */
const debounceMs = 250;

export interface Preview {
  html: string;
  /** Why the latest preview failed; html then still shows the one before. */
  error: string | null;
}

/**
 * Renders content as the note at path, following it as it changes. It starts
 * with initialHtml, the rendering of initialContent, so opening the editor
 * costs no request.
 */
export function usePreview(path: string, content: string, initialContent: string, initialHtml: string): Preview {
  const [preview, setPreview] = useState<Preview>({ html: initialHtml, error: null });

  useEffect(() => {
    if (content === initialContent) {
      setPreview({ html: initialHtml, error: null });
      return;
    }
    const ctrl = new AbortController();
    const timer = setTimeout(() => {
      previewNote(path, content, ctrl.signal)
        .then((html) => setPreview({ html, error: null }))
        .catch((err: unknown) => {
          if (!ctrl.signal.aborted) {
            setPreview((p) => ({ ...p, error: errorMessage(err) }));
          }
        });
    }, debounceMs);
    return () => {
      clearTimeout(timer);
      ctrl.abort();
    };
  }, [path, content, initialContent, initialHtml]);

  return preview;
}
