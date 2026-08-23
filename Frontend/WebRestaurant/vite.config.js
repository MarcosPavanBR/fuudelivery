/// <reference types="vitest/config" />
// =============================================================
// vite.config.js — WebRestaurant (Vite 6 + React 19 + Tailwind 4)
// =============================================================
// Migrado de webpack 5. Plugins:
//   - @vitejs/plugin-react  → JSX + Fast Refresh (substitui babel-loader)
//   - @tailwindcss/vite     → Tailwind CSS v4 (CSS-first, sem postcss.config.js)
//
// public/ é copiado automaticamente pelo Vite (manifest, favicons,
// _redirects, brand/) — substitui o CopyPlugin do webpack.
// =============================================================
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],

  // JSX em arquivos .js (herança do babel-loader do webpack): o esbuild
  // por padrão só trata .jsx/.tsx como JSX — o projeto usa .js com JSX.
  esbuild: {
    loader: "jsx",
    include: /src\/.*\.jsx?$/,
    exclude: [],
  },

  // O dep-scan do Vite (optimizeDeps) ignora esbuild.loader acima — sem isso
  // o `npm run dev` quebra ao escanear .js com JSX ("The JSX syntax extension
  // is not currently enabled"). O build de produção não é afetado.
  optimizeDeps: {
    esbuildOptions: {
      loader: { ".js": "jsx" },
    },
  },

  // Vite usa import.meta.env.VITE_*; mantemos REACT_APP_* como alias
  // para não quebrar o render.yaml (que injeta REACT_APP_API_URL no build).
  envPrefix: ["VITE_", "REACT_APP_"],

  server: {
    host: "0.0.0.0",
    port: 3000,
  },

  build: {
    outDir: "dist",

    // Separa vendors pesados em chunks próprios (cacheados entre deploys,
    // já que o hash só muda quando a lib muda) e evita um bundle único gigante.
    rollupOptions: {
      output: {
        manualChunks: {
          "vendor-react": ["react", "react-dom", "react-router-dom"],
          "vendor-dnd": ["@hello-pangea/dnd"],
        },
      },
    },
  },

  // Configuração do vitest (substitui react-scripts test)
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.js"],
  },
});
