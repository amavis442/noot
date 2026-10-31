import { vi } from "vitest";

/** A canned answer for one fetch call. */
export interface Reply {
  status?: number;
  body: unknown;
}

function respond(r: Reply): Response {
  return new Response(JSON.stringify(r.body), {
    status: r.status ?? 200,
    headers: { "Content-Type": "application/json" },
  });
}

const previewPath = "/api/preview";

type FetchFn = (input: string, init?: RequestInit) => Promise<Response>;

// The editor asks for previews on its own while text is typed. Those calls go
// to previewMock, so tests that count or order the other calls are not
// affected by them; by default every preview is empty.
let previewMock = emptyPreview();

function emptyPreview() {
  return vi.fn<FetchFn>(async () => respond({ body: { html: "" } }));
}

/**
 * Replaces fetch with a mock that answers the calls in order and returns it,
 * so a test can check what was requested. Preview calls are answered apart;
 * see stubPreview.
 */
export function stubFetch(...replies: Reply[]) {
  const fetchMock = vi.fn();
  for (const r of replies) {
    fetchMock.mockResolvedValueOnce(respond(r));
  }
  vi.stubGlobal("fetch", (input: string, init?: RequestInit) =>
    String(input).startsWith(previewPath) ? previewMock(input, init) : fetchMock(input, init),
  );
  return fetchMock;
}

/** Answers the preview calls in order and returns their mock. */
export function stubPreview(...replies: Reply[]) {
  previewMock = vi.fn<FetchFn>();
  for (const r of replies) {
    previewMock.mockResolvedValueOnce(respond(r));
  }
  return previewMock;
}

/** Puts back the empty previews; setupTests calls it after every test. */
export function resetPreview() {
  previewMock = emptyPreview();
}
