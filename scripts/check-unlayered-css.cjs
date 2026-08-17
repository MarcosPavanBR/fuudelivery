#!/usr/bin/env node
// check-unlayered-css.cjs — teste de regressão (Tailwind v4).
//
// No Tailwind v4 (CSS Cascade Layers), uma regra SEM layer sempre vence
// qualquer regra DENTRO de layer, independente de especificidade. Foi o
// bug do reset `* { margin:0; padding:0 }` fora de @layer: anulava todas
// as utilities (p-6, lg:ml-64, px-4...) e o padding de .card/.btn/.input.
//
// Este script garante que nenhuma regra de estilo exista fora de @layer
// em QUALQUER arquivo CSS customizado do projeto (não só nos index.css).
// Regras top-level permitidas: @import, @layer, @theme, @custom-variant,
// @keyframes (e comentários).
//
// Uso:
//   node scripts/check-unlayered-css.cjs            # descobre todos os CSS do repo
//   node scripts/check-unlayered-css.cjs <arquivo.css ...>   # só os listados
//   node scripts/check-unlayered-css.cjs --root <dir>        # descobre a partir de <dir>

const fs = require("fs");
const path = require("path");

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

// Diretórios gerados / vendidos — nunca são CSS customizado do projeto.
const SKIP_DIRS = new Set([
  "node_modules",
  ".git",
  "dist",
  "build",
  ".next",
  ".expo",
  ".cache",
  "coverage",
  "vendor",
  ".venv",
  "venv",
  "tmp",
  ".pg-embed",
  ".freebuff",
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

// Descobre todos os arquivos CSS customizados do projeto a partir de root,
// pulando diretórios gerados/vendidos. Ignora .min.css (build artifacts).
function discover(root) {
  const out = [];
  (function walk(dir) {
    let entries;
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const e of entries) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (e.name.startsWith(".") || SKIP_DIRS.has(e.name)) continue;
        walk(p);
      } else if (
        e.isFile() &&
        e.name.endsWith(".css") &&
        !e.name.endsWith(".min.css")
      ) {
        out.push(p);
      }
    }
  })(root);
  return out.sort();
}

// CLI: --root <dir> muda a base da descoberta; sem arquivos explícitos,
// varre o repositório inteiro (o CI cobre CSS novos automaticamente).
const args = process.argv.slice(2);
let root = process.cwd();
const files = [];
for (let i = 0; i < args.length; i++) {
  const a = args[i];
  if (a === "--root") {
    root = args[++i] || root;
  } else if (a.startsWith("--root=")) {
    root = a.slice("--root=".length);
  } else {
    files.push(a);
  }
}

const targets = files.length > 0 ? files : discover(root);
if (targets.length === 0) {
  console.error(`Nenhum arquivo CSS encontrado em ${root}.`);
  process.exit(2);
}

const rel = (p) => path.relative(process.cwd(), p).replace(/\\/g, "/") || p;

let total = 0;
for (const f of targets) {
  const issues = scan(f);
  for (const i of issues) console.log(`X ${i}`);
  total += issues.length;
}

if (total > 0) {
  console.error(
    `\n${total} regra(s) CSS fora de @layer encontrada(s) em ${targets.length} arquivo(s).\n` +
      "No Tailwind v4, regra sem layer sempre vence qualquer layer.\n" +
      "Envolva a regra em @layer base ou @layer components para nao anular utilities (ex.: p-6, lg:ml-64)."
  );
  process.exit(1);
}
console.log(
  `OK: ${targets.length} arquivo(s) CSS sem regras fora de @layer (${rel(root)}).`
);
