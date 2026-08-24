import axios from "axios";
import { getApiUrl } from "@/config/api";
import {
  getToken,
  setToken,
  clearToken,
  getRefreshToken,
  setRefreshToken,
  clearRefreshToken,
} from "@/config/tokenStorage";

const api = axios.create({
  baseURL: getApiUrl(),
  timeout: 15000,
  headers: {
    "Content-Type": "application/json",
  },
});

// ─── Sessão expirada (401) ─────────────────────────────────────
// O ApiContext registra aqui o logout() via setOnUnauthorized —
// evita dependência circular (api.tsx não importa o contexto).
let onUnauthorized: (() => void) | null = null;
export function setOnUnauthorized(cb: (() => void) | null): void {
  onUnauthorized = cb;
}

api.interceptors.request.use(
  async (config) => {
    const toe = await getToken();
    if (toe) {
      config.headers.Authorization = `Bearer ${toe}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Captura o refresh_token emitido no login/register e persiste no
// SecureStore. Assim nenhuma tela precisa saber da existência dele.
api.interceptors.response.use(async (response) => {
  const url = response.config.url || "";
  const refreshToken = response.data?.refresh_token;
  if (
    refreshToken &&
    (url.includes("users/login") || url.includes("users/register"))
  ) {
    await setRefreshToken(refreshToken);
  }
  return response;
});

// ── Refresh single-flight ──────────────────────────────────────
let isRefreshing = false;
let failedQueue: Array<{
  resolve: (token: string) => void;
  reject: (err: unknown) => void;
}> = [];

const processQueue = (error: unknown, token?: string) => {
  failedQueue.forEach(({ resolve, reject }) => {
    if (error) reject(error);
    else resolve(token!);
  });
  failedQueue = [];
};

const forceLogout = async () => {
  await clearToken();
  await clearRefreshToken();
  onUnauthorized?.(); // logout do contexto → nav.tsx mostra o login
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config || {};

    // ── 401 → renova com refresh token (uma vez por request) ──
    const url: string = originalRequest.url || "";
    const isAuthUrl =
      url.includes("auth/refresh") ||
      url.includes("users/login") ||
      url.includes("users/register");

    if (error.response?.status === 401 && !originalRequest._retry && !isAuthUrl) {
      const storedRefresh = await getRefreshToken();

      if (!storedRefresh) {
        await forceLogout();
        return Promise.reject(error);
      }

      if (isRefreshing) {
        // Renovação em andamento — enfileira até o novo token chegar.
        return new Promise<string>((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        }).then((token) => {
          originalRequest.headers.Authorization = `Bearer ${token}`;
          return api(originalRequest);
        });
      }

      originalRequest._retry = true;
      isRefreshing = true;

      try {
        const { data } = await axios.post(`${getApiUrl()}/auth/refresh`, {
          refresh_token: storedRefresh,
        });

        await setToken(data.token);
        if (data.refresh_token) {
          await setRefreshToken(data.refresh_token);
        }

        processQueue(null, data.token);

        originalRequest.headers.Authorization = `Bearer ${data.token}`;
        return api(originalRequest);
      } catch (refreshError) {
        processQueue(refreshError);
        await forceLogout();
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    // ── Cold-start retry (Render free tier dorme) ──────────────
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
