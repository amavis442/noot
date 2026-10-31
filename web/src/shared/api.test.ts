import { describe, expect, it } from "vitest";
import { ApiError, createNote, getNote, noteHref, searchNotes } from "./api";
import { stubFetch } from "./testing";

describe("api", () => {
  it("searches with an encoded query", async () => {
    const fetchMock = stubFetch({ body: [{ id: "A", path: "a.md", title: "Alpha", snippet: "«alp»ha", score: -1 }] });
    const hits = await searchNotes("café & co");
    expect(fetchMock.mock.calls[0][0]).toBe("/api/search?q=caf%C3%A9%20%26%20co");
    expect(hits[0].title).toBe("Alpha");
  });

  it("reads a note by an encoded ref", async () => {
    const fetchMock = stubFetch({
      body: { id: "A", path: "zettels/a.md", title: "Alpha", content: "# Alpha", html: "<h1>Alpha</h1>" },
    });
    const note = await getNote("zettels/a.md");
    expect(fetchMock.mock.calls[0][0]).toBe("/api/notes/zettels%2Fa.md");
    expect(note.html).toBe("<h1>Alpha</h1>");
  });

  it("creates a note as JSON", async () => {
    const fetchMock = stubFetch({ status: 201, body: { id: "N", path: "zettels/n.md", title: "N" } });
    const ref = await createNote({ title: "N", body: "tekst" });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/notes");
    expect(init.method).toBe("POST");
    expect(init.headers).toEqual({ "Content-Type": "application/json" });
    expect(JSON.parse(init.body)).toEqual({ title: "N", body: "tekst" });
    expect(ref.path).toBe("zettels/n.md");
  });

  it("turns an error response into an ApiError with the server's message", async () => {
    stubFetch({ status: 400, body: { error: "invalid note: title is required" } });
    const err = await createNote({ title: "" }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(400);
    expect((err as ApiError).message).toBe("invalid note: title is required");
  });

  it("links to a note through the hash route", () => {
    expect(noteHref("zettels/a b.md")).toBe("#/note/zettels%2Fa%20b.md");
  });
});
