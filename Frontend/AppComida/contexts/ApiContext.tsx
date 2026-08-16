// ApiContext.tsx
import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useRef,
  ReactNode,
} from "react";
import { Alert } from "react-native";
import LoadingPage from "@/components/LoadingPage";
import { jwtDecode } from "jwt-decode";
import {
  setToken,
  getToken,
  clearToken,
  setPhoneOverride,
  getPhoneOverride,
  clearPhoneOverride,
} from "@/config/tokenStorage";
import { setOnUnauthorized } from "@/services/api";

interface UserData {
  id?: number;
  name?: string;
  email?: string;
  phone?: string;
  role?: string;
  establishment_id?: number;
}

interface ApiContextProps {
  login(token: string, userData?: UserData): Promise<void>;
  updateUser(partial: Partial<UserData>): void;
  isLogged: boolean;
  getUserData(): UserData | null;
  isLoading: boolean;
  setIsLoading(status: boolean): void;
  logout(): void;
  token: string | null;
}

const ApiContext = createContext<ApiContextProps | undefined>(undefined);

interface ApiProviderProps {
  children: ReactNode;
}

function decodeJWT(token: string): UserData | null {
  try {
    const payload: any = jwtDecode(token);
    return {
      id: payload.id,
      name: payload.name,
      email: payload.email,
      phone: payload.phone,
      role: payload.role,
      establishment_id: payload.establishment_id,
    };
  } catch (e) {
    console.error("Failed to decode JWT:", e);
    return null;
  }
}

export const ApiProvider: React.FC<ApiProviderProps> = ({ children }) => {
  const [isLogged, setIsLogged] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [token, setToken] = useState<string | null>(null);
  const [userData, setUserData] = useState<UserData | null>(null);

  useEffect(() => {
    const loadUserFromStorage = async () => {
      try {
        const storedToken = await getToken();
        if (storedToken) {
          const decoded = decodeJWT(storedToken);
          if (decoded) {
            const override = await getPhoneOverride();
            setToken(storedToken);
            setUserData(override ? { ...decoded, phone: override } : decoded);
            setIsLogged(true);
          } else {
            await clearToken();
          }
        }
      } catch (error) {
        console.error("Error loading user from storage:", error);
      }
      setIsLoading(false);
    };

    loadUserFromStorage();
  }, []);

  const logout = async () => {
    await clearToken();
    await clearPhoneOverride();
    setToken(null);
    setUserData(null);
    setIsLogged(false);
  };

  // Atualiza dados do usuário em memória e persiste o telefone editado
  // (o JWT em si só é reemitido num novo login).
  const updateUser = (partial: Partial<UserData>) => {
    setUserData((prev) => {
      const next = { ...prev, ...partial };
      if (partial.phone !== undefined) {
        setPhoneOverride(partial.phone).catch(() => {});
      }
      return next;
    });
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

  const getUserData = (): UserData | null => {
    return userData;
  };

  const login = async (tokenValue: string, extraData?: UserData) => {
    if (tokenValue) {
      try {
        const decoded = decodeJWT(tokenValue);
        if (!decoded) {
          console.error("Invalid JWT token - decode failed");
          Alert.alert("Erro", "Não foi possível validar o token. Tente novamente.");
          return;
        }

        const override = await getPhoneOverride();
        const mergedData = {
          ...decoded,
          ...extraData,
          ...(override ? { phone: override } : {}),
        };
        await setToken(tokenValue);
        setToken(tokenValue);
        setUserData(mergedData);
        setIsLogged(true);

        // Registra push token em background
        if (mergedData.id) {
          import("@/helpers/pushNotifications").then(({ registerForPushNotifications }) => {
            registerForPushNotifications(mergedData.id!, "customer");
          });
        }
      } catch (error) {
        console.error("Error storing token:", error);
        Alert.alert("Erro", "Falha ao salvar sessão. Tente novamente.");
      }
    }
  };

  return (
    <ApiContext.Provider
      value={{ logout, login, updateUser, isLogged, isLoading, setIsLoading, getUserData, token }}
    >
      {!isLoading ? children : <LoadingPage />}
    </ApiContext.Provider>
  );
};

export const useApi = (): ApiContextProps => {
  const context = useContext(ApiContext);
  if (!context) {
    throw new Error("useApi must be used within an ApiProvider");
  }
  return context;
};
