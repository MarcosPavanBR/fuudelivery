// =============================================================
// CONFIG CENTRAL DA API — FONTE ÚNICA DE URLs DOS APPS MOBILE
// =============================================================
// Lista canônica de todas as URLs de produção: references/URLS.md
// (na raiz do repositório).
//
// Regra: ao trocar o sufixo do Render, edite SOMENTE a constante
// API_URL abaixo — todos os consumidores (api.tsx, live tracking,
// helpers) passam a usar o novo valor automaticamente.
//
// Precedência de resolução:
//   1. EXPO_PUBLIC_API_URL  — variável de ambiente (override p/ staging/dev)
//   2. API_URL              — constante abaixo (produção, valor padrão)
//
// Nota: com a centralização, o valor é embutido no bundle no build.
// Para apontar um build de staging, defina EXPO_PUBLIC_API_URL no perfil
// correspondente do eas.json (ou edite API_URL aqui) antes de rodar o build.
// =============================================================

/** URL base da API do monolito (produção). */
export const API_URL = process.env.EXPO_PUBLIC_API_URL || "https://api.fuudelivery.com";

/**
 * Retorna a URL base da API.
 * Usa `EXPO_PUBLIC_API_URL` se definida (dev/staging), senão a constante.
 */
export const getApiUrl = (): string =>
  process.env.EXPO_PUBLIC_API_URL || API_URL;

/**
 * Retorna a URL do WebSocket (Fuu Pulse / live tracking),
 * derivada da API_URL (https → wss).
 */
export const getWsUrl = (): string =>
  process.env.EXPO_PUBLIC_WS_URL || getApiUrl().replace(/^http/, "ws");

export default { API_URL, getApiUrl, getWsUrl };
