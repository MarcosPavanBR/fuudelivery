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
  withCredentials: true,
  headers: { "Content-Type": "application/json" },
});

export default paymentApi;
