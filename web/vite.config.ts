import { defineConfig } from "vite";
import { fileURLToPath, URL } from "node:url";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: true,
    rollupOptions: {
      output: {
        // Pull the big, stable vendor libraries out of the entry chunk into
        // long-lived, separately-cacheable chunks so a one-line app change
        // doesn't re-download all of React/TanStack/dnd-kit. These are all in
        // the *eager* import graph already (Board, Overview, the query client),
        // so grouping them only reorganizes the initial bundle — it never pulls
        // the lazy chunks (DiffPanel → @pierre/diffs + Shiki grammars, the
        // ghostty terminal) eager. Deliberately NOT grouped: anything Shiki
        // touches (e.g. hast-util-to-html, shared with react-markdown), since
        // forcing a Shiki dependency into an eager group would partly un-split
        // the diff chunk. Verify with a build that DiffPanel-*, wasm-* and the
        // per-language grammar chunks stay separate after changing this.
        codeSplitting: {
          groups: [
            {
              name: "react-vendor",
              test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/,
              priority: 100,
            },
            {
              name: "query-vendor",
              test: /node_modules[\\/]@tanstack[\\/]/,
              priority: 90,
            },
            {
              name: "dnd-vendor",
              test: /node_modules[\\/](@dnd-kit[\\/]|react-rnd[\\/]|react-draggable[\\/]|re-resizable[\\/])/,
              priority: 90,
            },
          ],
        },
      },
    },
  },
  server: {
    proxy: {
      "/api": `http://${process.env.KANBAN_BACKEND ?? "kanban:7474"}`,
      "/ws": { target: `ws://${process.env.KANBAN_BACKEND ?? "kanban:7474"}`, ws: true },
    },
  },
});
