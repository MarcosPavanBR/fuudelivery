import FontAwesome from "@expo/vector-icons/FontAwesome";
// NativeWind: estilos globais Tailwind (ver tailwind.config.js e global.css)
import "../global.css";
import { DarkTheme, DefaultTheme, ThemeProvider } from "expo-router/react-navigation";
import { useFonts } from "expo-font";
import { Stack } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { useEffect } from "react";

import { useColorScheme } from "@/components/useColorScheme";
import Texts from "@/constants/Texts";
import Colors from "@/constants/Colors";
import { AuthProvider } from "@/contexts/AuthContext";
import { migrateLegacyData } from "@/config/legacyMigration";
import StackNav from "./nav";
import { ErrorBoundary as AppErrorBoundary } from "@/components/ErrorBoundary";
import "react-native-reanimated";
export {
  // Catch any errors thrown by the Layout component.
  ErrorBoundary,
import * as React from "react";

export const unstable_settings = {
  // Ensure that reloading on `/modal` keeps a back button present.
  initialRouteName: "(tabs)",
};

// Prevent the splash screen from auto-hiding before asset loading is complete.
SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const [loaded, error] = useFonts({
    SpaceMono: require("../assets/fonts/SpaceMono-Regular.ttf"),
    ...FontAwesome.font,
  });

  // Migração one-time de dados legados (AsyncStorage → SecureStore).
  // Espera terminar ANTES de montar o AuthProvider, para que ele leia
  // o token JWT já migrado (sem race condition).
  const [migrationDone, setMigrationDone] = React.useState(false);

  React.useEffect(() => {
    migrateLegacyData().finally(() => setMigrationDone(true));
  }, []);

  // Expo Router uses Error Boundaries to catch errors in the navigation tree.
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
    <AppErrorBoundary screenName="AppEntrega">
      <RootLayoutNav />
    </AppErrorBoundary>
  );
}

function RootLayoutNav() {
  const colorScheme = useColorScheme();

  return (
    <AuthProvider>
      <ThemeProvider value={DefaultTheme}>
        <StackNav />
      </ThemeProvider>
    </AuthProvider>
  );
}
