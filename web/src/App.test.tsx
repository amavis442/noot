import { act, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { stubFetch } from "./shared/testing";

function go(hash: string) {
  act(() => {
    window.location.hash = hash;
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
}

describe("App", () => {
  it("shows search, a link to a new note, and the routed page", async () => {
    stubFetch({ body: { id: "A", path: "a.md", title: "Alpha", content: "", html: "<h1>Alpha</h1>" } });
    render(<App />);

    expect(screen.getByRole("searchbox", { name: /zoek/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /nieuwe notitie/i })).toHaveAttribute("href", "#/new");

    go("#/new");
    expect(screen.getByLabelText(/titel/i)).toBeInTheDocument();

    go("#/note/a.md");
    expect(await screen.findByRole("heading", { name: "Alpha" })).toBeInTheDocument();
  });

  it("links to today's journal and the reminders", async () => {
    stubFetch(
      { body: { date: "2026-10-31", version: "v", id: "J", path: "journal/2026-10-31.md", title: "", content: "", html: "<h1>Dag</h1>" } },
      { body: [] },
    );
    render(<App />);
    expect(screen.getByRole("link", { name: /vandaag/i })).toHaveAttribute("href", "#/journal");
    expect(screen.getByRole("link", { name: /reminders/i })).toHaveAttribute("href", "#/reminders");
    expect(screen.getByRole("link", { name: /agenda/i })).toHaveAttribute("href", "#/agenda");

    go("#/journal");
    expect(await screen.findByRole("heading", { name: "Dag" })).toBeInTheDocument();

    go("#/reminders");
    expect(await screen.findByText(/geen open reminders/i)).toBeInTheDocument();
  });

  it("routes to this week's agenda", async () => {
    stubFetch({ body: { week: "2026-W44", monday: "2026-10-26", id: "W", path: "agenda/2026-W44.md", title: "", version: "w", content: "", html: "<h1>Week 44</h1>" } });
    render(<App />);
    go("#/agenda");
    expect(await screen.findByRole("heading", { name: "Week 44" })).toBeInTheDocument();
  });
});
