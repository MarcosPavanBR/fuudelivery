import axios from "axios";
import { getApiUrl } from "@/config/api";
import { getToken, clearToken } from "@/config/tokenStorage";

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

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      clearToken();         // limpa o storage (sempre)
      onUnauthorized?.();   // logout do contexto → nav.tsx mostra o login
    }
    return Promise.reject(error);
  }
);

export default api;
