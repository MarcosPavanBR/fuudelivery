/**
 * paymentApi.js
 * Cliente HTTP (axios) para comunicacao com o Payment Service.
 * Configura baseURL, autenticacao JWT via interceptor e tratamento de erros.
 *
 * Endpoints disponiveis:
 * - Pagamentos: GET/POST /api/payments, approve, reject, stats
 * - Chargebacks: GET/POST /api/chargebacks, approve, reject, evidence
 * - Wallets: GET/POST /api/wallets/:id, transactions, credit, debit
 */
import axios from 'axios';
import Strings from '../constants/Strings';

/** URL base do Payment Service (configuravel via env) */
const PAYMENT_API_URL = process.env.REACT_APP_PAYMENT_API_URL || 'http://localhost:8084';

const paymentApi = axios.create({
  baseURL: PAYMENT_API_URL,
  timeout: 10000,
});

paymentApi.interceptors.request.use((config) => {
  const token = localStorage.getItem(Strings.token_jwt);
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

paymentApi.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem(Strings.token_jwt);
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

/**
 * Servico de pagamentos — todos os endpoints da API.
 * Cada metodo retorna uma Promise com a resposta do Payment Service.
 */
export const PaymentService = {
  /** Listar pagamentos com filtros (status, risk_level, etc) */
  getPayments: (params) => paymentApi.get('/api/payments', { params }),
  /** Buscar pagamento por ID */
  getPayment: (id) => paymentApi.get(`/api/payments/${id}`),
  /** Criar novo pagamento */
  createPayment: (data) => paymentApi.post('/api/payments', data),
  /** Aprovar pagamento manualmente */
  approvePayment: (id) => paymentApi.post(`/api/payments/${id}/approve`),
  /** Rejeitar pagamento com motivo */
  rejectPayment: (id, reason) => paymentApi.post(`/api/payments/${id}/reject`, { reason }),
  /** Estatisticas gerais de pagamentos */
  getStats: () => paymentApi.get('/api/payments/stats'),

  /** Listar estornos/disputas */
  getChargebacks: (params) => paymentApi.get('/api/chargebacks', { params }),
  /** Buscar estorno por ID */
  getChargeback: (id) => paymentApi.get(`/api/chargebacks/${id}`),
  /** Aprovar estorno */
  approveChargeback: (id) => paymentApi.post(`/api/chargebacks/${id}/approve`),
  /** Rejeitar estorno com motivo */
  rejectChargeback: (id, reason) => paymentApi.post(`/api/chargebacks/${id}/reject`, { reason }),
  /** Adicionar evidencia a um estorno */
  addEvidence: (id, data) => paymentApi.post(`/api/chargebacks/${id}/evidence`, data),

  /** Consultar saldo da carteira */
  getWallet: (userId) => paymentApi.get(`/api/wallets/${userId}`),
  /** Historico de transacoes da carteira */
  getWalletTransactions: (userId, limit) => paymentApi.get(`/api/wallets/${userId}/transactions`, { params: { limit } }),
  /** Creditar valor na carteira */
  creditWallet: (userId, data) => paymentApi.post(`/api/wallets/${userId}/credit`, data),
  /** Debitar valor da carteira */
  debitWallet: (userId, data) => paymentApi.post(`/api/wallets/${userId}/debit`, data),
};

export default paymentApi;
