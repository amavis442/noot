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
  return sendJSON("POST", "/api/notes", draft);
}

/** The journal note of one day. */
export interface JournalDay extends Note {
  date: string; // YYYY-MM-DD
  /** Identifies the loaded content; saveJournal sends it back. */
  version: string;
}

/** A line of reminders.md; n is its number, as the CLI shows it. */
export interface Reminder {
  n: number;
  text: string;
  done: boolean;
  due?: string;
  planned?: string;
  done_on?: string;
}

export interface Planned {
  reminder: Reminder;
  /** The agenda note the reminder was planned in. */
  agenda: string;
}

function sendJSON<T>(method: string, url: string, body: unknown): Promise<T> {
  return request(url, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** day is YYYY-MM-DD or a word the CLI knows: today, vandaag, morgen, vrijdag, +2 … */
export function getJournal(day: string, signal?: AbortSignal): Promise<JournalDay> {
  return request(`/api/journal/${encodeURIComponent(day)}`, { signal });
}

/** Fails with status 409 when the note changed after it was loaded at version. */
export function saveJournal(day: string, content: string, version: string): Promise<JournalDay> {
  return sendJSON("PUT", `/api/journal/${encodeURIComponent(day)}`, { content, version });
}

export function listReminders(all: boolean, signal?: AbortSignal): Promise<Reminder[]> {
  return request(`/api/reminders?all=${all}`, { signal });
}

export function addReminder(text: string, due?: string): Promise<Reminder> {
  return sendJSON("POST", "/api/reminders", due ? { text, due } : { text });
}

export function completeReminder(n: number): Promise<Reminder> {
  return sendJSON("POST", `/api/reminders/${n}/done`, {});
}

export function planReminder(n: number, day: string): Promise<Planned> {
  return sendJSON("POST", `/api/reminders/${n}/plan`, { day });
}

/** The frontend route that shows a note; ref is an id, path, title or alias. */
export function noteHref(ref: string): string {
  return `#/note/${encodeURIComponent(ref)}`;
}

/** The message to show for a failed call. */
export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
