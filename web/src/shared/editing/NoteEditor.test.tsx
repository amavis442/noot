import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { stubFetch, stubPreview } from "../testing";
import { NoteEditor } from "./NoteEditor";

function editor(onSave = vi.fn()) {
  return render(
    <NoteEditor
      path="zettels/a.md"
      initial="# Alpha"
      initialHtml="<h1>Alpha</h1>"
      saving={false}
      onSave={onSave}
      onCancel={vi.fn()}
    />,
  );
}

describe("NoteEditor", () => {
  it("starts with the preview of the saved note, without asking the server", async () => {
    stubFetch();
    const previewMock = stubPreview();
    editor();

    const preview = screen.getByRole("region", { name: /voorbeeld/i });
    expect(preview).toContainHTML("<h1>Alpha</h1>");
    await new Promise((r) => setTimeout(r, 400));
    expect(previewMock).not.toHaveBeenCalled();
  });

  it("follows the text once typing stops", async () => {
    stubFetch();
    const previewMock = stubPreview({ body: { html: "<h1>Alpha</h1><p>Meer tekst</p>" } });
    editor();

    await userEvent.type(screen.getByLabelText(/inhoud/i), "\n\nMeer tekst");

    const preview = screen.getByRole("region", { name: /voorbeeld/i });
    await waitFor(() => expect(preview).toContainHTML("<p>Meer tekst</p>"));
    // One request for the whole burst of keystrokes, with the note's path.
    expect(previewMock).toHaveBeenCalledTimes(1);
    const [url, init] = previewMock.mock.calls[0];
    expect(url).toBe("/api/preview");
    expect(JSON.parse(String(init?.body))).toEqual({ path: "zettels/a.md", content: "# Alpha\n\nMeer tekst" });
  });

  it("keeps the last preview and says why a new one failed", async () => {
    stubFetch();
    stubPreview({ status: 400, body: { error: "frontmatter: yaml: line 1: did not find expected ']'" } });
    editor();

    await userEvent.type(screen.getByLabelText(/inhoud/i), "!");

    expect(await screen.findByText(/did not find expected/)).toBeInTheDocument();
    expect(screen.getByRole("region", { name: /voorbeeld/i })).toContainHTML("<h1>Alpha</h1>");
  });

  it("saves what was typed", async () => {
    stubFetch();
    const onSave = vi.fn();
    editor(onSave);
    await userEvent.type(screen.getByLabelText(/inhoud/i), "!");
    await userEvent.click(screen.getByRole("button", { name: /opslaan/i }));
    expect(onSave).toHaveBeenCalledWith("# Alpha!");
  });
});
