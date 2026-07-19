import axios from "axios";
import Strings from "../constants/Strings";

const api = axios.create({
  baseURL: process.env.REACT_APP_API_URL || "http://localhost:3000",
  timeout: 15000,
  headers: {
    "Content-Type": "application/json",
  },
});

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
