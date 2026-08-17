#!/usr/bin/env node
// check-unlayered-css.test.cjs — teste unitário (regressão) do
// check-unlayered-css.cjs. Garante que o próprio checker não regrida:
// arquivo CSS limpo passa (exit 0) e arquivo com regra solta falha (exit 1).
//
// Sem dependências: só Node built-ins (fs/os/path/child_process), rodando o
// script real como subprocesso — cobre parser, saída e código de exit.
//
// Uso: node scripts/check-unlayered-css.test.cjs

const { spawnSync } = require("child_process");
const fs = require("fs");
const os = require("os");
const path = require("path");

const SCRIPT = path.join(__dirname, "check-unlayered-css.cjs");

let failures = 0;
let total = 0;

function check(name, cond, detail) {
  total++;
  if (cond) {
    console.log(`ok - ${name}`);
  } else {
    failures++;
    console.error(`FAIL - ${name}${detail ? `\n  ${detail}` : ""}`);
  }
}

// Cria um fixture CSS temporário e devolve {dir, file} para limpeza.
function makeTmp(content, name = "fixture.css") {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "unlayered-test-"));
  const file = path.join(dir, name);
  fs.writeFileSync(file, content, "utf8");
  return { dir, file };
}

// Roda o checker contra um arquivo (modo explícito) e devolve {code, stdout, stderr}.
function runFile(file) {
  const res = spawnSync(process.execPath, [SCRIPT, file], { encoding: "utf8" });
  if (res.error) throw res.error;
  return { code: res.status, stdout: res.stdout || "", stderr: res.stderr || "" };
}

// Roda o checker em modo descoberta a partir de um diretório.
function runRoot(root) {
  const res = spawnSync(process.execPath, [SCRIPT, "--root", root], {
    encoding: "utf8",
  });
  if (res.error) throw res.error;
  return { code: res.status, stdout: res.stdout || "", stderr: res.stderr || "" };
}

// ── 1. CSS limpo (tudo dentro de @layer) → passa ──────────────────────────
{
  const { dir, file } = makeTmp(
    [
      `@layer base {`,
      `  * { margin: 0; padding: 0; box-sizing: border-box; }`,
      `  body { background: #fff; }`,
      `}`,
      `@layer components { .card { border-radius: 12px; } }`,
      `@layer utilities { .p-6 { padding: 24px; } }`,
      ``,
    ].join("\n")
  );
  const r = runFile(file);
  check(
    "CSS limpo (tudo em @layer) passa com exit 0",
    r.code === 0,
    `exit=${r.code} stderr=${r.stderr.trim()}`
  );
  fs.rmSync(dir, { recursive: true, force: true });
}

// ── 2. Regra solta no top level → falha com exit 1 e mensagem ─────────────
{
  const { dir, file } = makeTmp(`* { margin: 0; }\n`);
  const r = runFile(file);
  const out = r.stdout + r.stderr;
  check(
    "regra solta fora de @layer falha com exit 1",
    r.code === 1,
    `exit=${r.code}`
  );
  check(
    "mensagem cita 'regra fora de @layer'",
    /regra fora de @layer/.test(out),
    `stdout=${r.stdout.trim()}`
  );
  check(
    "mensagem cita o arquivo do fixture",
    out.includes(file),
    `stdout=${r.stdout.trim()}`
  );
  fs.rmSync(dir, { recursive: true, force: true });
}

// ── 3. At-rules top-level permitidas → passa ──────────────────────────────
{
  const { dir, file } = makeTmp(
    [
      `@import "tailwindcss";`,
      `@theme { --color-foo: #000; }`,
      `@custom-variant dark (&:where(.dark, .dark *));`,
      `@keyframes spin { to { transform: rotate(360deg); } }`,
      ``,
    ].join("\n")
  );
  const r = runFile(file);
  check(
    "at-rules permitidas (@import/@theme/@custom-variant/@keyframes) passam",
    r.code === 0,
    `exit=${r.code} stderr=${r.stderr.trim()}`
  );
  fs.rmSync(dir, { recursive: true, force: true });
}

