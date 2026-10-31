import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { parseRoute, useHashRoute } from "./useHashRoute";

describe("parseRoute", () => {
  it("knows the note, new and home routes", () => {
    expect(parseRoute("#/note/zettels%2Fa.md")).toEqual({ name: "note", ref: "zettels/a.md" });
    expect(parseRoute("#/new")).toEqual({ name: "new" });
    expect(parseRoute("")).toEqual({ name: "home" });
    expect(parseRoute("#/whatever")).toEqual({ name: "home" });
  });

  it("knows the journal and reminders routes", () => {
    expect(parseRoute("#/journal")).toEqual({ name: "journal", day: "today" });
    expect(parseRoute("#/journal/2026-10-31")).toEqual({ name: "journal", day: "2026-10-31" });
    expect(parseRoute("#/reminders")).toEqual({ name: "reminders" });
    expect(parseRoute("#/agenda")).toEqual({ name: "agenda", day: "today" });
    expect(parseRoute("#/agenda/2026-11-02")).toEqual({ name: "agenda", day: "2026-11-02" });
  });

  it("treats a malformed escape as home", () => {
    expect(parseRoute("#/note/%E0%A4%A")).toEqual({ name: "home" });
  });
});

describe("useHashRoute", () => {
  it("follows hash changes", () => {
    const { result } = renderHook(() => useHashRoute());
    expect(result.current).toEqual({ name: "home" });
    act(() => {
      window.location.hash = "#/new";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(result.current).toEqual({ name: "new" });
  });
});
