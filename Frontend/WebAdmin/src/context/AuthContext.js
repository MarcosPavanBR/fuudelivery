import React, { createContext, useState, useContext, useEffect } from "react";
import api from "../services/api";

const AuthContext = createContext();

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  // ── Restaurar sessão após page refresh ──────────────────────
  useEffect(() => {
    let cancelled = false;
    async function restoreSession() {
      try {
        const { data } = await api.get("/auth/session/me", { withCredentials: true });
        if (!cancelled && data?.user) {
          setUser(data.user);
        }
      } catch {
        setUser(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    restoreSession();
    return () => { cancelled = true; };
  }, []);

  // ── Login via sessão HttpOnly ──────────────────────────────
  const login = async (email, password) => {
    const response = await api.post(
      "/auth/session",
      { email, password },
      { withCredentials: true }
    );
    const userData = response.data?.user;
    if (userData) {
      setUser(userData);
    }
    return response.data;
  };

  // ── Logout ─────────────────────────────────────────────────
  const logout = async () => {
    try {
      await api.post("/auth/session/logout", {}, { withCredentials: true });
    } catch (e) {
      // segue com logout local
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
