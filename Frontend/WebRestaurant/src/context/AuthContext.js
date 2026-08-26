import React, { createContext, useState, useContext, useEffect, useCallback } from "react";
import api, { getApiBaseUrl, requestWsTicket } from "../services/api";

import Strings from "../constants/Strings";
import { jwtDecode } from "jwt-decode";

import useWebSocket from "react-use-websocket";

const AuthContext = createContext();

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [openEstablishment, setOpenEstablishment] = useState(false);
  const [fmode, setFMode] = useState(false);
  const [theme] = useState("light");

  // Máximo de mensagens WebSocket em memória para evitar memory leak.
  // Acima disso, as mais antigas são descartadas.
  const MAX_SOCKET_MESSAGES = 100;
  const [socketMessage, setSocketMessage] = useState([]);

  // Tema fixo light (decisão 2026-08: dark mode removido — estava parcial
  // e páginas hardcodavam bg-white). toggleTheme mantido como no-op para
  // não quebrar consumidores.
  const toggleTheme = () => {};

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", "light");
    localStorage.setItem("theme", "light");
  }, []);

  // Só conecta WebSocket após login válido
  const getWsBaseUrl = () => {
    // Fonte única: services/api.js (lê VITE_API_URL do .env).
    const apiUrl = api.defaults.baseURL || getApiBaseUrl();
    return apiUrl.replace(/^http/, "ws").replace(/\/+$/, "");
  };

  const [wsUrl, setWsUrl] = useState(null);
  useEffect(() => {
    let cancelled = false;
    async function connectWs() {
      if (!user?.sub) return;
      try {
        const ticket = await requestWsTicket();
        if (!cancelled) {
          setWsUrl(getWsBaseUrl() + "/ws/" + user.sub + "?ticket=" + ticket);
        }
      } catch {
        // Fallback para JWT na query string (deprecated) se o ticket falhar
        if (!cancelled) {
          setWsUrl(
            getWsBaseUrl() +
              "/ws/" +
              user.sub +
              "?token=" +
              localStorage.getItem(Strings.token_jwt)
          );
        }
      }
    }
    connectWs();
    return () => { cancelled = true; };
  }, [user]);
  const { sendJsonMessage, lastMessage } = useWebSocket(wsUrl, {
    enabled: !!wsUrl && !!user?.sub,
    reconnectInterval: 1000,
    retryOnError: true,
    reconnectAttempts: 5,
    onReconnectStop: () => {
      setFMode(true);
    },
    onError: () => {
      setFMode(true);
    },
    onOpen: () => {
      setFMode(false);
    },
  });

  const getUser = useCallback(() => {
    const storedToken = localStorage.getItem(Strings.token_jwt);

    if (storedToken) {
      const decodedToken = jwtDecode(storedToken);

      return decodedToken;
    }
    return null;
  }, []);

  const sendSocketMessage = (type, data) => {
    sendJsonMessage({
      type,
      data,
    });
  };

  useEffect(() => {
    try {
      const decodedToken = getUser();
      setUser(decodedToken);
      if (decodedToken?.establishment) {
        sendSocketMessage("connect", {
          id: decodedToken.establishment.id,
          name: decodedToken.establishment.name,
        });
      }
    } catch (e) {
      console.error(e);
    }

    setLoading(false);
  }, []);

  useEffect(() => {
    if (lastMessage) {
      setSocketMessage(prev => {
        const next = [...prev, lastMessage];
        // Mantém apenas as últimas N mensagens para evitar memory leak
        return next.length > MAX_SOCKET_MESSAGES
          ? next.slice(next.length - MAX_SOCKET_MESSAGES)
          : next;
      });
    }
  }, [lastMessage]);

  const login = async (email, password) => {
    try {
      const response = await api.post("users/login", {
        email,
        password,
      });
      const { token, refresh_token } = response.data;
      const decoded = jwtDecode(token);
      setUser(decoded);

      localStorage.setItem(Strings.token_jwt, token);
      if (refresh_token) {
        localStorage.setItem(Strings.refresh_token, refresh_token);
      }
    } catch (error) {
      console.error("Erro ao fazer login:", error);
      throw error;
    }
  };

  // Renova o access token usando o refresh token.
  // Chamado automaticamente quando o token está perto de expirar.
  const refreshAccessToken = useCallback(async () => {
    const storedRefreshToken = localStorage.getItem(Strings.refresh_token);
    if (!storedRefreshToken) return false;

    try {
      const response = await api.post("auth/refresh", {
        refresh_token: storedRefreshToken,
      });
      const { token, refresh_token } = response.data;
      const decoded = jwtDecode(token);
      setUser(decoded);
      localStorage.setItem(Strings.token_jwt, token);
      if (refresh_token) {
        localStorage.setItem(Strings.refresh_token, refresh_token);
      }
      return true;
    } catch (error) {
      console.error("Failed to refresh token:", error);
      // Refresh token inválido — fazer logout
      localStorage.removeItem(Strings.token_jwt);
      localStorage.removeItem(Strings.refresh_token);
      setUser(null);
      return false;
    }
  }, []);

  // Agenda o refresh automático do token antes de expirar.
  // Roda sempre que o user muda (login) ou o token é renovado.
  useEffect(() => {
    if (!user) return;

    const token = localStorage.getItem(Strings.token_jwt);
    if (!token) return;

    try {
      const decoded = jwtDecode(token);
      if (!decoded.exp) return;

      const nowSec = Date.now() / 1000;
      const expiresInMs = (decoded.exp - nowSec) * 1000;

      // Renova 2 minutos antes de expirar
      const refreshMs = Math.max(expiresInMs - 2 * 60 * 1000, 10 * 1000);

      const timer = setTimeout(() => {
        refreshAccessToken();
      }, refreshMs);

      return () => clearTimeout(timer);
    } catch (e) {
      // token inválido — ignora
    }
  }, [user, refreshAccessToken]);

  // Função para fazer logout
  const logout = useCallback(async () => {
    const storedRefreshToken = localStorage.getItem(Strings.refresh_token);
    if (storedRefreshToken) {
      try {
        await api.post("auth/logout", { refresh_token: storedRefreshToken });
      } catch (e) {
        // ignora erro de logout no servidor
      }
    }
    localStorage.removeItem(Strings.token_jwt);
    localStorage.removeItem(Strings.refresh_token);
    setUser(null);
  }, []);

  const refreshOpen = async () => {
    // O establishment do dono vem do claim aninhado do JWT
    const id = getUser()?.establishment?.id;
    if (!id) return;

    try {
      const { data } = await api.get(
        "/establishments/" + id
      );

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

// Crie um hook personalizado para acessar o contexto de autenticação
export const useAuth = () => useContext(AuthContext);

