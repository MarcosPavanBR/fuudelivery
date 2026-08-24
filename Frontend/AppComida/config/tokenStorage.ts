// =============================================================
// tokenStorage.ts — Fonte ÚNICA de persistência do token JWT
// =============================================================
// AppComida (cliente). Todos os consumidores (ApiContext,
// interceptors de API, WebSockets) usam estas funções — nunca
// AsyncStorage/SecureStore diretamente. Ver references/autenticacao-mobile.md.
//
// Storage escolhido: expo-secure-store (cifrado pelo SO — mais seguro que
// AsyncStorage). O token do cliente NUNCA deve ir para AsyncStorage.
// =============================================================

import * as SecureStore from "expo-secure-store";
import Strings from "@/constants/Strings";

/** Grava o token JWT no SecureStore (cifrado). */
export async function setToken(token: string): Promise<void> {
  await SecureStore.setItemAsync(Strings.token_jwt, token);
}

/** Lê o token JWT do SecureStore. Retorna null se ausente. */
export async function getToken(): Promise<string | null> {
  return SecureStore.getItemAsync(Strings.token_jwt);
}

/** Remove o token JWT do SecureStore (logout ou 401). */
export async function clearToken(): Promise<void> {
  await SecureStore.deleteItemAsync(Strings.token_jwt);
}

/**
 * Telefone editado pelo usuário no perfil (override persistido).
 * O JWT carrega o phone na hora do login; como o token só é
 * reemitido num novo login, guardamos a edição aqui para o app
 * continuar usando o número certo mesmo após reiniciar.
 */
export async function setPhoneOverride(phone: string): Promise<void> {
  await SecureStore.setItemAsync(Strings.user_phone, phone);
}

/** Lê o telefone editado pelo usuário. Retorna null se ausente. */
export async function getPhoneOverride(): Promise<string | null> {
  return SecureStore.getItemAsync(Strings.user_phone);
}

/** Remove o override (logout). */
export async function clearPhoneOverride(): Promise<void> {
  await SecureStore.deleteItemAsync(Strings.user_phone);
}

// ── Refresh token (sessão longa) ──────────────────────────────────────
// Access token dura 15 min; o refresh (30 dias) renova a sessão sem novo
// login. Mesmo storage cifrado do access token.
const REFRESH_KEY = "REFRESH_TOKEN";

/** Grava o refresh token no SecureStore (cifrado). */
export async function setRefreshToken(token: string): Promise<void> {
  await SecureStore.setItemAsync(REFRESH_KEY, token);
}

/** Lê o refresh token. Retorna null se ausente. */
export async function getRefreshToken(): Promise<string | null> {
  return SecureStore.getItemAsync(REFRESH_KEY);
}

/** Remove o refresh token (logout ou falha de renovação). */
export async function clearRefreshToken(): Promise<void> {
  await SecureStore.deleteItemAsync(REFRESH_KEY);
}

export default {
  setToken,
  getToken,
  clearToken,
  setPhoneOverride,
  getPhoneOverride,
  clearPhoneOverride,
  setRefreshToken,
  getRefreshToken,
  clearRefreshToken,
};
