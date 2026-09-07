import React, { createContext, useState, useContext, useEffect } from "react";
import api from "../services/api";

const AuthContext = createContext();

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  // Sessão vive num cookie HttpOnly (setado por /auth/session) — não dá
  // pra ler o token no JS, então a única forma de saber quem está logado
  // num reload de página é perguntar ao backend.
  useEffect(() => {
    let cancelled = false;
    api
      .get("/auth/session")
      .then(({ data }) => {
        if (!cancelled) setUser(data.user);
      })
      .catch(() => {
        if (!cancelled) setUser(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const login = async (email, password) => {
    const response = await api.post("/auth/session", { email, password });
    setUser(response.data.user);
    return response.data.user;
  };

  const logout = async () => {
    try {
      await api.post("/auth/session/logout");
    } catch (e) {
      // segue com logout local mesmo se o servidor falhar
    }
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => useContext(AuthContext);
