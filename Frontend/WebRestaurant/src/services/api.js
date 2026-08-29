import axios from "axios";

// URL da API de produção (fallback). Sobrescrita por REACT_APP_API_URL
// ou VITE_API_URL no build (Vite expõe via import.meta.env).
const API_BASE_URL =
  import.meta.env.REACT_APP_API_URL ||
  import.meta.env.VITE_API_URL ||
  "https://fuudelivery-api-8y6l.onrender.com";

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 15000,
  withCredentials: true, // Envia cookies HttpOnly em todas as requisições
  headers: {
    "Content-Type": "application/json",
  },
});

// Export for use by other services that need the base URL
export const getApiBaseUrl = () => API_BASE_URL;

// Request a short-lived WebSocket ticket (60s) using the current JWT cookie.
// The backend reads the JWT from the HttpOnly cookie and returns a ticket.
export async function requestWsTicket() {
  const response = await api.post("/auth/ws-ticket", {});
  return response.data?.ticket;
}

// Cookie helper for CSRF token
function getCookie(name) {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop().split(";").shift();
  return null;
}

let csrfTokenCache = null;
let csrfFetchPromise = null; // single-flight: evita múltiplas chamadas simultâneas

// Garante que temos um CSRF token válido antes de mutações (POST/PUT/DELETE).
// Usa axios PURO (sem interceptor) para evitar recursão.
function ensureCsrfToken() {
  if (csrfTokenCache) return Promise.resolve(csrfTokenCache);
  if (csrfFetchPromise) return csrfFetchPromise;
  csrfFetchPromise = axios
    .get(`${API_BASE_URL}/csrf-token`, { withCredentials: true })
    .then((res) => {
      const token = res.data?.csrf_token || null;
      if (token) csrfTokenCache = token;
      return token;
    })
    .catch(() => null)
    .finally(() => {
      csrfFetchPromise = null;
    });
  return csrfFetchPromise;
}

// Request interceptor: adiciona CSRF token em mutações
api.interceptors.request.use(
  async (config) => {
    const url = config.url || "";
    const isAuthOrCsrf =
      url.includes("/auth/") || url.includes("/csrf-token");
    if (isAuthOrCsrf) {
      return config;
    }

    const method = (config.method || "get").toLowerCase();
    if (["post", "put", "delete", "patch"].includes(method)) {
      const csrfToken = await ensureCsrfToken();
      if (csrfToken) {
        config.headers["X-CSRF-Token"] = csrfToken;
      } else {
        csrfTokenCache = null;
        const retryToken = await ensureCsrfToken();
        if (retryToken) {
          config.headers["X-CSRF-Token"] = retryToken;
        }
      }
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Controle de refresh em andamento para evitar chamadas duplicadas
let isRefreshing = false;
let failedQueue = [];
let hasRedirected = false;

const processQueue = (error, token) => {
  failedQueue.forEach(({ resolve, reject }) => {
    if (error) reject(error);
    else resolve(token);
  });
  failedQueue = [];
};

const logoutAndRedirect = async () => {
  if (hasRedirected) return;
  hasRedirected = true;
  try {
    await api.post("/auth/session/logout", {});
  } catch {
    // ignore
  }
  if (window.location.pathname !== "/") {
    window.location.href = "/";
  }
};

// Response interceptor: CSRF retry + session refresh
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    if (!error.response) return Promise.reject(error);

    // ── CSRF 403 retry ────────────────────────────────────
    if (error.response.status === 403 && !originalRequest._csrfRetry) {
      const errMsg = error.response.data?.error || "";
      if (errMsg.toLowerCase().includes("csrf")) {
        originalRequest._csrfRetry = true;
        csrfTokenCache = null;
        const newToken = await ensureCsrfToken();
        if (newToken) {
          originalRequest.headers["X-CSRF-Token"] = newToken;
          return api(originalRequest);
        }
      }
    }

    // ── Session refresh single-flight ──────────────────────
    const isAuthUrl = originalRequest.url?.includes("/auth/") || false;
    const isRefreshUrl =
      originalRequest.url?.includes("/auth/session/refresh") || false;
    const isRetry = originalRequest._retry === true;

    if (
      error.response.status === 401 &&
      !isAuthUrl &&
      !isRefreshUrl &&
      !isRefreshing &&
      !isRetry
    ) {
      isRefreshing = true;
      originalRequest._retry = true;

      try {
        // Session refresh: backend lê refresh_token do cookie HttpOnly
        const refreshResponse = await api.post("/auth/session/refresh", {});
        processQueue(null, refreshResponse.data);
        hasRedirected = false;

        // Retry the original request (cookies are now refreshed)
        return api(originalRequest);
      } catch (refreshError) {
        processQueue(refreshError, null);
        logoutAndRedirect();
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(error);
  }
);

export default api;
