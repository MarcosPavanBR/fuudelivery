import api from "./api";

// O monolito expõe a carteira do restaurante em /wallet/establishment/*.
// O estabelecimento autenticado vem do JWT — não é preciso passar o ID.
// NOTA: usar paths relativos (ex: /wallet/establishment/balance) pois o
// axios instance já tem o baseURL configurado — NÃO concatenar com
// getApiBaseUrl() (causava URLs duplicadas como
// https://api...https://api.../wallet/...).

// === WALLET (papel restaurante — monolito) ===

// Saldo + totais do ledger: { available, pending, blocked, total_earned,
// total_withdrawn, last_updated }
export const getWallet = async () => {
  const response = await api.get("/wallet/establishment/balance");
  return response.data;
};

// Extrato paginado por cursor: { data, next_cursor, has_more }
// Cada item: { id, type (CREDIT|DEBIT|WITHDRAWAL), description, created_at,
// amount, balance, payment_ref }
export const getExtract = async (limit = 20, cursor = "") => {
  const params = new URLSearchParams();
  if (limit) params.append("limit", limit);
  if (cursor) params.append("cursor", cursor);

  const response = await api.get(
    `/wallet/establishment/transactions?${params.toString()}`
  );
  return response.data;
};

// newIdempotencyKey gera um identificador único por TENTATIVA de saque.
//
// crypto.randomUUID não existe em contexto inseguro (http:// que não seja
// localhost) nem em WebView antiga, e um saque não pode falhar por causa
// disso — o fallback é aleatório o bastante para o que a chave precisa ser:
// única entre as tentativas deste usuário.
export function newIdempotencyKey() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `wd-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

// Saque: { amount, destination, method }
//
// A Idempotency-Key é o que separa "o dono clicou duas vezes no mesmo saque"
// de "o dono quer sacar de novo o mesmo valor". Sem ela o backend cai num
// fallback derivado de (estabelecimento, valor, destino, janela de 1 min) que
// não consegue provar serem a MESMA requisição lógica e, por isso, responde
// 409 — correto da parte dele (falso sucesso num saque é o pior resultado
// possível), mas quem tomava esse 409 era justamente o duplo clique legítimo:
// o saque tinha funcionado e a tela mostrava erro.
//
// Com a chave explícita, o replay volta a ser 200 idempotente pelo caminho que
// confere valor e destino, e o 409 fica só para o caso genuinamente ambíguo.
// A chave é gerada por tentativa (não por sessão): um segundo saque
// deliberado, do mesmo valor para o mesmo destino, tem chave nova e passa.
export const requestWithdraw = async (data) => {
  const idempotencyKey = data.idempotencyKey || newIdempotencyKey();
  const response = await api.post(
    "/wallet/establishment/withdraw",
    {
      amount: data.amount,
      destination: data.destination,
      method: data.method,
      // Também no corpo: o backend aceita as duas formas, e há proxy que
      // descarta header desconhecido.
      idempotency_key: idempotencyKey,
    },
    { headers: { "Idempotency-Key": idempotencyKey } }
  );
  return response.data;
};

// === HEALTH (monolito) ===

export const getPaymentHealth = async () => {
  try {
    const response = await api.get("/health");
    return response.data;
  } catch (error) {
    return { status: "offline" };
  }
};

export default {
  getWallet,
  getExtract,
  requestWithdraw,
  getPaymentHealth,
};
