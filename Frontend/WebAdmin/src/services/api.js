import axios from "axios";

const api = axios.create({
  baseURL: "https://fuudelivery-api-8y6l.onrender.com",
  timeout: 15000,
  headers: {
    "Content-Type": "application/json",
  },
});

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