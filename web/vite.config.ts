import { writeFileSync } from "node:fs";
import react from "@vitejs/plugin-react";
import { defineConfig, type Plugin } from "vitest/config";

// The Go package web embeds dist/ and needs it to exist in a fresh checkout,
// so dist/.gitkeep is committed. Vite empties dist/ on every build; this puts
// the file back.
const keepGitkeep: Plugin = {
  name: "keep-gitkeep",
  closeBundle() {
    writeFileSync("dist/.gitkeep", "");
  },
};

export default defineConfig({
  plugins: [react(), keepGitkeep],
  server: {
    // `noot serve` on its default address answers the API calls.
    proxy: { "/api": "http://127.0.0.1:4242" },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/setupTests.ts"],
  },
});
