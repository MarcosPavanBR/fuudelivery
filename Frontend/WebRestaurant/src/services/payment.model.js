import api from "./api";

const PAYMENT_API =
  process.env.REACT_APP_PAYMENT_API_URL ||
  "https://fuudelivery-api-8y6l.onrender.com/api/v1";

// === WALLET (para o restaurante) ===

export const getWallet = async (establishmentId) => {
  try {
    const response = await api.get(
      `${PAYMENT_API}/wallets/${establishmentId}`
    );
    return response.data;
  } catch (error) {
    console.error("Erro ao buscar carteira:", error);
    throw error;
  }
};

export const getExtract = async (establishmentId, limit = 20, cursor = "") => {
  try {
    const params = new URLSearchParams();
    if (limit) params.append("limit", limit);
    if (cursor) params.append("cursor", cursor);

    const response = await api.get(
      `${PAYMENT_API}/wallets/${establishmentId}/transactions?${params.toString()}`
    );
    return response.data;
  } catch (error) {
    console.error("Erro ao buscar extrato:", error);
    throw error;
  }
};

export const requestWithdraw = async (establishmentId, data) => {
  try {
    const response = await api.post(
      `${PAYMENT_API}/wallets/${establishmentId}/debit`,
      {
        amount: data.amount,
        description: `Saque solicitado via ${data.method} - ${data.destination}`,
        reference_id: `withdraw_${Date.now()}`,
      }
    );
    return response.data;
  } catch (error) {
    console.error("Erro ao solicitar saque:", error);
    throw error;
  }
};

// === PAYMENTS (somente leitura para o restaurante) ===

export const getMyPayments = async (establishmentId, limit = 20, cursor = "") => {
  try {
    const params = new URLSearchParams();
    params.append("limit", limit);
    if (cursor) params.append("cursor", cursor);

    const response = await api.get(
      `${PAYMENT_API}/payments?${params.toString()}`
    );
    return response.data;
  } catch (error) {
    console.error("Erro ao buscar pagamentos:", error);
    throw error;
  }
};

export const getPaymentDetail = async (paymentId) => {
  try {
    const response = await api.get(`${PAYMENT_API}/payments/${paymentId}`);
    return response.data;
  } catch (error) {
    console.error("Erro ao buscar detalhe do pagamento:", error);
    throw error;
  }
};

// === CHARGEBACKS (restaurante vê e envia evidências) ===

export const getMyChargebacks = async (limit = 20, cursor = "") => {
  try {
    const params = new URLSearchParams();
    params.append("limit", limit);
    if (cursor) params.append("cursor", cursor);

    const response = await api.get(
      `${PAYMENT_API}/chargebacks?${params.toString()}`
    );
    return response.data;
  } catch (error) {
    console.error("Erro ao buscar disputas:", error);
    throw error;
  }
};

export const addEvidence = async (disputeId, formData) => {
  try {
    const response = await api.post(
      `${PAYMENT_API}/chargebacks/${disputeId}/evidence`,
      formData,
      { headers: { "Content-Type": "multipart/form-data" } }
    );
    return response.data;
  } catch (error) {
    console.error("Erro ao enviar evidência:", error);
    throw error;
  }
};

// === HEALTH ===

export const getPaymentHealth = async () => {
  try {
    const response = await api.get(`${PAYMENT_API}/health`);
    return response.data;
  } catch (error) {
    return { status: "offline", mongo: false };
  }
};

export default {
  getWallet,
  getExtract,
  requestWithdraw,
  getMyPayments,
  getPaymentDetail,
  getMyChargebacks,
  addEvidence,
  getPaymentHealth,
};
