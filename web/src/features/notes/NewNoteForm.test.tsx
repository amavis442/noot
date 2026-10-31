import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { NewNoteForm } from "./NewNoteForm";

describe("NewNoteForm", () => {
  it("creates a note and opens it", async () => {
    const fetchMock = stubFetch({ status: 201, body: { id: "N", path: "zettels/nieuw-idee.md", title: "Nieuw idee" } });
    render(<NewNoteForm />);

    await userEvent.type(screen.getByLabelText(/titel/i), "Nieuw idee");
    await userEvent.type(screen.getByLabelText(/tekst/i), "Eerste regel");
    await userEvent.type(screen.getByLabelText(/tags/i), "idee, werk");
    await userEvent.click(screen.getByRole("button", { name: /aanmaken/i }));

    await waitFor(() => expect(window.location.hash).toBe("#/note/zettels%2Fnieuw-idee.md"));
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      title: "Nieuw idee",
      body: "Eerste regel",
      tags: ["idee", "werk"],
    });
  });

  it("needs a title", async () => {
    const fetchMock = stubFetch();
    render(<NewNoteForm />);
    expect(screen.getByRole("button", { name: /aanmaken/i })).toBeDisabled();
    await userEvent.type(screen.getByLabelText(/titel/i), "   ");
    expect(screen.getByRole("button", { name: /aanmaken/i })).toBeDisabled();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shows the server's error and keeps the input", async () => {
    stubFetch({ status: 400, body: { error: 'invalid note: tag "a b" may only contain letters' } });
    render(<NewNoteForm />);
    await userEvent.type(screen.getByLabelText(/titel/i), "X");
    await userEvent.click(screen.getByRole("button", { name: /aanmaken/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent("may only contain letters");
    expect(screen.getByLabelText(/titel/i)).toHaveValue("X");
  });
});
