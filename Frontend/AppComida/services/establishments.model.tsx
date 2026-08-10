import api from "./api";
import { fetchWithCache, CACHE_TTL, CACHE_KEYS } from "@/config/cache";

/**
 * Lista de estabelecimentos com cache local (TTL 5 min) — evita
 * chamada de rede a cada abertura da home. Se a rede falhar, serve
 * o último dado cacheado (modo offline).
 */
async function getEstablishment() {
  return fetchWithCache(
    CACHE_KEYS.ESTABLISHMENTS,
    async () => {
      const { data } = await api.get("/api/auth/establishments");
      return data;
    },
    CACHE_TTL.ESTABLISHMENTS,
    []
  );
}

export default { getEstablishment };
