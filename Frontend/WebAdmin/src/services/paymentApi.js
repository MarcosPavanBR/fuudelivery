import axios from "axios";

// Payment routes now live in the monolith (fuudelivery-api).
// The isolated fuudelivery-payment service was removed.
const PAYMENT_BASE_URL =
  import.meta.env.VITE_API_URL ||
  "https://api.fuudelivery.com";

const paymentApi = axios.create({
  baseURL: PAYMENT_BASE_URL,
  timeout: 15000,
  headers: { "Content-Type": "application/json" },
});

paymentApi.interceptors.request.use(
  (config) => {
    const token = sessionStorage.getItem("fuu_admin_token");
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

paymentApi.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      sessionStorage.removeItem("fuu_admin_token");
      window.location.href = "/login";
    }
    return Promise.reject(error);
  }
);

export default paymentApi;
