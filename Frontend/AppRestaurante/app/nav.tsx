/**
 * NavStack — Navegação principal do AppRestaurante.
 *
 * Se não está logado, mostra tela de login.
 * Se está logado, mostra as tabs (Pedidos, Cardápio, Relatórios, Config).
 */
import React from "react";
import { Stack } from "expo-router";
import LoginScreen from "./pages/auth/login";
import { useApi } from "@/contexts/ApiContext";

export default function NavStack() {
  const { isLogged } = useApi();

  if (!isLogged) {
    return <LoginScreen />;
  }

  return (
    <Stack>
      <Stack.Screen
        name="(tabs)"
        options={{ headerShown: false }}
      />
    </Stack>
  );
}
