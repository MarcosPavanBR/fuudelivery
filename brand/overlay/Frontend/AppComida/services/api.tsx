import axios from "axios";
import * as SecureStore from "expo-secure-store";
import Strings from "@/constants/Strings";
import helpers from "@/helpers/helpers";

const api = axios.create({
  baseURL: process.env.EXPO_PUBLIC_API_URL || helpers.getApiUrl(),
  timeout: 15000,
  headers: {
    "Content-Type": "application/json",
  },
});

api.interceptors.request.use(
  async (config) => {
    const toe = await SecureStore.getItemAsync(Strings.token_jwt);
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
      SecureStore.deleteItemAsync(Strings.token_jwt);
    }
    return Promise.reject(error);
  }
);

export default api;
