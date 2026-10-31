import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { NoteView } from "./NoteView";

describe("NoteView", () => {
  it("shows the rendered note with its path", async () => {
    const fetchMock = stubFetch({
      body: {
        id: "A",
        path: "zettels/a.md",
        title: "Alpha",
        content: "# Alpha",
        html: '<h1>Alpha</h1><p>Zie <a href="#/note/Beta">Beta</a></p>',
      },
    });
    render(<NoteView noteRef="zettels/a.md" />);

    expect(await screen.findByRole("heading", { name: "Alpha" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Beta" })).toHaveAttribute("href", "#/note/Beta");
    expect(screen.getByText("zettels/a.md")).toBeInTheDocument();
    expect(fetchMock.mock.calls[0][0]).toBe("/api/notes/zettels%2Fa.md");
  });

  it("shows when a note does not exist", async () => {
    stubFetch({ status: 404, body: { error: 'no note matches "Beta"' } });
    render(<NoteView noteRef="Beta" />);
    expect(await screen.findByRole("alert")).toHaveTextContent('no note matches "Beta"');
  });
});
