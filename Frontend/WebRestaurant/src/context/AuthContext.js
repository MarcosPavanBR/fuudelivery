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
      if (!user?.id) return;
      try {
        const ticket = await requestWsTicket();
        if (!cancelled) {
          setWsUrl(getWsBaseUrl() + "/ws/" + user.id + "?ticket=" + ticket);
        }
      } catch {
        // Sem fallback de token na query string: o access token vive num
        // cookie HttpOnly e não é mais legível pelo JS. Sem ticket, não há
        // como conectar — a tela segue funcional via polling/REST.
      }
    }
    connectWs();
    return () => { cancelled = true; };
  }, [user]);
  const { sendJsonMessage, lastMessage } = useWebSocket(wsUrl, {
    enabled: !!wsUrl && !!user?.id,
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

  const getUser = useCallback(() => user, [user]);

  const sendSocketMessage = (type, data) => {
    sendJsonMessage({
      type,
      data,
    });
  };

  // Restaura a sessão a partir do cookie HttpOnly no load da página — o
  // access token não é mais legível no cliente, então quem sabe se (e
  // quem) está logado é sempre o backend, via GET /auth/session.
  useEffect(() => {
    let cancelled = false;
    api
      .get("/auth/session")
      .then(({ data }) => {
        if (cancelled) return;
        setUser(data.user);
        if (data.user?.establishment) {
          sendSocketMessage("connect", {
            id: data.user.establishment.id,
            name: data.user.establishment.name,
          });
        }
      })
      .catch(() => {
        if (!cancelled) setUser(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
      const response = await api.post("/auth/session", {
        email,
        password,
      });
      setUser(response.data.user);
    } catch (error) {
      console.error("Erro ao fazer login:", error);
      throw error;
    }
  };

  // Função para fazer logout
  const logout = useCallback(async () => {
    try {
      await api.post("/auth/session/logout");
    } catch (e) {
      // ignora erro de logout no servidor
    }
    setUser(null);
  }, []);

  const refreshOpen = async () => {
    // O establishment do dono vem do próprio objeto de usuário (ver
    // GET/POST /auth/session no backend).
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

  // Roda de novo quando a sessão termina de carregar (a leitura do usuário
  // agora é assíncrona — no primeiro mount o establishment ainda não existe).
  useEffect(() => {
    refreshOpen();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.establishment?.id]);

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
