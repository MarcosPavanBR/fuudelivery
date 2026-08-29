import axios from "axios";

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

export const getApiBaseUrl = () => API_BASE_URL;

function getCookie(name) {
  const value = `; ${document.cookie}`
  const parts = value.split(`; ${name}=`)
  if (parts.length === 2) return parts.pop().split(";").shift()
  return null
}

api.interceptors.request.use(
  (config) => {
    const withCredentials = config.withCredentials !== false
    if (withCredentials) {
      config.withCredentials = true
    }
    const csrfToken = getCookie("csrf_token")
    if (csrfToken && ["post", "put", "delete", "patch"].includes((config.method || "get").toLowerCase())) {
      config.headers["X-CSRF-Token"] = csrfToken
    }
    return config
  },
  (error) => Promise.reject(error)
);

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
    localStorage.removeItem("fuu_admin_token");
    localStorage.removeItem("fuu_admin_refresh_token");
    await api.post("/auth/session/logout", {}, { withCredentials: true })
  } catch {
    // ignore
  }
  // Usa React Router navigate via path变化 para não recarregar toda a página.
  // Fallback para window.location apenas se o SPA já estiver no /login
  if (window.location.pathname !== "/login") {
    window.location.href = "/login";
  }
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Ignora erros de rede (sem response) — não tenta refresh
    if (!error.response) return Promise.reject(error);

    const isAuthUrl = originalRequest.url?.includes("/auth/") || false
    const isRefreshUrl = originalRequest.url?.includes("/auth/session/refresh") || false
    // Não tenta refresh em requisições que já falharam com refresh
    const isRetry = originalRequest._retry === true

    if (error.response.status === 401 && !isAuthUrl && !isRefreshUrl && !isRefreshing && !isRetry) {
      isRefreshing = true
      originalRequest._retry = true

      try {
        const refreshToken = localStorage.getItem("fuu_admin_refresh_token")
        if (!refreshToken) {
          throw new Error("no refresh token")
        }

        const refreshResponse = await api.post("/auth/refresh", {
          refresh_token: refreshToken,
        })
        const { token, refresh_token } = refreshResponse.data

        if (token) {
          localStorage.setItem("fuu_admin_token", token)
        }
        if (refresh_token) {
          localStorage.setItem("fuu_admin_refresh_token", refresh_token)
        }

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

export default api;
