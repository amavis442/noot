import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { RemindersPage } from "./RemindersPage";

const tandarts = { n: 2, text: "Tandarts bellen", done: false, due: "2026-11-06" };
const melk = { n: 3, text: "Melk", done: false, planned: "2026-11-02" };

describe("RemindersPage", () => {
  it("lists open reminders with their dates", async () => {
    const fetchMock = stubFetch({ body: [tandarts, melk] });
    render(<RemindersPage />);

    const items = await screen.findAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText("Tandarts bellen")).toBeInTheDocument();
    expect(within(items[0]).getByText(/uiterlijk 2026-11-06/)).toBeInTheDocument();
    expect(within(items[1]).getByText(/gepland 2026-11-02/)).toBeInTheDocument();
    expect(fetchMock.mock.calls[0][0]).toBe("/api/reminders?all=false");
  });

  it("adds a reminder and reloads the list", async () => {
    const fetchMock = stubFetch({ body: [] }, { status: 201, body: { n: 1, text: "Fiets", done: false } }, { body: [{ n: 1, text: "Fiets", done: false, due: "2026-11-01" }] });
    render(<RemindersPage />);
    await screen.findByText(/geen open reminders/i);

    await userEvent.type(screen.getByLabelText(/nieuwe reminder/i), "Fiets");
    await userEvent.type(screen.getByLabelText(/uiterlijk/i), "morgen");
    await userEvent.click(screen.getByRole("button", { name: /toevoegen/i }));

    expect(await screen.findByText("Fiets")).toBeInTheDocument();
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ text: "Fiets", due: "morgen" });
    expect(screen.getByLabelText(/nieuwe reminder/i)).toHaveValue("");
  });

  it("marks a reminder done", async () => {
    const fetchMock = stubFetch({ body: [tandarts] }, { body: { ...tandarts, done: true } }, { body: [] });
    render(<RemindersPage />);

    await userEvent.click(await screen.findByRole("checkbox", { name: /tandarts bellen/i }));

    expect(await screen.findByText(/geen open reminders/i)).toBeInTheDocument();
    expect(fetchMock.mock.calls[1][0]).toBe("/api/reminders/2/done");
  });

  it("plans a reminder on a day", async () => {
    const fetchMock = stubFetch(
      { body: [tandarts] },
      { body: { reminder: { ...tandarts, planned: "2026-11-04" }, agenda: "agenda/2026-W45.md" } },
      { body: [{ ...tandarts, planned: "2026-11-04" }] },
    );
    render(<RemindersPage />);

    await userEvent.type(await screen.findByLabelText(/inplannen op/i), "woensdag");
    await userEvent.click(screen.getByRole("button", { name: /inplannen/i }));

    expect(await screen.findByText(/gepland 2026-11-04/)).toBeInTheDocument();
    expect(fetchMock.mock.calls[1][0]).toBe("/api/reminders/2/plan");
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ day: "woensdag" });
  });

  it("shows done reminders when asked", async () => {
    const fetchMock = stubFetch({ body: [] }, { body: [{ n: 1, text: "Oud", done: true, done_on: "2026-10-20" }] });
    render(<RemindersPage />);
    await screen.findByText(/geen open reminders/i);

    await userEvent.click(screen.getByRole("checkbox", { name: /toon afgeronde/i }));

    const done = await screen.findByRole("checkbox", { name: "Oud" });
    expect(done).toBeChecked();
    expect(done).toBeDisabled();
    expect(screen.getByText(/afgerond 2026-10-20/)).toBeInTheDocument();
    expect(fetchMock.mock.calls[1][0]).toBe("/api/reminders?all=true");
  });

  it("shows the server's error", async () => {
    stubFetch({ body: [] }, { status: 400, body: { error: '"ooit" is not a day' } });
    render(<RemindersPage />);
    await screen.findByText(/geen open reminders/i);
    await userEvent.type(screen.getByLabelText(/nieuwe reminder/i), "X");
    await userEvent.type(screen.getByLabelText(/uiterlijk/i), "ooit");
    await userEvent.click(screen.getByRole("button", { name: /toevoegen/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent('"ooit" is not a day');
  });
});
