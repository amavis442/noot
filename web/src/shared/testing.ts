import { vi } from "vitest";

/** A canned answer for one fetch call. */
export interface Reply {
  status?: number;
  body: unknown;
}

/**
 * Replaces fetch with a mock that answers the calls in order and returns it,
 * so a test can check what was requested.
 */
export function stubFetch(...replies: Reply[]) {
  const fetchMock = vi.fn();
  for (const r of replies) {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify(r.body), {
        status: r.status ?? 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
  }
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}
