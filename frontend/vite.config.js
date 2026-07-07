import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";

// Read .env from the repo root so Go and Vite share the same file.
const envDir = fileURLToPath(new URL("..", import.meta.url));
const devHost = process.env.NEX_VITE_HOST ?? "127.0.0.1";
const devPort = Number(process.env.NEX_VITE_PORT ?? 5179);

export default defineConfig({
  plugins: [react()],
  base: "./",          // relative paths so assets work when served from Go
  envDir,
  publicDir: "../res",
  server: {
    host: devHost,
    port: devPort,
    strictPort: true,
    fs: {
      allow: [envDir],
    },
    proxy: {
      // In dev mode Vite proxies /api to the Go server on its fixed port
      "/api": "http://127.0.0.1:34115",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
