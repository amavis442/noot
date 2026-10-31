import { describe, expect, it } from "vitest";
import { agendaHref, journalHref, shiftDay } from "./days";

describe("shiftDay", () => {
  it("moves across month and year ends", () => {
    expect(shiftDay("2026-10-31", 1)).toBe("2026-11-01");
    expect(shiftDay("2026-01-01", -1)).toBe("2025-12-31");
    expect(shiftDay("2028-02-28", 1)).toBe("2028-02-29");
    expect(shiftDay("2026-12-28", 7)).toBe("2027-01-04");
  });
});

describe("routes", () => {
  it("link to the journal and agenda of a day", () => {
    expect(journalHref("2026-10-31")).toBe("#/journal/2026-10-31");
    expect(agendaHref("2026-11-02")).toBe("#/agenda/2026-11-02");
  });
});
