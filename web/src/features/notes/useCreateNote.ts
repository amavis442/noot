import { useState } from "react";
import { createNote, errorMessage, noteHref, type NoteDraft } from "../../shared/api";

export interface CreateNote {
  create: (draft: NoteDraft) => Promise<void>;
  saving: boolean;
  error: string | null;
}

/** Creates notes; a created note is opened right away. */
export function useCreateNote(): CreateNote {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function create(draft: NoteDraft) {
    setSaving(true);
    setError(null);
    try {
      const ref = await createNote(draft);
      window.location.hash = noteHref(ref.path);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return { create, saving, error };
}

/** Splits "a, b c" into ["a", "b", "c"]. */
export function parseTags(input: string): string[] {
  return input.split(/[\s,]+/).filter((t) => t !== "");
}
