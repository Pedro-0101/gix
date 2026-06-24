/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import tailwindcss from "@tailwindcss/vite";

// https://vitejs.dev/config/
export default defineConfig({
  // Pure command logic has no DOM dependency, so the default 'node' environment
  // is enough — no jsdom needed.
  test: {
    include: ["src/**/*.test.ts"],
  },
  server: {
    // Wails' asset proxy dials IPv4 `tcp4 127.0.0.1:<port>`, so bind Vite to
    // 127.0.0.1 (not "localhost", which on Windows binds IPv6 ::1 only and makes
    // the proxy fail with "connection refused").
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react(), wails("./bindings"), tailwindcss()],
});
