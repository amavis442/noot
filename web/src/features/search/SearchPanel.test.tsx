import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { stubFetch } from "../../shared/testing";
import { SearchPanel } from "./SearchPanel";

describe("SearchPanel", () => {
  it("shows hits with a highlighted snippet that link to the note", async () => {
    const fetchMock = stubFetch({
      body: [{ id: "A", path: "zettels/koffie.md", title: "Koffie", snippet: "met een «V60» filter", score: -1 }],
    });
    render(<SearchPanel />);

    await userEvent.type(screen.getByRole("searchbox", { name: /zoek/i }), "v60");

    const link = await screen.findByRole("link", { name: /koffie/i });
    expect(link).toHaveAttribute("href", "#/note/zettels%2Fkoffie.md");
    expect(screen.getByText("V60").tagName).toBe("MARK");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/search?q=v60");
  });

  it("says so when nothing matches", async () => {
    stubFetch({ body: [] });
    render(<SearchPanel />);
    await userEvent.type(screen.getByRole("searchbox", { name: /zoek/i }), "niets");
    expect(await screen.findByText(/geen notities gevonden/i)).toBeInTheDocument();
  });

  it("shows the server's error", async () => {
    stubFetch({ status: 400, body: { error: "fts5: syntax error" } });
    render(<SearchPanel />);
    await userEvent.type(screen.getByRole("searchbox", { name: /zoek/i }), "x");
    expect(await screen.findByRole("alert")).toHaveTextContent("fts5: syntax error");
  });

  it("does not search for an empty query", async () => {
    const fetchMock = stubFetch();
    render(<SearchPanel />);
    await userEvent.type(screen.getByRole("searchbox", { name: /zoek/i }), "   ");
    await new Promise((r) => setTimeout(r, 300));
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
