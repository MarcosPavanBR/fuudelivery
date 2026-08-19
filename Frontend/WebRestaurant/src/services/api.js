import axios from "axios";
import Strings from "../constants/Strings";

// URL da API de produção (fallback). Sobrescrita por REACT_APP_API_URL
// ou VITE_API_URL no build (Vite expõe via import.meta.env).
// Lista canônica de URLs em references/URLS.md.
const API_BASE_URL =
  import.meta.env.VITE_API_URL ||
  "https://api.fuudelivery.com";

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
    const toe = sessionStorage.getItem(Strings.token_jwt);
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
      sessionStorage.removeItem(Strings.token_jwt);
      // HashRouter: redirect to base URL so HashRouter shows login
      window.location.href = window.location.pathname;
    }
    return Promise.reject(error);
  }
);

export default api;
