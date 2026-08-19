import axios from "axios";
import Strings from "../constants/Strings";
import { jwtDecode } from "jwt-decode";

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

// Verifica se o token está perto de expirar (menos de 60s)
const isTokenExpiringSoon = () => {
  const token = localStorage.getItem(Strings.token_jwt);
  if (!token) return false;
  try {
    const decoded = jwtDecode(token);
    if (!decoded.exp) return false;
    const nowSec = Date.now() / 1000;
    return decoded.exp - nowSec < 60;
  } catch {
    return false;
  }
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Se 401 e não é request de refresh/login e token existe
    if (
      error.response?.status === 401 &&
      !originalRequest._retry &&
      !originalRequest.url.includes("auth/refresh") &&
      !originalRequest.url.includes("users/login") &&
      !originalRequest.url.includes("users/register")
    ) {
      const storedRefreshToken = localStorage.getItem(Strings.refresh_token);

      // Se tem refresh token, tenta renovar
      if (storedRefreshToken && !isRefreshing) {
        if (isTokenExpiringSoon()) {
          originalRequest._retry = true;
        }

        if (originalRequest._retry) {
          if (isRefreshing) {
            // Já está renovando — enfileira esta requisição
            return new Promise((resolve, reject) => {
              failedQueue.push({ resolve, reject });
            }).then((token) => {
              originalRequest.headers.Authorization = `Bearer ${token}`;
              return api(originalRequest);
            });
          }

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
      }

      // Sem refresh token ou refresh falhou — logout
      logoutAndRedirect();
    }

    return Promise.reject(error);
  }
);

export default api;
