import React, { createContext, useState, useContext, useEffect } from "react";
import api from "../services/api";

const TOKEN_KEY = "fuu_admin_token";
const REFRESH_KEY = "fuu_admin_refresh_token";

const AuthContext = createContext();

const decodePayload = (token) => {
  const payload = JSON.parse(atob(token.split(".")[1]));
  return payload;
};

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem(TOKEN_KEY);
    if (token) {
      try {
        setUser(decodePayload(token));
      } catch (e) {
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(REFRESH_KEY);
      }
    }
    setLoading(false);
  }, []);

  const login = async (email, password) => {
    const response = await api.post("/users/login", { email, password });
    const token = response.data.token;
    localStorage.setItem(TOKEN_KEY, token);
    // Access token dura 15 min; o refresh (30 dias) mantém a sessão viva.
    if (response.data.refresh_token) {
      localStorage.setItem(REFRESH_KEY, response.data.refresh_token);
    }
    setUser(decodePayload(token));
    return token;
  };

  const logout = async () => {
    // Revoga o refresh token no servidor antes de limpar local.
    const refreshToken = localStorage.getItem(REFRESH_KEY);
    if (refreshToken) {
      try {
        await api.post("/auth/logout", { refresh_token: refreshToken });
      } catch (e) {
        // segue com logout local mesmo se o servidor falhar
      }
    }
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(REFRESH_KEY);
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => useContext(AuthContext);
