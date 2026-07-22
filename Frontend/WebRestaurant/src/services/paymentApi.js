import axios from 'axios';
import Strings from '../constants/Strings';

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

export const PaymentService = {
  getPayments: (params) => paymentApi.get('/api/payments', { params }),
  getPayment: (id) => paymentApi.get(`/api/payments/${id}`),
  createPayment: (data) => paymentApi.post('/api/payments', data),
  approvePayment: (id) => paymentApi.post(`/api/payments/${id}/approve`),
  rejectPayment: (id, reason) => paymentApi.post(`/api/payments/${id}/reject`, { reason }),
  getStats: () => paymentApi.get('/api/payments/stats'),

  getChargebacks: (params) => paymentApi.get('/api/chargebacks', { params }),
  getChargeback: (id) => paymentApi.get(`/api/chargebacks/${id}`),
  approveChargeback: (id) => paymentApi.post(`/api/chargebacks/${id}/approve`),
  rejectChargeback: (id, reason) => paymentApi.post(`/api/chargebacks/${id}/reject`, { reason }),
  addEvidence: (id, data) => paymentApi.post(`/api/chargebacks/${id}/evidence`, data),

  getWallet: (userId) => paymentApi.get(`/api/wallets/${userId}`),
  getWalletTransactions: (userId, limit) => paymentApi.get(`/api/wallets/${userId}/transactions`, { params: { limit } }),
  creditWallet: (userId, data) => paymentApi.post(`/api/wallets/${userId}/credit`, data),
  debitWallet: (userId, data) => paymentApi.post(`/api/wallets/${userId}/debit`, data),
};

export default paymentApi;
