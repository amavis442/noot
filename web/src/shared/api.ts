// Client for the JSON API of `noot serve`.

export interface NoteRef {
  id: string;
  path: string;
  title: string;
}

export interface SearchHit extends NoteRef {
  /** Matching text; the matched terms are wrapped in « and ». */
  snippet: string;
  score: number;
}

export interface Note extends NoteRef {
  /** The file content, frontmatter included. */
  content: string;
  /** The body rendered by the server; links to notes point at noteHref. */
  html: string;
}

export interface NoteDraft {
  title: string;
  body?: string;
  tags?: string[];
}

/** An error response; message is the server's explanation. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  const data: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const message = (data as { error?: string } | null)?.error ?? res.statusText;
    throw new ApiError(message, res.status);
  }
  return data as T;
}

export function searchNotes(query: string, signal?: AbortSignal): Promise<SearchHit[]> {
  return request(`/api/search?q=${encodeURIComponent(query)}`, { signal });
}

export function getNote(ref: string, signal?: AbortSignal): Promise<Note> {
  return request(`/api/notes/${encodeURIComponent(ref)}`, { signal });
}

export function createNote(draft: NoteDraft): Promise<NoteRef> {
  return request("/api/notes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(draft),
  });
}

/** The frontend route that shows a note; ref is an id, path, title or alias. */
export function noteHref(ref: string): string {
  return `#/note/${encodeURIComponent(ref)}`;
}

/** The message to show for a failed call. */
export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
