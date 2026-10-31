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
});