// ── 4. @media aninhado dentro de @layer → passa ───────────────────────────
{
  const { dir, file } = makeTmp(
    [
      `@layer components {`,
      `  @media (min-width: 640px) { .card { padding: 16px; } }`,
      `}`,
      ``,
    ].join("\n")
  );
  const r = runFile(file);
  check(
    "regras aninhadas (@media) dentro de @layer passam",
    r.code === 0,
    `exit=${r.code} stderr=${r.stderr.trim()}`
  );
  fs.rmSync(dir, { recursive: true, force: true });
}

// ── 5. @media no top level (sem layer) → falha ────────────────────────────
{
  const { dir, file } = makeTmp(
    `@media (min-width: 640px) { .card { padding: 16px; } }\n`
  );
  const r = runFile(file);
  check(
    "@media no top level (estilo fora de layer) falha",
    r.code === 1,
    `exit=${r.code}`
  );
  fs.rmSync(dir, { recursive: true, force: true });
}

// ── 6. Modo multi-arquivo: um limpo + um ruim → falha ─────────────────────
{
  const { dir, file: good } = makeTmp(`@layer base { body { margin: 0; } }\n`, "good.css");
  const bad = path.join(dir, "bad.css");
  fs.writeFileSync(bad, `.bad { color: red; }\n`, "utf8");
  const res = spawnSync(process.execPath, [SCRIPT, good, bad], { encoding: "utf8" });
  check(
    "multi-arquivo com qualquer regra solta falha",
    res.status === 1,
    `exit=${res.status}`
  );
  fs.rmSync(dir, { recursive: true, force: true });
}

// ── 7. Modo descoberta (--root): acha regra solta e falha ─────────────────
{
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "unlayered-root-"));
  // subárvore limpa (tudo em @layer)
  fs.mkdirSync(path.join(root, "src"), { recursive: true });
  fs.writeFileSync(
    path.join(root, "src", "index.css"),
    `@layer utilities { .p-6 { padding: 24px; } }\n`,
    "utf8"
  );
  // subárvore com regra solta (deve derrubar a descoberta a partir de root)
  fs.mkdirSync(path.join(root, "overlay"), { recursive: true });
  fs.writeFileSync(
    path.join(root, "overlay", "brand.css"),
    `.brand { color: red; }\n`,
    "utf8"
  );
  // lixo que a descoberta deve pular
  fs.mkdirSync(path.join(root, "node_modules"), { recursive: true });
  fs.writeFileSync(path.join(root, "node_modules", "bad.css"), `* { margin: 0; }\n`, "utf8");
  fs.mkdirSync(path.join(root, "dist"), { recursive: true });
  fs.writeFileSync(path.join(root, "dist", "bad.css"), `* { margin: 0; }\n`, "utf8");

  const rClean = runRoot(path.join(root, "src"));
  check(
    "descoberta em root limpo passa",
    rClean.code === 0,
    `exit=${rClean.code}`
  );

  const rBad = runRoot(root);
  check(
    "descoberta acha brand.css solto e falha (ignora node_modules/dist)",
    rBad.code === 1 && /brand\.css/.test(rBad.stdout + rBad.stderr),
    `exit=${rBad.code} stdout=${rBad.stdout.trim()}`
  );
  fs.rmSync(root, { recursive: true, force: true });
}

// ── 8. Arquivo vazio → passa ──────────────────────────────────────────────
{
  const { dir, file } = makeTmp(``);
  const r = runFile(file);
  check("arquivo vazio passa", r.code === 0, `exit=${r.code}`);
  fs.rmSync(dir, { recursive: true, force: true });
}

// ── Resumo ────────────────────────────────────────────────────────────────
if (failures > 0) {
  console.error(`\n${failures}/${total} teste(s) do check-unlayered-css FALHARAM.`);
  process.exit(1);
}
console.log(`\nOK: ${total}/${total} testes do check-unlayered-css passaram.`);
