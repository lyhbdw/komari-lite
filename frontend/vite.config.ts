import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";


// https://vite.dev/config/
import type { Plugin, UserConfig } from "vite";
import * as fs from "fs";
import * as path from "path";

function localKomariThemePlugin(): Plugin {
  const themeRequestPath = "/themes/default/komari-theme.json";
  const localThemeFile = path.resolve(__dirname, "komari-theme.json");

  return {
    name: "local-komari-theme",
    apply: "serve",
    enforce: "pre",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (!req.url) return next();

        const url = new URL(req.url, "http://localhost");
        if (!url.pathname.endsWith(themeRequestPath)) return next();

        fs.readFile(localThemeFile, (err, data) => {
          if (err) {
            res.statusCode = 404;
            res.setHeader("Content-Type", "application/json; charset=utf-8");
            res.end(
              JSON.stringify({
                error: "Local theme file not found",
                file: localThemeFile,
              })
            );
            return;
          }

          res.statusCode = 200;
          res.setHeader("Content-Type", "application/json; charset=utf-8");
          res.setHeader("Cache-Control", "no-store");
          res.end(data);
        });
      });
    },
  };
}

export default defineConfig(({ mode }) => {
  const buildTime = new Date().toISOString();

  // Supports configuring BASE_URL via environment variables, defaulting to the root path.
  const base: string = process.env.VITE_BASE_URL ? process.env.VITE_BASE_URL : '/';
  const baseConfig: UserConfig = {
    base: base,
    plugins: [
      localKomariThemePlugin(),
      react(),
      tailwindcss(),
    ],
    define: {
      __BUILD_TIME__: JSON.stringify(buildTime),
    },
      resolve: {
        alias: [
          { find: "@", replacement: path.resolve(__dirname, "./src") },
          { find: "@radix-ui/themes", replacement: path.resolve(__dirname, "./src/components/ui/radix-shim.tsx") },
      ],
    },
    build: {
      assetsDir: "assets",
      outDir: "dist",
      chunkSizeWarningLimit: 800,
      rollupOptions: {
        output: {
          // go embed ignore files start with '_'
          chunkFileNames: "assets/chunk-[name]-[hash].js",
          entryFileNames: "assets/entry-[name]-[hash].js",
          // Do not use manualChunks, use React.lazy() and <Suspense> instead
        }
      },
    },
  };

  if (mode === "development") {
    const loadedEnv = loadEnv(mode, process.cwd(), "");
    const apiTarget = loadedEnv.VITE_API_TARGET || process.env.VITE_API_TARGET || "http://127.0.0.1:25774";
    baseConfig.server = {
      proxy: {
        "/api": {
          target: apiTarget,
          changeOrigin: true,
          rewriteWsOrigin: true,
          ws: true,
        },
        "/themes": {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    };
  }

  return baseConfig;
});
