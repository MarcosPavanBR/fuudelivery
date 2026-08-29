import React, { createContext, useState, useContext, useEffect, useCallback } from "react";
import api, { getApiBaseUrl, requestWsTicket } from "../services/api";

import useWebSocket from "react-use-websocket";

const AuthContext = createContext();

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [openEstablishment, setOpenEstablishment] = useState(false);
  const [fmode, setFMode] = useState(false);
  const [theme] = useState("light");

  // Máximo de mensagens WebSocket em memória para evitar memory leak.
  const MAX_SOCKET_MESSAGES = 100;
  const [socketMessage, setSocketMessage] = useState([]);

  // Tema fixo light
  const toggleTheme = () => {};

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", "light");
    localStorage.setItem("theme", "light");
  }, []);

  // ── Restaurar sessão após page refresh ──────────────────────
  // Cookies HttpOnly não são legíveis via JS, então chamamos
  // GET /auth/session/me que lê o cookie no server e retorna
  // os dados do usuário.
  useEffect(() => {
    let cancelled = false;
    async function restoreSession() {
      try {
        const { data } = await api.get("/auth/session/me", { withCredentials: true });
        if (!cancelled && data?.user) {
          setUser(data.user);
        }
      } catch {
        // Sem cookie válido → usuário não está logado
        setUser(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    restoreSession();
    return () => { cancelled = true; };
  }, []);

  // ── WebSocket ──────────────────────────────────────────────
  const getWsBaseUrl = () => {
    const apiUrl = api.defaults.baseURL || getApiBaseUrl();
    return apiUrl.replace(/^http/, "ws").replace(/\/+$/, "");
  };

  const [wsUrl, setWsUrl] = useState(null);
  useEffect(() => {
    let cancelled = false;
    async function connectWs() {
      if (!user?.id) return;
      try {
        const ticket = await requestWsTicket();
        if (!cancelled) {
          setWsUrl(getWsBaseUrl() + "/ws/" + user.id + "?ticket=" + ticket);
        }
      } catch {
        // Ticket falhou — tenta sem (será bloqueado se não autenticado)
        if (!cancelled) {
          setWsUrl(getWsBaseUrl() + "/ws/" + user.id);
        }
      }
    }
    connectWs();
    return () => { cancelled = true; };
  }, [user]);

  const { sendJsonMessage, lastMessage } = useWebSocket(wsUrl, {
    enabled: !!wsUrl && !!user?.id,
    reconnectInterval: (attempt) => Math.min(3000 * Math.pow(2, attempt), 48000),
    retryOnError: true,
    reconnectAttempts: 8,
    onReconnectStop: () => { setFMode(true); },
    onError: () => { setFMode(true); },
    onOpen: () => { setFMode(false); },
  });

  // getUser retorna os dados do user em state (não mais de localStorage)
  const getUser = useCallback(() => user, [user]);

  const sendSocketMessage = (type, data) => {
    sendJsonMessage({ type, data });
  };

  useEffect(() => {
    if (user?.id) {
      sendSocketMessage("connect", {
        id: user.establishment_id || user.id,
        name: user.name,
      });
    }
  }, [user]);

  useEffect(() => {
    if (lastMessage) {
      setSocketMessage(prev => {
        const next = [...prev, lastMessage];
        return next.length > MAX_SOCKET_MESSAGES
          ? next.slice(next.length - MAX_SOCKET_MESSAGES)
          : next;
      });
    }
  }, [lastMessage]);

  // ── Login via sessão HttpOnly ──────────────────────────────
  // POST /auth/session → backend seta cookies HttpOnly
  // e retorna dados do usuário no body.
  const login = async (email, password) => {
    try {
      const response = await api.post("/auth/session", { email, password }, {
        withCredentials: true,
      });
      const userData = response.data?.user;
      if (userData) {
        setUser(userData);
      }
    } catch (error) {
      console.error("Erro ao fazer login:", error);
      throw error;
    }
  };

  // ── Refresh de sessão ──────────────────────────────────────
  // POST /auth/session/refresh → lê refresh_token cookie no server,
  // rotaciona, seta novos cookies, retorna novos dados do user.
  const refreshSession = useCallback(async () => {
    try {
      const response = await api.post("/auth/session/refresh", {}, {
        withCredentials: true,
      });
      const userData = response.data?.user;
      if (userData) {
        setUser(userData);
      }
      return true;
    } catch (error) {
      console.error("Failed to refresh session:", error);
      setUser(null);
      return false;
    }
  }, []);

  // ── Logout ─────────────────────────────────────────────────
  const logout = useCallback(async () => {
    try {
      await api.post("/auth/session/logout", {}, { withCredentials: true });
    } catch (e) {
      // ignora erro de logout no servidor
    }
    setUser(null);
    setSocketMessage([]);
    setWsUrl(null);
  }, []);

  // ── Refresh automático do estado (estilo keepalive) ─────────
  // Session refresh automático: a cada 10 minutos, chama
  // /auth/session/refresh para manter a sessão viva.
  useEffect(() => {
    if (!user) return;

    const refreshMs = 10 * 60 * 1000; // 10 minutos
    const timer = setInterval(() => {
      refreshSession();
    }, refreshMs);

    return () => clearInterval(timer);
  }, [user, refreshSession]);

  // ── Refresh open status ────────────────────────────────────
  const refreshOpen = async () => {
    const id = user?.establishment_id;
    if (!id) return;

    try {
      const { data } = await api.get("/establishments/" + id);
      setOpenEstablishment(data?.open_data ?? false);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    refreshOpen();
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        login,
        logout,
        getUser,
        sendSocketMessage,
        socketMessage,
        openEstablishment,
        setOpenEstablishment,
        refreshOpen,
        fmode,
        theme,
        toggleTheme,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => useContext(AuthContext);
