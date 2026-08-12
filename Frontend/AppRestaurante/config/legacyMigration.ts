// =============================================================
// legacyMigration.ts — Migração única de dados legados (AppComida)
// =============================================================
// Antes da migração para MMKV (config/storage.ts) e SecureStore
// (config/tokenStorage.ts), o app persistia dados no AsyncStorage.
// Este módulo lê essas chaves legadas UMA única vez e transfere para
// o storage atual, para não perder a localização salva dos usuários
// (e a sessão JWT) após o update.
//
// ⚠️ O módulo @react-native-async-storage/async-storage foi
// re-adicionado ao package.json APENAS para esta migração — os dados
// nativos do AsyncStorage sobrevivem no dispositivo mesmo com o módulo
// removido do bundle. Após a janela de migração (todas as instalações
// atualizadas), a dependência E este arquivo podem ser removidos.
//
// Garantias:
//  - Idempotente: roda uma vez (flag no MMKV), nunca re-executa.
//  - Nunca lança exceção — falha é logada, o startup do app continua.
//  - Copia primeiro, remove a chave legada depois (crash no meio é seguro).
// =============================================================

import { Platform } from "react-native";
import storage from "./storage";
import * as tokenStorage from "./tokenStorage";
import Strings from "@/constants/Strings";

/** Flag que marca a migração como já executada (evita re-leituras). */
const MIGRATION_FLAG = "legacy_migration_v1";

interface LegacyRule {
  /** Chave original no AsyncStorage. */
  legacyKey: string;
  /** Destino: MMKV (storage.ts) ou SecureStore (tokenStorage.ts). */
  target: "storage" | "secure";
  /** Validação opcional — dados inválidos são ignorados. */
  validate?: (value: string) => boolean;
}

/** Parece um JWT? (header.payload.signature — 3 segmentos). */
function looksLikeJwt(value: string): boolean {
  const parts = value.split(".");
  return parts.length === 3 && parts.every((p) => p.length > 0);
}

/** A localização salva é um objeto JSON (endereço do cliente)? */
function isValidLocationJson(value: string): boolean {
  try {
    const parsed = JSON.parse(value);
    return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed);
  } catch {
    return false;
  }
}

/**
 * Regras de migração do AppComida:
 *  - token_location : endereço salvo → MMKV (lido pelo ApiCartContext)
 *  - TOKEN_JWT      : sessão do cliente → SecureStore (lido pelo ApiContext)
 * (USER_DATA foi removido do código atual — nenhum leitor —, então
 *  não é migrado: seria escrever dado morto.)
 */
const LEGACY_RULES: LegacyRule[] = [
  { legacyKey: Strings.token_location, target: "storage", validate: isValidLocationJson },
  { legacyKey: Strings.token_jwt, target: "secure", validate: looksLikeJwt },
];

/** API mínima do AsyncStorage que a migração usa. */
interface LegacySource {
  getItem(key: string): Promise<string | null>;
  removeItem(key: string): Promise<void>;
}

/**
 * Resolve a fonte legada. Com o pacote re-adicionado, o AsyncStorage
 * funciona em nativo E web. Se o require falhar (ex.: bundle sem o pacote),
 * retorna null — na web a migração usa localStorage diretamente.
 */
function getLegacySource(): LegacySource | null {
  try {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    const mod = require("@react-native-async-storage/async-storage");
    const AsyncStorage = mod.default ?? mod;
    if (AsyncStorage && typeof AsyncStorage.getItem === "function") {
      return AsyncStorage as LegacySource;
    }
    return null;
  } catch {
    return null;
  }
}

/**
 * Executa a migração one-time. Deve ser chamada UMA vez no startup,
 * ANTES de montar os providers (ApiContext/ApiCartContext) para que
 * leiam os dados já migrados.
 */
export async function migrateLegacyData(): Promise<void> {
  try {
    // Já migrou? Sai imediatamente (leitura síncrona do MMKV).
    if (storage.getItem(MIGRATION_FLAG) !== null) return;

    const source = getLegacySource();
    const isWeb = Platform.OS === "web";

    // Sem fonte legada em nativo (pacote ausente): NÃO marca a flag,
    // para tentar de novo no próximo launch.
    if (!source && !isWeb) return;

    let anythingMigrated = false;
    let hadUnexpectedError = false;

    for (const rule of LEGACY_RULES) {
      try {
        let value: string | null = null;
        if (source) {
          value = await source.getItem(rule.legacyKey);
        } else if (isWeb && typeof localStorage !== "undefined") {
          // Fallback web: o backend web do AsyncStorage usa localStorage.
          value = localStorage.getItem(rule.legacyKey);
        }
        if (value == null) continue;

        if (rule.validate && !rule.validate(value)) {
          console.log(`[MIGRATION] chave legada '${rule.legacyKey}' inválida — ignorada`);
          continue;
        }

        // Não sobrescreve destino já preenchido: usuário que já logou na
        // versão SecureStore (ou já salvou localização na versão MMKV) tem
        // dado NOVO no destino e talvez um dado VELHO (stale) no AsyncStorage.
        // Copiar por cima derrubaria a sessão/troca a localização ativa.
        if (rule.target === "secure") {
          if ((await tokenStorage.getToken()) !== null) continue;
        } else if (storage.getItem(rule.legacyKey) !== null) {
          continue;
        }

        // Copia para o destino atual.
        if (rule.target === "storage") {
          storage.setItem(rule.legacyKey, value);
        } else {
          await tokenStorage.setToken(value);
        }
        anythingMigrated = true;

        // Remove a chave legada SÓ depois de copiar com sucesso — e apenas
        // em NATIVO: na web, storage.ts (localStorage) compartilha o MESMO
        // keyspace do AsyncStorage web — remover apagaria o valor recém-copiado.
        if (!isWeb && source) {
          await source.removeItem(rule.legacyKey);
        }
        console.log(
          `[MIGRATION] '${rule.legacyKey}' migrada -> ${rule.target === "storage" ? "MMKV" : "SecureStore"}`
        );
      } catch (err) {
        // Erro inesperado numa chave: não marca a flag — tenta de novo no
        // próximo launch (a chave pode ter sido copiada ou não, idempotente).
        hadUnexpectedError = true;
        console.log(`[MIGRATION] erro migrando '${rule.legacyKey}'`, err);
      }
    }

    // Flag marcada mesmo sem dados (evita re-leituras inúteis) — mas só
    // se nenhuma chave falhou de forma inesperada (senão, retenta no próximo launch).
    if (!hadUnexpectedError) {
      storage.setItem(MIGRATION_FLAG, anythingMigrated ? "1" : "0");
    }
  } catch (err) {
    // Nunca quebra o startup do app.
    console.log("[MIGRATION] falha geral", err);
  }
}

export default { migrateLegacyData };
