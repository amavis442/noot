import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";
import { resetPreview } from "./shared/testing";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  resetPreview();
  window.location.hash = "";
});
