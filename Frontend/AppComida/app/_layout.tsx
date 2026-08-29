import FontAwesome from "@expo/vector-icons/FontAwesome";
import { DefaultTheme } from "@react-navigation/native"; import { ThemeProvider } from "@react-navigation/core";
import { useFonts } from "expo-font";

import * as SplashScreen from "expo-splash-screen";
import "react-native-reanimated";

import { useEffect, useState } from "react";

import { useColorScheme } from "@/components/useColorScheme";

import "../global.css";
import { ApiProvider } from "@/contexts/ApiContext";
import NavStack from "./nav";
import { ApiCartProvider } from "@/contexts/ApiCartContext";
import { migrateLegacyData } from "@/config/legacyMigration";
import { ErrorBoundary } from "@/components/ErrorBoundary";

export { ErrorBoundary } from "expo-router";

export const unstable_settings = {
  initialRouteName: "(tabs)",
};

SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const [loaded, error] = useFonts({
    SpaceMono: require("../assets/fonts/SpaceMono-Regular.ttf"),
    ...FontAwesome.font,
  });

  // Migração one-time de dados legados (AsyncStorage → MMKV/SecureStore).
  // Espera terminar ANTES de montar os providers, para que ApiContext e
  // ApiCartContext leiam o token/localização já migrados (sem race condition).
  const [migrationDone, setMigrationDone] = useState(false);

  useEffect(() => {
    migrateLegacyData().finally(() => setMigrationDone(true));
  }, []);

  useEffect(() => {
    if (error) throw error;
  }, [error]);

  useEffect(() => {
    if (loaded) {
      SplashScreen.hideAsync();
    }
  }, [loaded]);

  if (!loaded || !migrationDone) {
    return null;
  }

  return (
    <ErrorBoundary screenName="AppComida">
      <RootLayoutNav />
    </ErrorBoundary>
  );
}

function RootLayoutNav() {
  const colorScheme = useColorScheme();
  return (
    <ApiProvider>
      <ApiCartProvider>
        <ThemeProvider value={DefaultTheme}>
          <NavStack />
        </ThemeProvider>
      </ApiCartProvider>
    </ApiProvider>
  );
}
