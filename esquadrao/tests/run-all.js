#!/usr/bin/env node
/**
 * Smoke test: confirma que todo agente e skill tem o frontmatter mínimo exigido,
 * e que hooks.json é JSON válido. Não testa comportamento do modelo — só a
 * integridade estrutural do plugin.
 */
const fs = require("node:fs");
const path = require("node:path");

const ROOT = path.resolve(__dirname, "..");
let failures = 0;

function checkFrontmatter(filePath, requiredFields) {
  const content = fs.readFileSync(filePath, "utf8");
  if (!content.startsWith("---")) {
    console.error(`[FAIL] ${filePath}: sem frontmatter YAML`);
    failures++;
    return;
  }
  for (const field of requiredFields) {
    if (!new RegExp(`^${field}:`, "m").test(content)) {
      console.error(`[FAIL] ${filePath}: campo obrigatório "${field}" ausente`);
      failures++;
    }
  }
}

function walk(dir, filename, cb) {
  if (!fs.existsSync(dir)) return;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, filename, cb);
    else if (entry.name === filename) cb(full);
  }
}

walk(path.join(ROOT, "agents"), null, () => {}); // placeholder, agents are files directly
for (const f of fs.readdirSync(path.join(ROOT, "agents"))) {
  if (f.endsWith(".md")) checkFrontmatter(path.join(ROOT, "agents", f), ["name", "description"]);
}

walk(path.join(ROOT, "skills"), "SKILL.md", (f) => checkFrontmatter(f, ["name", "description"]));

try {
  JSON.parse(fs.readFileSync(path.join(ROOT, "hooks", "hooks.json"), "utf8"));
} catch (e) {
  console.error(`[FAIL] hooks/hooks.json inválido: ${e.message}`);
  failures++;
}

try {
  JSON.parse(fs.readFileSync(path.join(ROOT, ".claude-plugin", "plugin.json"), "utf8"));
} catch (e) {
  console.error(`[FAIL] .claude-plugin/plugin.json inválido: ${e.message}`);
  failures++;
}

if (failures > 0) {
  console.error(`\n${failures} problema(s) encontrado(s).`);
  process.exit(1);
} else {
  console.log("OK — todos os agentes, skills e configs têm estrutura válida.");
}
