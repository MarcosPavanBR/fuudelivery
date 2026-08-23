#!/usr/bin/env node
/**
 * FuuPayment Panel — build script
 *
 * O painel é um HTML estático standalone (sem bundle, sem build de JS).
 * Este script apenas garante que o diretório dist/ exista e copia o
 * index.html para ele, pronto para deploy em hostings estáticos (Render,
 * Netlify, Vercel, etc).
 *
 * Uso:
 *   npm run build        → copia index.html para dist/
 *   npm run build -- --clean → remove dist/ antes de copiar (build limpa)
 */
'use strict';

const fs = require('fs');
const path = require('path');

// ─── Caminhos ────────────────────────────────────────────────
const ROOT = path.resolve(__dirname, '..');
const DIST_DIR = path.join(ROOT, 'dist');
const SOURCE_FILE = path.join(ROOT, 'index.html');
const TARGET_FILE = path.join(DIST_DIR, 'index.html');

// ─── Utilitários ─────────────────────────────────────────────
function log(message) {
  console.log(`[build] ${message}`);
}

function cleanDist() {
  if (fs.existsSync(DIST_DIR)) {
    fs.rmSync(DIST_DIR, { recursive: true, force: true });
    log(`removido ${path.relative(ROOT, DIST_DIR)}/`);
  }
}

function copyIndexHtml() {
  if (!fs.existsSync(SOURCE_FILE)) {
    console.error(`ERRO: ${path.relative(ROOT, SOURCE_FILE)} não encontrado.`);
    process.exit(1);
  }

  fs.mkdirSync(DIST_DIR, { recursive: true });
  fs.copyFileSync(SOURCE_FILE, TARGET_FILE);
  log(`copiado ${path.relative(ROOT, SOURCE_FILE)} → ${path.relative(ROOT, TARGET_FILE)}`);
}

// ─── Execução ────────────────────────────────────────────────
const shouldClean = process.argv.includes('--clean');
if (shouldClean) {
  cleanDist();
}

copyIndexHtml();
log('Build OK ✔');
