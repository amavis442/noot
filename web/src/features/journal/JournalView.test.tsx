import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { JournalView } from "./JournalView";

function day(version: string, content: string, html: string) {
  return {
    body: { date: "2026-10-31", version, id: "J", path: "journal/2026-10-31.md", title: "2026-10-31", content, html },
  };
}

describe("JournalView", () => {
  it("shows the day with links to the days around it", async () => {
    const fetchMock = stubFetch(day("v1", "# Zaterdag", "<h1>Zaterdag</h1>"));
    render(<JournalView day="today" />);

    expect(await screen.findByRole("heading", { name: "Zaterdag" })).toBeInTheDocument();
    expect(fetchMock.mock.calls[0][0]).toBe("/api/journal/today");
    expect(screen.getByRole("link", { name: /vorige dag/i })).toHaveAttribute("href", "#/journal/2026-10-30");
    expect(screen.getByRole("link", { name: /volgende dag/i })).toHaveAttribute("href", "#/journal/2026-11-01");
  });

  it("saves an edit as the note it is", async () => {
    const fetchMock = stubFetch(day("v1", "# Zaterdag\n", "<h1>Zaterdag</h1>"), {
      body: { id: "J", path: "journal/2026-10-31.md", title: "2026-10-31", version: "v2", content: "x", html: "<p>Wandeling</p>" },
    });
    render(<JournalView day="2026-10-31" />);

    await userEvent.click(await screen.findByRole("button", { name: /bewerken/i }));
    await userEvent.type(screen.getByLabelText(/inhoud/i), "Wandeling");
    await userEvent.click(screen.getByRole("button", { name: /opslaan/i }));

    expect(await screen.findByText("Wandeling")).toBeInTheDocument();
    expect(fetchMock.mock.calls[1][0]).toBe("/api/notes/journal%2F2026-10-31.md");
    // The save answer has no date; the day navigation stays.
    expect(screen.getByRole("link", { name: /volgende dag/i })).toHaveAttribute("href", "#/journal/2026-11-01");
  });
});
