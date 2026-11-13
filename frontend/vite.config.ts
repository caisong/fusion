import { execSync } from "child_process";
import path from "path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { tanstackRouter } from "@tanstack/router-vite-plugin";
import { defineConfig } from "vite";

function getGitVersion(): string {
  try {
    return execSync("git describe --tags --always").toString().trim();
  } catch {
    return "unknown";
  }
}

// https://vite.dev/config/
export default defineConfig(({ command }) => ({
  // Development serves from the root. Production builds use relative URLs so
  // the same bundle runs at any mount point; the backend injects a <base> tag
  // into index.html based on FUSION_BASE_PATH.
  base: command === "build" ? "./" : "/",
  define: {
    __APP_VERSION__: JSON.stringify(getGitVersion()),
  },
  plugins: [react(), tanstackRouter(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
}));
