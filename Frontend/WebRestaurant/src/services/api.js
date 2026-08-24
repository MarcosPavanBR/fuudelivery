import axios from "axios";
import Strings from "../constants/Strings";

// URL da API de produção (fallback). Sobrescrita por REACT_APP_API_URL
// ou VITE_API_URL no build (Vite expõe via import.meta.env).
// Lista canônica de URLs em references/URLS.md.
const API_BASE_URL =
  import.meta.env.REACT_APP_API_URL ||
  import.meta.env.VITE_API_URL ||
  "https://fuudelivery-api-8y6l.onrender.com";

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 15000,
  headers: {
    "Content-Type": "application/json",
  },
});

// Export for use by other services that need the base URL
export const getApiBaseUrl = () => API_BASE_URL;

api.interceptors.request.use(
  (config) => {
    const toe = localStorage.getItem(Strings.token_jwt);
    if (toe) {
      config.headers.Authorization = `Bearer ${toe}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Controle de refresh em andamento para evitar chamadas duplicadas
let isRefreshing = false;
let failedQueue = [];

const processQueue = (error, token) => {
  failedQueue.forEach(({ resolve, reject }) => {
    if (error) reject(error);
    else resolve(token);
  });
  failedQueue = [];
};

const logoutAndRedirect = () => {
  localStorage.removeItem(Strings.token_jwt);
  localStorage.removeItem(Strings.refresh_token);
  // BrowserRouter: redirect to root so React Router shows login
  window.location.href = "/";
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // ── Refresh single-flight ─────────────────────────────
    // Se 401 e não é request de refresh/login e token existe
    const isAuthUrl =
      originalRequest.url.includes("auth/refresh") ||
      originalRequest.url.includes("users/login") ||
      originalRequest.url.includes("users/register");

    if (
      error.response?.status === 401 &&
      !originalRequest._retry &&
      !isAuthUrl
    ) {
      const storedRefreshToken = localStorage.getItem(Strings.refresh_token);

      // Sem refresh token — logout direto.
      if (!storedRefreshToken) {
        logoutAndRedirect();
        return Promise.reject(error);
      }

      if (isRefreshing) {
        // Já está renovando — enfileira esta requisição até o novo token.
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        }).then((token) => {
          originalRequest.headers.Authorization = `Bearer ${token}`;
          return api(originalRequest);
        });
      }

      originalRequest._retry = true;
      isRefreshing = true;

      try {
        const { data } = await axios.post(
          `${API_BASE_URL}/auth/refresh`,
          { refresh_token: storedRefreshToken },
          { headers: { "Content-Type": "application/json" } }
        );

        const newToken = data.token;
        const newRefreshToken = data.refresh_token;

        localStorage.setItem(Strings.token_jwt, newToken);
        if (newRefreshToken) {
          localStorage.setItem(Strings.refresh_token, newRefreshToken);
        }

        processQueue(null, newToken);

        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        return api(originalRequest);
      } catch (refreshError) {
        processQueue(refreshError, null);
        logoutAndRedirect();
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    // ── Cold-start retry (Render free tier dorme) ─────────
    // 503 / rede / timeout em GET ganha UMA tentativa extra após backoff.
    const status = error.response?.status;
    const isNetworkLike =
      !error.response ||
      error.code === "ECONNABORTED" ||
      error.code === "ERR_NETWORK";
    if (
      !originalRequest._coldRetry &&
      originalRequest.method === "get" &&
      (status === 503 || isNetworkLike)
    ) {
      originalRequest._coldRetry = true;
      await new Promise((r) => setTimeout(r, 1500));
      return api(originalRequest);
    }

    return Promise.reject(error);
  }
);

export default api;
