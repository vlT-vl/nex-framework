import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";

const envDir = fileURLToPath(new URL("..", import.meta.url));
const devHost = process.env.NEX_VITE_HOST ?? "127.0.0.1";
const devPort = Number(process.env.NEX_VITE_PORT ?? 5179);

export default defineConfig({
  plugins: [react()],
  base: "./",
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
      "/api": "http://127.0.0.1:34115",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
