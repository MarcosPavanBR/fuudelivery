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

// Garante que o cookie csrf_token existe antes de mutações.
async function ensureCsrfToken() {
  let token = getCookie("csrf_token")
  if (!token) {
    const res = await api.get("/csrf-token", { withCredentials: true })
    token = res.data?.csrf_token
  }
  return token
}

api.interceptors.request.use(
  async (config) => {
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

const processQueue = (error, token) => {
  failedQueue.forEach(({ resolve, reject }) => {
    if (error) reject(error);
    else resolve(token);
  });
  failedQueue = [];
};

const logoutAndRedirect = async () => {
  try {
    await api.post("/auth/session/logout", {}, { withCredentials: true })
  } catch {
    // ignore
  }
  window.location.href = "/"
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // ── Refresh single-flight ─────────────────────────────
    // Se 401 e não é request de refresh/login e token existe
    const isAuthUrl = originalRequest.url?.includes("/auth/") || false
    const isRefreshUrl = originalRequest.url?.includes("/auth/session/refresh") || false

    if (error.response?.status === 401 && !isAuthUrl && !isRefreshUrl && !isRefreshing) {
      isRefreshing = true

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
