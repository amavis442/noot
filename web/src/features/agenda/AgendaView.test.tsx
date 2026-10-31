import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { AgendaView } from "./AgendaView";

const week = {
  body: {
    week: "2026-W45",
    monday: "2026-11-02",
    id: "W",
    path: "agenda/2026-W45.md",
    title: "Agenda 2026-W45",
    version: "w1",
    content: "# Week 45, 2026\n",
    html: "<h1>Week 45, 2026</h1>",
  },
};

describe("AgendaView", () => {
  it("shows the week with links to the weeks around it", async () => {
    const fetchMock = stubFetch(week);
    render(<AgendaView day="today" />);

    expect(await screen.findByRole("heading", { name: "Week 45, 2026" })).toBeInTheDocument();
    expect(fetchMock.mock.calls[0][0]).toBe("/api/agenda/today");
    expect(screen.getByRole("link", { name: /vorige week/i })).toHaveAttribute("href", "#/agenda/2026-10-26");
    expect(screen.getByRole("link", { name: /volgende week/i })).toHaveAttribute("href", "#/agenda/2026-11-09");
  });

  it("edits the week like any note", async () => {
    const fetchMock = stubFetch(week, {
      body: { ...week.body, week: undefined, monday: undefined, version: "w2", html: "<p>Sporten</p>" },
    });
    render(<AgendaView day="2026-11-05" />);

    await userEvent.click(await screen.findByRole("button", { name: /bewerken/i }));
    await userEvent.type(screen.getByLabelText(/inhoud/i), "Sporten");
    await userEvent.click(screen.getByRole("button", { name: /opslaan/i }));

    expect(await screen.findByText("Sporten")).toBeInTheDocument();
    const [url, init] = fetchMock.mock.calls[1];
    expect(url).toBe("/api/notes/agenda%2F2026-W45.md");
    expect(JSON.parse(init.body)).toEqual({ content: "# Week 45, 2026\nSporten", version: "w1" });
    expect(screen.getByRole("link", { name: /volgende week/i })).toHaveAttribute("href", "#/agenda/2026-11-09");
  });
});
