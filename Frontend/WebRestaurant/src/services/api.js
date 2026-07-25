import axios from "axios";
import Strings from "../constants/Strings";

const API_BASE_URL =
  process.env.REACT_APP_API_URL ||
  process.env.API_URL ||
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

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem(Strings.token_jwt);
      window.location.href = "/login";
    }
    return Promise.reject(error);
  }
);

export default api;
