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
  const response = await api.get("/wallets/establishment/balance");
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
    `/wallets/establishment/transactions?${params.toString()}`
  );
  return response.data;
};

// Saque: { amount, destination, method }
export const requestWithdraw = async (data) => {
  const response = await api.post("/wallets/establishment/withdraw", {
    amount: data.amount,
    destination: data.destination,
    method: data.method,
  });
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
