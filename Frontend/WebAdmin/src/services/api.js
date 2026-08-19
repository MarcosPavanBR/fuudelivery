import axios from "axios";

// API principal (monolito). Sobrescrita por REACT_APP_API_URL ou VITE_API_URL
// no build (Vite expõe via import.meta.env). Lista canônica em references/URLS.md.
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
    const token = localStorage.getItem("fuu_admin_token");
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem("fuu_admin_token");
      window.location.href = "/login";
    }
    return Promise.reject(error);
  }
);

export default api;