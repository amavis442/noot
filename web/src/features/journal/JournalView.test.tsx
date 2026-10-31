import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { JournalView } from "./JournalView";
import { shiftDay } from "./days";

function day(version: string, content: string, html: string) {
  return {
    body: { date: "2026-10-31", version, id: "J", path: "journal/2026-10-31.md", title: "2026-10-31", content, html },
  };
}

describe("shiftDay", () => {
  it("moves across month and year ends", () => {
    expect(shiftDay("2026-10-31", 1)).toBe("2026-11-01");
    expect(shiftDay("2026-01-01", -1)).toBe("2025-12-31");
    expect(shiftDay("2028-02-28", 1)).toBe("2028-02-29");
  });
});

describe("JournalView", () => {
  it("shows the day with links to the days around it", async () => {
    const fetchMock = stubFetch(day("v1", "# Zaterdag", "<h1>Zaterdag</h1>"));
    render(<JournalView day="today" />);

    expect(await screen.findByRole("heading", { name: "Zaterdag" })).toBeInTheDocument();
    expect(fetchMock.mock.calls[0][0]).toBe("/api/journal/today");
    expect(screen.getByRole("link", { name: /vorige dag/i })).toHaveAttribute("href", "#/journal/2026-10-30");
    expect(screen.getByRole("link", { name: /volgende dag/i })).toHaveAttribute("href", "#/journal/2026-11-01");
  });

  it("edits the whole note and saves it with its version", async () => {
    const fetchMock = stubFetch(day("v1", "# Zaterdag\n", "<h1>Zaterdag</h1>"), day("v2", "# Zaterdag\nWandeling", "<p>Wandeling</p>"));
    render(<JournalView day="2026-10-31" />);

    await userEvent.click(await screen.findByRole("button", { name: /bewerken/i }));
    const editor = screen.getByLabelText(/inhoud/i);
    expect(editor).toHaveValue("# Zaterdag\n");
    await userEvent.type(editor, "Wandeling");
    await userEvent.click(screen.getByRole("button", { name: /opslaan/i }));

    expect(await screen.findByText("Wandeling")).toBeInTheDocument();
    const [url, init] = fetchMock.mock.calls[1];
    expect(url).toBe("/api/journal/2026-10-31");
    expect(JSON.parse(init.body)).toEqual({ content: "# Zaterdag\nWandeling", version: "v1" });
    expect(screen.queryByLabelText(/inhoud/i)).not.toBeInTheDocument();
  });

  it("keeps the edit and offers a reload when the file changed elsewhere", async () => {
    const fetchMock = stubFetch(
      day("v1", "oud", "<p>oud</p>"),
      { status: 409, body: { error: "journal/2026-10-31.md was changed in the meantime; reload it and edit again" } },
      day("v3", "van elders", "<p>van elders</p>"),
    );
    render(<JournalView day="2026-10-31" />);

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
    const fetchMock = stubFetch(day("v1", "tekst", "<p>tekst</p>"));
    render(<JournalView day="2026-10-31" />);
    await userEvent.click(await screen.findByRole("button", { name: /bewerken/i }));
    await userEvent.type(screen.getByLabelText(/inhoud/i), " weg");
    await userEvent.click(screen.getByRole("button", { name: /annuleren/i }));
    expect(screen.getByText("tekst")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
