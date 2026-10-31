import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { AgendaView } from "./AgendaView";

function week(tuesday = { content: "### Planned", version: "d2", html: "<h3>Planned</h3>" }) {
  return {
    body: {
      week: "2026-W45",
      monday: "2026-11-02",
      id: "W",
      path: "agenda/2026-W45.md",
      title: "Agenda 2026-W45",
      version: "w1",
      content: "# Week 45, 2026\n",
      html: "<h1>Week 45, 2026</h1>",
      intro_html: "<h1>Week 45, 2026</h1>",
      days: [
        { date: "2026-11-02", heading: "Monday 2 November", content: "- [ ] Melk", version: "d1", html: "<ul><li>Melk</li></ul>" },
        { date: "2026-11-03", heading: "Tuesday 3 November", ...tuesday },
      ],
    },
  };
}

function day(name: string) {
  return screen.getByRole("region", { name });
}

describe("AgendaView", () => {
  it("shows the week with links to the weeks around it", async () => {
    const fetchMock = stubFetch(week());
    render(<AgendaView day="today" />);

    expect(await screen.findByRole("heading", { name: "Week 45, 2026" })).toBeInTheDocument();
    expect(fetchMock.mock.calls[0][0]).toBe("/api/agenda/today");
    expect(screen.getByRole("link", { name: /vorige week/i })).toHaveAttribute("href", "#/agenda/2026-10-26");
    expect(screen.getByRole("link", { name: /volgende week/i })).toHaveAttribute("href", "#/agenda/2026-11-09");
  });

  it("marks the note as an agenda, so the days get dividers", async () => {
    stubFetch(week());
    render(<AgendaView day="today" />);
    expect(await screen.findByRole("article")).toHaveClass("note", "agenda");
  });

  it("shows every day on its own with its own edit button", async () => {
    stubFetch(week());
    render(<AgendaView day="today" />);

    await screen.findByRole("heading", { name: "Monday 2 November" });
    expect(within(day("Monday 2 November")).getByText("Melk")).toBeInTheDocument();
    expect(within(day("Tuesday 3 November")).getByRole("heading", { name: "Planned" })).toBeInTheDocument();
    expect(within(day("Monday 2 November")).getByRole("button", { name: /bewerken/i })).toBeInTheDocument();
  });

  it("gives every day the class of its weekday, for its colour", async () => {
    stubFetch(week());
    render(<AgendaView day="today" />);

    await screen.findByRole("heading", { name: "Monday 2 November" });
    expect(day("Monday 2 November")).toHaveClass("agenda-day", "agenda-monday");
    expect(day("Tuesday 3 November")).toHaveClass("agenda-day", "agenda-tuesday");
  });

  it("edits one day and shows the week as it is now", async () => {
    const fetchMock = stubFetch(
      week(),
      { body: { date: "2026-11-03", heading: "Tuesday 3 November", content: "x", version: "d3", html: "" } },
      week({ content: "### Planned\nSporten", version: "d3", html: "<p>Sporten</p>" }),
    );
    render(<AgendaView day="2026-11-05" />);

    await screen.findByRole("heading", { name: "Tuesday 3 November" });
    await userEvent.click(within(day("Tuesday 3 November")).getByRole("button", { name: /bewerken/i }));
    const editor = within(day("Tuesday 3 November")).getByLabelText(/inhoud/i);
    expect(editor).toHaveValue("### Planned");
    await userEvent.type(editor, "\nSporten");
    await userEvent.click(within(day("Tuesday 3 November")).getByRole("button", { name: /opslaan/i }));

    await waitFor(() => expect(within(day("Tuesday 3 November")).getByText("Sporten")).toBeInTheDocument());
    const [url, init] = fetchMock.mock.calls[1];
    expect(url).toBe("/api/agenda/2026-11-03/day");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({ content: "### Planned\nSporten", version: "d2" });
    expect(fetchMock.mock.calls[2][0]).toBe("/api/agenda/2026-11-05");
    // Monday stays readable meanwhile.
    expect(within(day("Monday 2 November")).getByText("Melk")).toBeInTheDocument();
  });

  it("keeps a day's edit and offers a reload when that day changed elsewhere", async () => {
    const fetchMock = stubFetch(
      week(),
      { status: 409, body: { error: "Tuesday 3 November of agenda/2026-W45.md was changed in the meantime" } },
      week({ content: "van elders", version: "d9", html: "<p>van elders</p>" }),
    );
    render(<AgendaView day="2026-11-05" />);

    await screen.findByRole("heading", { name: "Tuesday 3 November" });
    await userEvent.click(within(day("Tuesday 3 November")).getByRole("button", { name: /bewerken/i }));
    await userEvent.type(within(day("Tuesday 3 November")).getByLabelText(/inhoud/i), " mijn tekst");
    await userEvent.click(within(day("Tuesday 3 November")).getByRole("button", { name: /opslaan/i }));

    expect(await within(day("Tuesday 3 November")).findByRole("alert")).toHaveTextContent(/changed in the meantime/);
    expect(within(day("Tuesday 3 November")).getByLabelText(/inhoud/i)).toHaveValue("### Planned mijn tekst");

    await userEvent.click(within(day("Tuesday 3 November")).getByRole("button", { name: /herladen/i }));
    await waitFor(() => expect(within(day("Tuesday 3 November")).getByText("van elders")).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("still edits the whole week", async () => {
    const fetchMock = stubFetch(week(), {
      body: { id: "W", path: "agenda/2026-W45.md", title: "", version: "w2", content: "", html: "<p>Sporten</p>" },
    });
    render(<AgendaView day="2026-11-05" />);

    await userEvent.click(await screen.findByRole("button", { name: /hele week bewerken/i }));
    await userEvent.type(screen.getByLabelText(/inhoud/i), "Sporten");
    await userEvent.click(screen.getByRole("button", { name: /opslaan/i }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    const [url, init] = fetchMock.mock.calls[1];
    expect(url).toBe("/api/notes/agenda%2F2026-W45.md");
    expect(JSON.parse(init.body)).toEqual({ content: "# Week 45, 2026\nSporten", version: "w1" });
  });
});
