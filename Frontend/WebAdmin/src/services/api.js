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

export default api;
