// =============================================================
// vite.config.js — WebAdmin (Vite 6 + React 19 + Tailwind 4)
// =============================================================
// Migrado de webpack 5. Plugins:
//   - @vitejs/plugin-react → JSX + Fast Refresh (substitui babel-loader)
//   - @tailwindcss/vite     → Tailwind CSS v4 (CSS-first, sem postcss.config.js)
//
// outDir "build" mantém o staticPublishPath já configurado no render.yaml.
// public/ é copiado automaticamente pelo Vite (inclui _redirects).
// =============================================================
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],

  test: {
    environment: "jsdom",
    globals: true,
  },

  // JSX em arquivos .js (herança do babel-loader do webpack): o esbuild
  // por padrão só trata .jsx/.tsx como JSX — o projeto usa .js com JSX.
  esbuild: {
    loader: "jsx",
    include: /src\/.*\.jsx?$/,
    exclude: [],
  },

  // Vite usa import.meta.env.VITE_*; mantemos REACT_APP_* como alias
  // para não quebrar o render.yaml (que injeta REACT_APP_API_URL no build).
  envPrefix: ["VITE_", "REACT_APP_"],

  server: {
    host: "0.0.0.0",
    port: 3000,
  },

  build: {
    outDir: "build",
  },
});
