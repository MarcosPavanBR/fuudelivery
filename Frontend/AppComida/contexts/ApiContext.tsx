// ApiContext.tsx
import Strings from "@/constants/Strings";
import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  ReactNode,
} from "react";
import AsyncStorage from "@react-native-async-storage/async-storage";
import * as SecureStore from "expo-secure-store";
import LoadingPage from "@/components/LoadingPage";

interface UserData {
  id: number;
  name: string;
  phone: string;
}

interface ApiContextProps {
  login(token: string, userData: UserData): Promise<void>;
  isLogged: boolean;
  getUserData(): UserData | null;
  isLoading: boolean;
  setIsLoading(status: boolean): void;
  logout(): void;
}

const ApiContext = createContext<ApiContextProps | undefined>(undefined);

interface ApiProviderProps {
  children: ReactNode;
}

export const ApiProvider: React.FC<ApiProviderProps> = ({ children }) => {
  const [isLogged, setIsLogged] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [userData, setUserData] = useState<UserData | null>(null);

  useEffect(() => {
    const loadUserFromStorage = async () => {
      try {
        // Tenta carregar o JWT do SecureStore (onde a api.tsx já procura)
        const token = await SecureStore.getItemAsync(Strings.token_jwt);
        if (token) {
          setIsLogged(true);

          // Carrega dados do usuario do AsyncStorage
          const storedUserData = await AsyncStorage.getItem("USER_DATA");
          if (storedUserData) {
            setUserData(JSON.parse(storedUserData));
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
    await AsyncStorage.removeItem("USER_DATA");
    setUserData(null);
    setIsLogged(false);
  };

  function getUserData(): UserData | null {
    return userData;
  }

  const login = async (token: string, userDataParam: UserData) => {
    if (token) {
      try {
        // Armazena o JWT no SecureStore (onde a api.tsx já procura)
        await SecureStore.setItemAsync(Strings.token_jwt, token);
        // Armazena dados do usuario no AsyncStorage para acesso rapido
        await AsyncStorage.setItem("USER_DATA", JSON.stringify(userDataParam));
        setUserData(userDataParam);
        setIsLogged(true);
      } catch (error) {
        console.error("Error storing credentials:", error);
      }
    }
  };

  return (
    <ApiContext.Provider
      value={{ logout, login, isLogged, isLoading, setIsLoading, getUserData }}
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
