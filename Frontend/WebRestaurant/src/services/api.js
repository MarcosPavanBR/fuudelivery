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

// Request a short-lived WebSocket ticket (60s) using the current JWT.
// The ticket avoids sending JWT in the WS query string.
export async function requestWsTicket() {
  const token = localStorage.getItem(Strings.token_jwt)
  if (!token) {
    throw new Error("No JWT token available")
  }

  const response = await api.post("/auth/ws-ticket", {}, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  return response.data?.ticket || response.data?.expires_in
}

// Cookie helper for session-based auth
function getCookie(name) {
  const value = `; ${document.cookie}`
  const parts = value.split(`; ${name}=`)
  if (parts.length === 2) return parts.pop().split(";").shift()
  return null
}

let csrfTokenCache = null;

// Garante que o cookie csrf_token existe antes de mutações.
// Usa cache em memória para evitar chamadas recursivas ao /csrf-token
// dentro do interceptor de request (que chamaria api.get → outro interceptor → loop).
function ensureCsrfToken() {
  const cached = getCookie("csrf_token") || csrfTokenCache;
  if (cached) return Promise.resolve(cached);
  // Busca uma vez e cacheia — se falhar, retorna null (mutações seguirão sem CSRF header)
  return api.get("/csrf-token", { withCredentials: true }).then(res => {
    csrfTokenCache = res.data?.csrf_token || null;
    return csrfTokenCache;
  }).catch(() => null);
}

api.interceptors.request.use(
  async (config) => {
    // Não aplica CSRF a requests de auth (login, refresh, csrf-token) — eles não precisam
    const url = config.url || "";
    const isAuthOrCsrf = url.includes("/auth/") || url.includes("/csrf-token") || url.includes("/csrf_token");
    if (isAuthOrCsrf) {
      config.withCredentials = true;
      return config;
    }

    const withCredentials = config.withCredentials !== false
    if (withCredentials) {
      config.withCredentials = true
    }
    const method = (config.method || "get").toLowerCase()
    if (["post", "put", "delete", "patch"].includes(method)) {
      const csrfToken = await ensureCsrfToken()
      if (csrfToken) {
        config.headers["X-CSRF-Token"] = csrfToken
      }
    }
    return config
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
  // Guard: só redireciona uma vez para evitar loop infinito de reloads
  if (hasRedirected) return;
  hasRedirected = true;
  try {
    localStorage.removeItem(Strings.token_jwt);
    localStorage.removeItem(Strings.refresh_token);
    await api.post("/auth/session/logout", {}, { withCredentials: true })
  } catch {
    // ignore
  }
  // Redireciona para / (landing page) sem reload forçado quando possível
  if (window.location.pathname !== "/") {
    window.location.href = "/";
  }
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Ignora erros de rede (sem response)
    if (!error.response) return Promise.reject(error);

    // ── Refresh single-flight ─────────────────────────────
    // Se 401 e não é request de refresh/login e não é retry
    const isAuthUrl = originalRequest.url?.includes("/auth/") || false
    const isRefreshUrl = originalRequest.url?.includes("/auth/session/refresh") || false
    const isRetry = originalRequest._retry === true

    if (error.response.status === 401 && !isAuthUrl && !isRefreshUrl && !isRefreshing && !isRetry) {
      isRefreshing = true
      originalRequest._retry = true

      try {
        const refreshResponse = await api.post("/auth/session/refresh", {}, { withCredentials: true })
        const { token } = refreshResponse.data

        processQueue(null, token)

        originalRequest.headers.Authorization = `Bearer ${token}`
        return api(originalRequest)
      } catch (refreshError) {
        processQueue(refreshError, null)
        logoutAndRedirect()
        return Promise.reject(refreshError)
      } finally {
        isRefreshing = false
      }
    }

    return Promise.reject(error)
  }
);

// API genérica com interceptors já aplicados
export default api;
