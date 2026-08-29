import axios from "axios";

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

export const getApiBaseUrl = () => API_BASE_URL;

function getCookie(name) {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop().split(";").shift();
  return null;
}

let csrfTokenCache = null;
let csrfFetchPromise = null;

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
  if (window.location.pathname !== "/login") {
    window.location.href = "/login";
  }
};

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
