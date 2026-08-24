import axios from "axios";

// API principal (monolito). Sobrescrita por REACT_APP_API_URL ou VITE_API_URL
// no build (Vite expõe via import.meta.env). Lista canônica em references/URLS.md.
const API_BASE_URL =
  import.meta.env.REACT_APP_API_URL ||
  import.meta.env.VITE_API_URL ||
  "https://fuudelivery-api-8y6l.onrender.com";

const TOKEN_KEY = "fuu_admin_token";
const REFRESH_KEY = "fuu_admin_refresh_token";

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
    const token = localStorage.getItem(TOKEN_KEY);
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// ── Refresh single-flight (access token dura 15 min) ──────────────────
let isRefreshing = false;
let failedQueue = [];

const processQueue = (error, token) => {
  failedQueue.forEach(({ resolve, reject }) => {
    if (error) reject(error);
    else resolve(token);
  });
  failedQueue = [];
};

const clearAuthAndRedirect = () => {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
  window.location.href = "/login";
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    const isAuthUrl =
      originalRequest.url.includes("auth/refresh") ||
      originalRequest.url.includes("users/login");

    // ── Refresh single-flight ────────────────────────────────
    if (
      error.response?.status === 401 &&
      !originalRequest._retry &&
      !isAuthUrl
    ) {
      const storedRefreshToken = localStorage.getItem(REFRESH_KEY);

      if (!storedRefreshToken) {
        clearAuthAndRedirect();
        return Promise.reject(error);
      }

      if (isRefreshing) {
        // Renovação em andamento — enfileira até o novo token chegar.
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

        localStorage.setItem(TOKEN_KEY, data.token);
        if (data.refresh_token) {
          localStorage.setItem(REFRESH_KEY, data.refresh_token);
        }

        processQueue(null, data.token);

        originalRequest.headers.Authorization = `Bearer ${data.token}`;
        return api(originalRequest);
      } catch (refreshError) {
        processQueue(refreshError, null);
        clearAuthAndRedirect();
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    // ── Cold-start retry (Render free tier dorme) ────────────
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
