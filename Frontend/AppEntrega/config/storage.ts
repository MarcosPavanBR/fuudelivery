// =============================================================
// storage.ts — Persistência local síncrona (MMKV) do AppEntrega
// =============================================================
// Mesmo padrão do AppComida: centraliza a persistência NÃO sensível
// em um único módulo — nenhum componente acessa o storage diretamente.
//
// ⚠️ O token JWT NÃO deve usar este módulo — use config/tokenStorage.ts
// (SecureStore, cifrado pelo SO). MMKV não é cifrado por padrão.
//
// Web: MMKV é nativo-only (não existe no browser). Usamos localStorage
// como fallback, mantendo a mesma API síncrona.
// =============================================================

import { Platform } from "react-native";
// import type é apagado em tempo de compilação — não quebra o bundle web.
import type { MMKV as MMKVInstance } from "react-native-mmkv";

const STORAGE_ID = "fuudelivery-entrega";

// MMKV carregado de forma preguiçosa e apenas em plataformas nativas:
// importar react-native-mmkv no bundle web quebraria o runtime.
let mmkv: MMKVInstance | null = null;

function getMMKV(): MMKVInstance | null {
  if (Platform.OS === "web") return null;
  if (!mmkv) {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    const { MMKV } = require("react-native-mmkv");
    mmkv = new MMKV({ id: STORAGE_ID });
  }
  return mmkv;
}

/** Lê um valor do storage (síncrono). Retorna null se ausente. */
export function getItem(key: string): string | null {
  const store = getMMKV();
  if (store) return store.getString(key) ?? null;
  if (typeof localStorage !== "undefined") return localStorage.getItem(key);
  return null;
}

/** Grava um valor no storage (síncrono). */
export function setItem(key: string, value: string): void {
  const store = getMMKV();
  if (store) {
    store.set(key, value);
  } else if (typeof localStorage !== "undefined") {
    localStorage.setItem(key, value);
  }
}

/** Remove um valor do storage (síncrono). */
export function removeItem(key: string): void {
  const store = getMMKV();
  if (store) {
    store.remove(key);
  } else if (typeof localStorage !== "undefined") {
    localStorage.removeItem(key);
  }
}

export default { getItem, setItem, removeItem };
