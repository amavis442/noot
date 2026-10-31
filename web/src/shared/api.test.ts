import { describe, expect, it } from "vitest";
import {
  addReminder,
  ApiError,
  completeReminder,
  createNote,
  getJournal,
  getNote,
  listReminders,
  noteHref,
  planReminder,
  saveJournal,
  searchNotes,
} from "./api";
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

  it("loads and saves a journal day", async () => {
    const fetchMock = stubFetch(
      { body: { date: "2026-10-31", version: "v1", id: "J", path: "journal/2026-10-31.md", title: "", content: "x", html: "" } },
      { body: { date: "2026-10-31", version: "v2", id: "J", path: "journal/2026-10-31.md", title: "", content: "y", html: "" } },
    );
    expect((await getJournal("today")).version).toBe("v1");
    expect(fetchMock.mock.calls[0][0]).toBe("/api/journal/today");

    expect((await saveJournal("2026-10-31", "y", "v1")).version).toBe("v2");
    const [url, init] = fetchMock.mock.calls[1];
    expect(url).toBe("/api/journal/2026-10-31");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({ content: "y", version: "v1" });
  });

  it("lists, adds, completes and plans reminders", async () => {
    const fetchMock = stubFetch(
      { body: [] },
      { status: 201, body: { n: 1, text: "Melk", done: false } },
      { body: { n: 1, text: "Melk", done: true } },
      { body: { reminder: { n: 2, text: "X", done: false, planned: "2026-11-02" }, agenda: "agenda/2026-W45.md" } },
    );
    await listReminders(true);
    await addReminder("Melk", "morgen");
    await completeReminder(1);
    const planned = await planReminder(2, "maandag");

    expect(fetchMock.mock.calls.map((c) => [c[0], c[1]?.method ?? "GET", c[1]?.body])).toEqual([
      ["/api/reminders?all=true", "GET", undefined],
      ["/api/reminders", "POST", JSON.stringify({ text: "Melk", due: "morgen" })],
      ["/api/reminders/1/done", "POST", "{}"],
      ["/api/reminders/2/plan", "POST", JSON.stringify({ day: "maandag" })],
    ]);
    expect(planned.agenda).toBe("agenda/2026-W45.md");
  });
});
