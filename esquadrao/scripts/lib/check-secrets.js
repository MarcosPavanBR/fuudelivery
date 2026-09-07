#!/usr/bin/env node
/**
 * Rede de segurança leve, rodada via hook PreToolUse antes de comandos Bash.
 * NÃO substitui o agente security-reviewer — só pega os padrões mais óbvios
 * antes que cheguem perto de um `git commit`.
 *
 * Ver skills/credential-hygiene para o checklist completo.
 */
const { execSync } = require("node:child_process");

const SUSPECT_PATTERNS = [
  /sk-[a-zA-Z0-9]{20,}/,       // chaves de API estilo "sk-..."
  /AKIA[0-9A-Z]{16}/,          // AWS access key id
  /Bearer\s+[a-zA-Z0-9._-]{20,}/,
  /(mongodb(\+srv)?:\/\/[^\s"']+:[^\s"']+@)/i, // connection string com credencial embutida
];

function stagedDiff() {
  try {
    return execSync("git diff --cached", { encoding: "utf8" });
  } catch {
    return "";
  }
}

function main() {
  const diff = stagedDiff();
  if (!diff) return;

  const hits = SUSPECT_PATTERNS.filter((re) => re.test(diff));
  if (hits.length > 0) {
    console.error(
      "[esquadrao] Possível segredo no diff staged. Revise antes de commitar " +
        "(ver skills/credential-hygiene). Isto é um aviso automático simples, " +
        "não substitui a revisão do agente security-reviewer."
    );
    // Aviso, não bloqueio automático — falsos positivos em regex de segredo são comuns.
  }
}

main();
