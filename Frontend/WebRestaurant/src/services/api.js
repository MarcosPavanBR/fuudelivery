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
let csrfFetchPromise = null; // single-flight: evita múltiplas chamadas simultâneas

// Garante que temos um CSRF token válido antes de mutações (POST/PUT/DELETE).
// Usa axios PURO (sem interceptor) para evitar recursão.
// O cookie SameSite=None pode não ser enviado cross-origin, então sempre
// buscamos o token via response body e cacheamos em memória.
function ensureCsrfToken() {
  // 1. Retorna do cache se existe
  if (csrfTokenCache) return Promise.resolve(csrfTokenCache);
  // 2. Single-flight: se já está buscando, reusa a promise
  if (csrfFetchPromise) return csrfFetchPromise;
  // 3. Busca com axios puro (sem interceptor) + withCredentials para setar cookie
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
      } else {
        // CSRF fetch falhou — tenta uma vez mais limpando o cache
        csrfTokenCache = null;
        const retryToken = await ensureCsrfToken()
        if (retryToken) {
          config.headers["X-CSRF-Token"] = retryToken
        }
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

    // ── CSRF 403 retry ────────────────────────────────────
    // Se 403 CSRF, limpa cache e tenta com token novo
    if (error.response.status === 403 && !originalRequest._csrfRetry) {
      const errMsg = error.response.data?.error || ""
      if (errMsg.toLowerCase().includes("csrf")) {
        originalRequest._csrfRetry = true
        csrfTokenCache = null // limpa cache para buscar novo token
        const newToken = await ensureCsrfToken()
        if (newToken) {
          originalRequest.headers["X-CSRF-Token"] = newToken
          return api(originalRequest)
        }
      }
    }

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
        // Reset guard: refresh funcionou, pode haver outros 401s legítimos depois
        hasRedirected = false

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
