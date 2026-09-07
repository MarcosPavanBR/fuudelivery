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

// Garante que o cookie csrf_token existe antes de mutações — sem isso o
// double-submit do backend não tem nada pra comparar com o header e a
// proteção contra CSRF fica inerte (ver cmd/fuudelivery/csrf.go: sem
// cookie, a checagem deixa passar por assumir que não é sessão de browser).
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

    const isAuthUrl = originalRequest.url?.includes("/auth/") || false
    const isRefreshUrl = originalRequest.url?.includes("/auth/session/refresh") || false

    if (error.response?.status === 401 && !isAuthUrl && !isRefreshUrl && !isRefreshing) {
      isRefreshing = true

      try {
        // Refresh token vive só no cookie HttpOnly — o backend lê e devolve
        // um access_token novo, também via cookie. Não há nada pra guardar
        // aqui nem Authorization pra setar: a próxima chamada já sai com o
        // cookie atualizado.
        await api.post("/auth/session/refresh", {}, { withCredentials: true })
        processQueue(null, null)
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
