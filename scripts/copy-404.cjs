// Cria 404.html como cópia do index.html buildado.
// Fallback SPA para hosts estáticos (Netlify, GitHub Pages, Cloudflare
// Pages, etc.) que servem um 404.html customizado para rotas inexistentes.
// Uso: node copy-404.cjs <outDir>   (ex.: build, dist)
const fs = require("fs");
const path = require("path");

const outDir = process.argv[2];
if (!outDir) {
  console.error("Uso: node copy-404.cjs <outDir>");
  process.exit(1);
}

const src = path.join(outDir, "index.html");
const dest = path.join(outDir, "404.html");

if (!fs.existsSync(src)) {
  console.error(`index.html nao encontrado em ${src}`);
  process.exit(1);
}

fs.copyFileSync(src, dest);
console.log(`404.html criado (copia de index.html) em ${dest}`);
