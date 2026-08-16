#!/usr/bin/env node
// check-unlayered-css.cjs — teste de regressão (Tailwind v4).
//
// No Tailwind v4 (CSS Cascade Layers), uma regra SEM layer sempre vence
// qualquer regra DENTRO de layer, independente de especificidade. Foi o
// bug do reset `* { margin:0; padding:0 }` fora de @layer: anulava todas
// as utilities (p-6, lg:ml-64, px-4...) e o padding de .card/.btn/.input.
//
// Este script garante que nenhuma regra de estilo exista fora de @layer
// nos index.css. Regras top-level permitidas: @import, @layer, @theme,
// @custom-variant, @keyframes (e comentários).
//
// Uso: node scripts/check-unlayered-css.cjs <arquivo.css ...>

const fs = require("fs");

// @rules top-level que NÃO competem com utilities (não precisam de layer).
const ALLOWED_TOP_AT = new Set([
  "charset",
  "import",
  "layer",
  "theme",
  "custom-variant",
  "keyframes",
  "namespace",
]);

function scan(file) {
  const src = fs.readFileSync(file, "utf8").replace(/\r\n/g, "\n");
  const clean = src.replace(/\/\*[\s\S]*?\*\//g, ""); // remove comentários

  const issues = [];
  let inString = false;
  let strChar = "";
  let pendingAt = ""; // @rule do statement atual ("" = regra comum)
  let depth = 0; // profundidade de chaves
  let stmtStart = 0;

  for (let j = 0; j < clean.length; j++) {
    const c = clean[j];

    if (inString) {
      if (c === "\\") {
        j++;
        continue;
      }
      if (c === strChar) inString = false;
      continue;
    }
    if (c === '"' || c === "'") {
      inString = true;
      strChar = c;
      continue;
    }

    if (c === "@" && pendingAt === "") {
      const m = clean.slice(j).match(/^@([a-zA-Z-]+)/);
      if (m) {
        pendingAt = m[1];
        j += m[0].length - 1;
        continue;
      }
    }

    if (c === "{") {
      const at = pendingAt;
      if (depth === 0 && !ALLOWED_TOP_AT.has(at)) {
        const line = clean.slice(0, j).split("\n").length;
        const snippet = clean
          .slice(stmtStart, j)
          .trim()
          .replace(/\s+/g, " ")
          .slice(0, 80);
        issues.push(`${file}:${line} — regra fora de @layer: ${snippet}`);
      }
      depth++;
      pendingAt = "";
      stmtStart = j + 1;
      continue;
    }
    if (c === "}") {
      depth = Math.max(0, depth - 1);
      pendingAt = "";
      stmtStart = j + 1;
      continue;
    }
    if (c === ";") {
      pendingAt = "";
      stmtStart = j + 1;
      continue;
    }
  }
  return issues;
}

const files = process.argv.slice(2);
if (files.length === 0) {
  console.error("Uso: node scripts/check-unlayered-css.cjs <arquivo.css ...>");
  process.exit(2);
}

let total = 0;
for (const f of files) {
  const issues = scan(f);
  for (const i of issues) console.log(`X ${i}`);
  total += issues.length;
}

if (total > 0) {
  console.error(
    `\n${total} regra(s) CSS fora de @layer encontrada(s).\n` +
      "No Tailwind v4, regra sem layer sempre vence qualquer layer.\n" +
      "Envolva a regra em @layer base ou @layer components para nao anular utilities (ex.: p-6, lg:ml-64)."
  );
  process.exit(1);
}
console.log(`OK: ${files.length} arquivo(s) CSS sem regras fora de @layer.`);
