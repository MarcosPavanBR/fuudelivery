// =============================================================
// tokenStorage.ts — Fonte ÚNICA de persistência do token JWT
// =============================================================
// AppEntrega (entregador). Todos os consumidores (AuthContext,
// interceptors de API) usam estas funções — nunca AsyncStorage/SecureStore
// diretamente. Ver references/autenticacao-mobile.md.
//
// Storage escolhido: expo-secure-store (cifrado pelo SO — mais seguro que
// AsyncStorage). O token do entregador NUNCA deve ir para AsyncStorage.
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

export default { setToken, getToken, clearToken };
