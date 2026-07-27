// ApiContext.tsx
import Strings from "@/constants/Strings";
import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  ReactNode,
} from "react";
import * as SecureStore from "expo-secure-store";
import LoadingPage from "@/components/LoadingPage";

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
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")));
    return {
      id: payload.id,
      name: payload.name,
      email: payload.email,
      phone: payload.phone,
      role: payload.role,
      establishment_id: payload.establishment_id,
    };
  } catch {
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
        const storedToken = await SecureStore.getItemAsync(Strings.token_jwt);
        if (storedToken) {
          const decoded = decodeJWT(storedToken);
          if (decoded) {
            setToken(storedToken);
            setUserData(decoded);
            setIsLogged(true);
          } else {
            await SecureStore.deleteItemAsync(Strings.token_jwt);
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
    await SecureStore.deleteItemAsync(Strings.token_jwt);
    setToken(null);
    setUserData(null);
    setIsLogged(false);
  };

  const getUserData = (): UserData | null => {
    return userData;
  };

  const login = async (tokenValue: string, extraData?: UserData) => {
    if (tokenValue) {
      try {
        const decoded = decodeJWT(tokenValue);
        if (!decoded) {
          console.error("Invalid JWT token");
          return;
        }

        const mergedData = { ...decoded, ...extraData };
        await SecureStore.setItemAsync(Strings.token_jwt, tokenValue);
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
      }
    }
  };

  return (
    <ApiContext.Provider
      value={{ logout, login, isLogged, isLoading, setIsLoading, getUserData, token }}
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
