/**
 * RootLayout — Layout raiz do AppRestaurante.
 *
 * Versão simplificada do AppComida (sem cart, sem onboarding).
 * Foco: pedidos, cardápio, relatórios e configurações.
 */
import FontAwesome from "@expo/vector-icons/FontAwesome";
import { DefaultTheme, ThemeProvider } from "expo-router/react-navigation";
import { useFonts } from "expo-font";
import * as SplashScreen from "expo-splash-screen";
import "react-native-reanimated";
import { useEffect } from "react";
import { useColorScheme } from "@/components/useColorScheme";
import "../global.css";
import { ApiProvider } from "@/contexts/ApiContext";
import NavStack from "./nav";
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

  useEffect(() => {
    if (error) throw error;
  }, [error]);

  useEffect(() => {
    if (loaded) {
      SplashScreen.hideAsync();
    }
  }, [loaded]);

  if (!loaded) {
    return null;
  }

  return (
    <ErrorBoundary screenName="AppRestaurante">
      <RootLayoutNav />
    </ErrorBoundary>
  );
}

function RootLayoutNav() {
  const colorScheme = useColorScheme();
  return (
    <ApiProvider>
      <ThemeProvider value={DefaultTheme}>
        <NavStack />
      </ThemeProvider>
    </ApiProvider>
  );
}
