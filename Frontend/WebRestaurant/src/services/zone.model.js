import api from "./api";

// Zona padrão quando o estabelecimento REALMENTE não tem zona atribuída —
// espelha exatamente a resposta do backend (zone_handler.go, zone == nil).
// NÃO é o retorno de erro: erro tem forma própria (abaixo).
export const DEFAULT_ZONE_FEE = {
  has_zone: false,
  current_platform_pct: 5.0,
  current_establishment_pct: 85.0,
  at_target: true,
};

// Resposta de ERRO: a página sabe que a tela está desatualizada — diferente
// de "sem zona", que é um estado de negócio legítimo (taxa padrão 5%/85%).
// Mascaram os dois com o mesmo fallback, o restaurante via uma comissão
// errada em silêncio (ex.: 403 de token expirado virava "5% de taxa").
export const ZONE_FEE_ERROR = {
  error: true,
  code: "zone_fee_unavailable",
};

/**
 * Busca a taxa da zona do estabelecimento logado.
 *
 * Contrato de retorno:
 *  - objeto com has_zone true/false — resposta legítima do backend;
 *  - ZONE_FEE_ERROR (error: true) — 403/404/500/network. O consumidor DEVE
 *    tratar este caso (aviso, retry, esconder painel) em vez de renderizar
 *    números como se fossem reais.
 *
 * Distinguir os dois casos importa: "sem zona" é dado (taxa padrão válida);
 * "erro" é estado quebrado (auth expirada, backend fora) e mentir aqui
 * esconde incidente do único usuário que perceberia.
 */
export async function getMyZoneFee() {
  try {
    const { data } = await api.get("/establishments/me/zone");
    // Defesa: backend responder 2xx com corpo inesperado (proxy quebrado,
    // HTML de manutenção) também é erro, não "sem zona".
    if (typeof data?.has_zone !== "boolean") {
      console.error("[zone] resposta inesperada de /establishments/me/zone", data);
      return { ...ZONE_FEE_ERROR };
    }
    return data;
  } catch (e) {
    console.error("[zone] falha ao consultar taxa da zona:", e?.response?.status || e);
    return { ...ZONE_FEE_ERROR };
  }
}

export default {
  getMyZoneFee,
  DEFAULT_ZONE_FEE,
  ZONE_FEE_ERROR,
};
