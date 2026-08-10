// =============================================================
// cache.ts — Cache local com TTL (MMKV via config/storage.ts)
// =============================================================
// Cacheia dados de rede (cardápio, estabelecimentos) para reduzir
// chamadas de API ao abrir o app. Usa o MESMO storage síncrono do
// resto do app (config/storage.ts) — MMKV em nativo, localStorage
// na web.
//
// Formato do envelope: { data, savedAt, ttlMs } — o TTL viaja com o
// dado, então leituras expiram sozinhas (lazy expiry: remove ao ler
// algo vencido).
//
// ⚠️ NUNCA guarde token/segredos aqui — use config/tokenStorage.ts
// (SecureStore). Este cache é para dados públicos (cardápio, lista
// de restaurantes).
//
// Política de stale: durante uma falha de rede, o dado vencido é
// servido INDEFINIDAMENTE (last-known-good) — a tela mostra o último
// cardápio/lista conhecidos em vez de quebrar. Quando a rede volta, a
// próxima leitura refaz a busca e sobrescreve. Para forçar atualização
// imediata, use removeCached() + refetch (ex.: pull-to-refresh).
// =============================================================

import storage from "./storage";

/** Prefixo de todas as chaves de cache (evita colisão com outros usos). */
const CACHE_PREFIX = "fuu:cache:";

/** Envelope persistido no storage. */
interface CacheEnvelope<T> {
  data: T;
  savedAt: number;
  ttlMs: number;
}

/**
 * TTLs padrão por tipo de dado.
 * - ESTABLISHMENTS: lista de restaurantes (estado aberto/fechado muda).
 * - MENU: cardápio (categorias + produtos) — muda raramente.
 */
export const CACHE_TTL = {
  ESTABLISHMENTS: 5 * 60 * 1000, // 5 minutos
  MENU: 15 * 60 * 1000, // 15 minutos
} as const;

/** Chaves canônicas do cache. */
export const CACHE_KEYS = {
  ESTABLISHMENTS: "establishments",
  /** Cardápio: categorias de um estabelecimento. */
  menuCategories: (id: string | number) => `menu:categories:${id}`,
  /** Cardápio: produtos de um estabelecimento. */
  menuProducts: (id: string | number) => `menu:products:${id}`,
} as const;

function cacheKey(key: string): string {
  return CACHE_PREFIX + key;
}

/**
 * Lê o cache. Retorna null se ausente, corrompido ou VENCIDO
 * (neste caso remove a chave — lazy expiry).
 */
export function getCached<T>(key: string): T | null {
  const raw = storage.getItem(cacheKey(key));
  if (!raw) return null;
  try {
    const env = JSON.parse(raw) as CacheEnvelope<T>;
    if (Date.now() - env.savedAt > env.ttlMs) {
      storage.removeItem(cacheKey(key)); // expira e limpa
      return null;
    }
    return env.data;
  } catch {
    storage.removeItem(cacheKey(key)); // corrompido: limpa
    return null;
  }
}

/** Grava um valor no cache com TTL. Síncrono. */
export function setCached<T>(key: string, data: T, ttlMs: number): void {
  const env: CacheEnvelope<T> = { data, savedAt: Date.now(), ttlMs };
  storage.setItem(cacheKey(key), JSON.stringify(env));
}

/** Remove uma chave do cache (invalidação manual). */
export function removeCached(key: string): void {
  storage.removeItem(cacheKey(key));
}

/**
 * Padrão principal: retorna o cache se fresco; senão chama o fetcher
 * e armazena o resultado. Se a rede FALHAR, serve o dado stale se
 * existir (modo offline) e só então cai no fallback.
 */
export async function fetchWithCache<T>(
  key: string,
  fetcher: () => Promise<T>,
  ttlMs: number,
  fallback: T
): Promise<T> {
  const k = cacheKey(key);
  const raw = storage.getItem(k);

  // 1) Cache fresco? Retorna sem tocar a rede.
  // (Lê o envelope cru SEM expirar — o stale precisa sobreviver para
  //  o modo offline do passo 3. `getCached` não é usado aqui justamente
  //  porque ele REMOVE a chave vencida.)
  if (raw) {
    try {
      const env = JSON.parse(raw) as CacheEnvelope<T>;
      if (Date.now() - env.savedAt <= env.ttlMs) {
        return env.data;
      }
    } catch {
      // envelope corrompido — segue para a rede
    }
  }

  // 2) Sem cache fresco: busca na rede.
  try {
    const data = await fetcher();
    setCached(key, data, ttlMs);
    return data;
  } catch (e) {
    // 3) Rede falhou: serve o stale (mesmo vencido) para não quebrar
    //    a tela em modo offline. A chave vencida é mantida (será
    //    sobrescrita na próxima busca com sucesso).
    if (raw) {
      try {
        return (JSON.parse(raw) as CacheEnvelope<T>).data;
      } catch {
        // Envelope corrompido (sem stale aproveitável): remove e cai
        // no fallback — mesmo hygiene do getCached.
        storage.removeItem(k);
      }
    }
    return fallback;
  }
}

export default { getCached, setCached, removeCached, fetchWithCache, CACHE_TTL, CACHE_KEYS };
