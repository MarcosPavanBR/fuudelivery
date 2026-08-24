import { createContext, useState, useEffect, useContext, useRef } from "react";

import api, { setOnUnauthorized } from "@/services/api";
import { setToken, getToken, clearToken, getRefreshToken, clearRefreshToken } from "@/config/tokenStorage";
import axios from "axios";
import { getApiUrl } from "@/config/api";
import { jwtDecode } from "jwt-decode";
import { useNavigation } from "expo-router";
import * as React from "react";
import { Alert } from "react-native";

interface User {
  email: string;
  name: string;
  id: number;
  phone?: string;
}

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isLogged: boolean;
  disponivel: boolean;
  setDisponivel: (a: boolean) => void;
  login: (email: string, password: string) => Promise<void>;
  register: (
    email: string,
    password: string,
    name: string,
    phone: string
  ) => Promise<void>;
  logout: () => Promise<void>;
  inWork: boolean;
  setIsLoading: (a: boolean) => void;
  mylocation: boolean;
  setMyLocation: (a: boolean) => void;
  isActiveOrder: () => Promise<void>;
}
const AuthContext = createContext<AuthContextType | undefined>(undefined);

const AuthProvider = ({ children }: any) => {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLogged, setIsLogged] = useState(false);
  const [disponivel, setDisponivel] = useState(false);
  const [mylocation, setMyLocation] = useState<any>(null);

  const [inWork, setInWork] = useState({ status: false, order: null });

  const nav = useNavigation();

  // Normaliza o payload de has-active: a API pode devolver null, [] ou
  // objeto. Antes, `[]` virava status=true e derrubava a HomeDelivery
  // com TypeError ao ler order[0].
  const applyActiveOrder = (data: any) => {
    const has = data != null && (!Array.isArray(data) || data.length > 0);
    setInWork({ status: has, order: data ?? null });
    setDisponivel((prev) => has || prev);
  };

  const isActiveOrder = async () => {
    // Faltava o return: sem usuário, a chamava seguia e batia em
    // /deliveryman/has-active/undefined a cada ciclo de polling.
    if (!user || !user.id) {
      setInWork({ status: false, order: null });
      return;
    }

    try {
      const { data } = await api.get("/deliveryman/has-active/" + user.id);
      applyActiveOrder(data);
    } catch (e) {
      // Silencioso: o polling tenta de novo no próximo ciclo.
    }
  };

  const decodeToken = (token: string): User => {
    return jwtDecode(token) as unknown as User;
  };

  const login = async (email: string, password: string) => {
    const response = await api.post("/delivery-man/login", {
      email,
      password,
    });
    const { token } = response.data;

    await setToken(token);
    setUser(decodeToken(token));
    nav.navigate("index" as never);
    setIsLoading(false);
  };

  const register = async (
    email: string,
    password: string,
    name: string,
    phone: string
  ) => {
    try {
      const response = await api.post("/delivery-man/register", {
        email,
        password,
        name,
        phone,
      });
      const { token } = response.data;

      await setToken(token);
      setUser(decodeToken(token));
      nav.navigate("index" as never);
      setIsLoading(false);
    } catch (error) {
      Alert.alert(
        "",
        "Tivemos um problema ao fazer o cadastro, verifique se o e-mail já está cadastrado e tente novamente."
      );
      throw error;
    }
  };

  const logout = async () => {
    try {
      // Revoga o refresh token no servidor antes de limpar o storage local.
      try {
        const refreshToken = await getRefreshToken();
        if (refreshToken) {
          await axios.post(`${getApiUrl()}/auth/logout`, {
            refresh_token: refreshToken,
          });
        }
      } catch {
        // segue com logout local mesmo se o servidor falhar
      }
      await clearToken();
      await clearRefreshToken();
      setUser(null);
      setInWork({ status: false, order: null });
      setDisponivel(false);
      setIsLoading(false);
    } catch (error) {
      throw error;
    }
  };

  // Registra o logout como callback de sessão expirada (401) —
  // o interceptor de api.tsx chama, e o nav.tsx redireciona ao login.
  const logoutRef = useRef(logout);
  logoutRef.current = logout;

  useEffect(() => {
    setOnUnauthorized(() => {
      logoutRef.current();
    });
    return () => setOnUnauthorized(null);
  }, []);

  const getUser = async (): Promise<User | null> => {
    try {
      const token = await getToken();
      if (!token) return null;
      return decodeToken(token);
    } catch (e) {
      return null;
    }
  };

  const checkAuth = async () => {
    const decodedToken = await getUser();
    setUser(decodedToken);
    setIsLoading(false);
  };

  useEffect(() => {
    checkAuth();
  }, []);

  useEffect(() => {
    setIsLogged(user != null);
    isActiveOrder();
  }, [user]);

  return (
    <AuthContext.Provider
      value={
        {
          user,
          isLoading,
          setIsLoading,
          isLogged,
          login,
          logout,
          inWork,
          disponivel,
          setDisponivel,
          isActiveOrder,
          mylocation,
          setMyLocation,
          register,
        } as any
      }
    >
      {children}
    </AuthContext.Provider>
  );
};

const useAuthApi = (): any => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuthApi must be used within an AuthProvider");
  }
  return context;
};

export { AuthProvider, AuthContext, useAuthApi };
