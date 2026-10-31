import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { NoteView } from "./NoteView";

function alpha(version: string, content: string, html: string) {
  return { body: { id: "A", path: "zettels/a.md", title: "Alpha", version, content, html } };
}

describe("NoteView", () => {
  it("shows the rendered note with its path", async () => {
    const fetchMock = stubFetch(alpha("v1", "# Alpha", '<h1>Alpha</h1><p>Zie <a href="#/note/Beta">Beta</a></p>'));
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

  it("edits the whole note and saves it with its version", async () => {
    const fetchMock = stubFetch(alpha("v1", "# Alpha\n", "<h1>Alpha</h1>"), alpha("v2", "# Alpha\nMeer", "<p>Meer</p>"));
    render(<NoteView noteRef="Alpha" />);

    await userEvent.click(await screen.findByRole("button", { name: /bewerken/i }));
    const editor = screen.getByLabelText(/inhoud/i);
    expect(editor).toHaveValue("# Alpha\n");
    await userEvent.type(editor, "Meer");
    await userEvent.click(screen.getByRole("button", { name: /opslaan/i }));

    expect(await screen.findByText("Meer")).toBeInTheDocument();
    const [url, init] = fetchMock.mock.calls[1];
    expect(url).toBe("/api/notes/zettels%2Fa.md");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({ content: "# Alpha\nMeer", version: "v1" });
    expect(screen.queryByLabelText(/inhoud/i)).not.toBeInTheDocument();
  });

  it("keeps the edit and offers a reload when the file changed elsewhere", async () => {
    const fetchMock = stubFetch(
      alpha("v1", "oud", "<p>oud</p>"),
      { status: 409, body: { error: "zettels/a.md was changed in the meantime; reload it and edit again" } },
      alpha("v3", "van elders", "<p>van elders</p>"),
    );
    render(<NoteView noteRef="zettels/a.md" />);

    await userEvent.click(await screen.findByRole("button", { name: /bewerken/i }));
    await userEvent.type(screen.getByLabelText(/inhoud/i), " mijn tekst");
    await userEvent.click(screen.getByRole("button", { name: /opslaan/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent(/changed in the meantime/);
    expect(screen.getByLabelText(/inhoud/i)).toHaveValue("oud mijn tekst");

    await userEvent.click(screen.getByRole("button", { name: /herladen/i }));
    await waitFor(() => expect(screen.getByText("van elders")).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("cancels an edit without saving", async () => {
    const fetchMock = stubFetch(alpha("v1", "tekst", "<p>tekst</p>"));
    render(<NoteView noteRef="zettels/a.md" />);
    await userEvent.click(await screen.findByRole("button", { name: /bewerken/i }));
    await userEvent.type(screen.getByLabelText(/inhoud/i), " weg");
    await userEvent.click(screen.getByRole("button", { name: /annuleren/i }));
    expect(screen.getByText("tekst")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
