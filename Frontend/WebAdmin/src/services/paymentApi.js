import axios from "axios";

// Payment routes now live in the monolith (fuudelivery-api).
// The isolated fuudelivery-payment service was removed.
const PAYMENT_BASE_URL =
  import.meta.env.REACT_APP_PAYMENT_API_URL ||
  import.meta.env.VITE_PAYMENT_API_URL ||
  import.meta.env.REACT_APP_API_URL ||
  import.meta.env.VITE_API_URL ||
  "https://fuudelivery-api-8y6l.onrender.com";

const paymentApi = axios.create({
  baseURL: PAYMENT_BASE_URL,
  timeout: 15000,
  headers: { "Content-Type": "application/json" },
});

// Sessão via cookie HttpOnly (mesmo backend de api.js) — sem token pra ler
// ou guardar aqui. Ver services/api.js para o mesmo padrão comentado.
function getCookie(name) {
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return parts.pop().split(";").shift();
  return null;
}

async function ensureCsrfToken() {
  let token = getCookie("csrf_token");
  if (!token) {
    const res = await paymentApi.get("/csrf-token", { withCredentials: true });
    token = res.data?.csrf_token;
  }
  return token;
}

paymentApi.interceptors.request.use(
  async (config) => {
    const withCredentials = config.withCredentials !== false;
    if (withCredentials) {
      config.withCredentials = true;
    }
    const method = (config.method || "get").toLowerCase();
    if (["post", "put", "delete", "patch"].includes(method)) {
      const csrfToken = await ensureCsrfToken();
      if (csrfToken) {
        config.headers["X-CSRF-Token"] = csrfToken;
      }
    }
    return config;
  },
  (error) => Promise.reject(error)
);

paymentApi.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      window.location.href = "/login";
    }
    return Promise.reject(error);
  }
);

export default paymentApi;
