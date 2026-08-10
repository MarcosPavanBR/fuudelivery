# 🔐 FuuDelivery — Fluxo de Autenticação Mobile

> **Auditoria:** 2 de agosto de 2026
> Documenta onde os apps mobile (AppComida e AppEntrega) guardam o token JWT,
> como o login/restauração/logout funcionam e como o 401 é tratado — com os
> achados de inconsistências verificados no código.

---

## 1. Visão Geral

Os dois apps autenticam contra o **monolito** (`cmd/fuudelivery`, Go + Fiber),
que emite **JWT** nos endpoints de login. O token é armazenado localmente e
enviado no header `Authorization: Bearer <token>` em cada request autenticada.

| Aspecto | AppComida (cliente) | AppEntrega (entregador) |
|---------|--------------------|------------------------|
| **Storage do token** | `expo-secure-store` (**SecureStore**) | `expo-secure-store` (**SecureStore** — via `config/tokenStorage.ts`) ✅ |
| **Chave do token** | `TOKEN_JWT` | `JWT-TOKENIZED` |
| **Endpoint login** | `POST /users/login` | `POST /api/auth/delivery-man/login` ⚠️ |
| **Endpoint register** | `POST /users/register` | `POST /api/auth/delivery-man/register` ⚠️ |
| **Decodificação JWT** | lib `jwt-decode` | manual (Buffer/base64url) |
| **Decode inválido no login** | Bloqueia login com Alert | Não valida (assume válido) |

> **Nota:** os dois apps usam **chaves de storage diferentes** (`TOKEN_JWT` vs
> `JWT-TOKENIZED`). Isso não é um problema entre apps (cada um tem seu storage
> isolado), mas significa que não há um padrão único no projeto.

---

## 2. AppComida (cliente)

### 2.1 Armazenamento do token
- **SecureStore** (cifrado pelo SO — mais seguro), chave `Strings.token_jwt = "TOKEN_JWT"`.
- Arquivos: `contexts/ApiContext.tsx` (grava/apaga), `services/api.tsx` (lê/apaga).

### 2.2 Fluxo de Login
```
login.tsx ──POST /users/login (ou /users/register)──▶ { token }
    └─ ApiContext.login(token)
        ├─ decodeJWT(token) → se inválido: Alert + aborta
        ├─ SecureStore.setItemAsync("TOKEN_JWT", token)
        ├─ setToken/setUserData/setIsLogged(true)
        └─ registra push token em background
```
- Arquivo: `app/pages/auth/login.tsx` + `contexts/ApiContext.tsx`.

### 2.3 Restauração de sessão (app aberto)
```
ApiProvider mount
  └─ SecureStore.getItemAsync("TOKEN_JWT")
      ├─ existe + decode válido → isLogged = true
      └─ decode falhou → SecureStore.deleteItemAsync (limpa lixo)
```
- `isLoading` controla a tela de carregamento até a verificação terminar.

### 2.4 Logout
- `SecureStore.deleteItemAsync("TOKEN_JWT")` + reset dos estados (`token`, `userData`, `isLogged`).

### 2.5 Tratamento de 401 ✅ (corrigido em 2026-08-02)
```
api.tsx (response interceptor)
  └─ se status 401 → clearToken() + onUnauthorized?.()
       └─ onUnauthorized = logout do ApiContext (registrado via setOnUnauthorized)
            └─ nav.tsx (isLogged=false) redireciona ao LoginScreen imediatamente
```
- **Antes:** o interceptor só apagava o token do storage — o `isLogged` do contexto
  continuava `true` e o usuário só voltava ao login na próxima abertura do app.
- **Depois:** o `ApiContext` registra seu `logout()` como callback de sessão expirada
  (`setOnUnauthorized`), então um 401 limpa o storage **e** sincroniza o estado —
  o `nav.tsx` mostra a tela de login imediatamente. (Sem dependência circular:
  `api.tsx` não importa o contexto — o contexto registra o callback.)

### 2.6 ✅ Bug CORRIGIDO (2026-08-02) — LiveTrackingReadonly lia de AsyncStorage
**Antes:** `components/LiveTrackingReadonly.tsx` lia o token com
`AsyncStorage.getItem(Strings.token_jwt)`, mas o AppComida guarda em SecureStore
→ token nunca encontrado → WebSocket de tracking nunca conectava.

**Depois:** o componente usa `const { token } = useApi()` (o token do contexto,
mesma fonte do SecureStore em `ApiContext`) — o WebSocket conecta de verdade e
reconecta quando o token muda (dependência `[orderId, token]` no `useEffect`).

### 2.7 ✅ AsyncStorage substituído por MMKV (2026-08-03) — dados não sensíveis
**Antes:** `contexts/ApiCartContext.tsx` persistia a localização salva com
`AsyncStorage` (`Strings.token_location`).

**Depois:** criado **`config/storage.ts`** — fonte única de persistência local
síncrona via **MMKV** (`react-native-mmkv`, ~30x mais rápido que AsyncStorage),
no mesmo padrão centralizado do `config/tokenStorage.ts`. No web, onde MMKV não
existe, cai para `localStorage` (mesma API síncrona).

```
ApiCartContext
  ├─ getMyLocationStorange() → storage.getItem(Strings.token_location)   (síncrono)
  └─ setMyLocation(locs)     → storage.setItem(Strings.token_location, JSON)  (síncrono)

config/storage.ts
  ├─ nativo: new MMKV({ id: "fuudelivery-app" })   (síncrono, rápido)
  └─ web:    localStorage (fallback)
```

- **O token JWT NÃO usa MMKV** — continua exclusivamente no SecureStore via
  `config/tokenStorage.ts` (MMKV não é cifrado; token nunca vai para storage
  não cifrado).
- **Migração one-time:** `config/legacyMigration.ts` transfere a localização
  salva (e o JWT antigo) do AsyncStorage para o novo storage no primeiro
  launch após o update — ver seção **2.8**.
- `react-native-mmkv` instalado com `npx expo install` (versão do SDK 54).
- ⚠️ **Dev build:** MMKV é módulo nativo — requer *development build* (EAS
  build/dev client). Não funciona em Expo Go.

### 2.8 ✅ Migração one-time AsyncStorage → MMKV/SecureStore (2026-08-03)

**Problema:** usuários com dados salvos ANTES do update (2.7) perderiam a
localização salva — o novo código lê de MMKV, mas os dados antigos viviam no
AsyncStorage (que vive em storage nativo separado no dispositivo).

**Solução:** **`config/legacyMigration.ts`** — migração única, chamada no
`app/_layout.tsx` **antes** de montar os providers (sem race condition):

```
app/_layout.tsx (RootLayout)
  └─ migrateLegacyData().finally(() => setMigrationDone(true))
       └─ (só renderiza a árvore com providers DEPOIS da migração)

config/legacyMigration.ts
  ├─ Regras (AppComida):
  │    token_location (endereço salvo) → storage.ts (MMKV)   [valida JSON]
  │    TOKEN_JWT (sessão antiga)        → tokenStorage.ts (SecureStore) [valida JWT]
  ├─ Flag legacy_migration_v1 (no MMKV) → idempotente: roda UMA vez
  ├─ Copia primeiro, remove a chave legada depois (só em nativo)
  └─ Nunca lança exceção — erro é logado e a flag NÃO é marcada (retenta)
```

- **Por que `@react-native-async-storage/async-storage` voltou ao package.json:**
  os dados nativos do AsyncStorage sobrevivem no dispositivo mesmo com o módulo
  fora do bundle — a única forma de lê-los é com o próprio módulo. A dependência
  (1.23.1) foi re-adicionada **apenas para esta migração**.
- **Web:** no browser, `storage.ts` usa `localStorage` — o MESMO keyspace do
  backend web do AsyncStorage. Por isso a chave legada **não é removida na web**
  (remover apagaria o valor recém-copiado). Em nativo, MMKV e AsyncStorage são
  stores separados — aí a remoção é segura e acontece.
- **Retry:** se uma chave falhar de forma inesperada, a flag não é gravada e a
  migração tenta de novo no próximo launch (idempotente).
- **Limpeza futura:** após a janela de migração (todas as instalações
  atualizadas), `async-storage` + `legacyMigration.ts` podem ser removidos.

---

## 3. AppEntrega (entregador)

### 3.1 Armazenamento do token ✅ (corrigido em 2026-08-02)
- **SecureStore** (cifrado pelo SO), chave `Strings.token_jwt = "JWT-TOKENIZED"`, via **`config/tokenStorage.ts`** — fonte única.
- Arquivos: `config/tokenStorage.ts` (grava/lê/apaga), `contexts/AuthContext.tsx` e `services/api.tsx` usam apenas as funções do módulo.

### 3.2 Fluxo de Login
```
login.tsx ──POST /api/auth/delivery-man/login──▶ { token }
    └─ AuthContext.login(email, senha)
        ├─ setToken(token) → SecureStore.setItemAsync("JWT-TOKENIZED", token)
        ├─ decode manual (Buffer) → setUser(decoded)
        ├─ nav.navigate("index")
        └─ setIsLoading(false)
```
- Arquivo: `contexts/AuthContext.tsx`. O register segue o mesmo padrão.

### 3.3 Restauração de sessão
```
checkAuth() → getUser()
  └─ getToken() → SecureStore.getItemAsync("JWT-TOKENIZED")
      └─ decode manual → setUser → isLogged = true
```

### 3.4 Logout
- `clearToken()` → `SecureStore.deleteItemAsync("JWT-TOKENIZED")` + `setUser(null)`.

### 3.5 ✅ Bug CRÍTICO CORRIGIDO (2026-08-02) — storage divergente entre contexto e API
**Antes:** `AuthContext.tsx` gravava em AsyncStorage, mas `services/api.tsx` lia e
apagava em SecureStore → header `Authorization` nunca montado + re-login quebrado.

> **401 → logout (2026-08-02):** o `AuthContext` registra seu `logout()` via
> `setOnUnauthorized` (mesmo padrão do AppComida) — o interceptor de 401 agora
> limpa o token **e** dispara o logout do contexto, fazendo o `nav.tsx` redirecionar
> ao login imediatamente.

**Depois:** ambos usam `config/tokenStorage.ts` (SecureStore), criado na correção:

| Operação | Antes | Depois |
|----------|-------|--------|
| **Escrever** | `AsyncStorage.setItem` (AuthContext) | `setToken()` → SecureStore ✅ |
| **Ler p/ Authorization** | `SecureStore.getItemAsync` (api.tsx) | `getToken()` → SecureStore ✅ |
| **401 handler** | `SecureStore.deleteItemAsync` (api.tsx) | `clearToken()` → SecureStore ✅ |
| **Logout** | `AsyncStorage.removeItem` (AuthContext) | `clearToken()` → SecureStore ✅ |
| **Restauração** | `AsyncStorage.getItem` (AuthContext) | `getToken()` → SecureStore ✅ |

- **Efeito da correção:** o token é gravado e lido do **mesmo storage** (SecureStore,
  cifrado) — o header `Authorization` volta a ser enviado e o fluxo de re-login
  após 401 funciona (o token real é removido no 401/logout).
- **Nota:** usuários com token antigo em AsyncStorage faziam login de novo uma
  única vez após o update. **Desde 2026-08-03 isso é automático** — a migração
  one-time (`config/legacyMigration.ts`, seção 3.8) transfere o JWT para o
  SecureStore no primeiro launch.

### 3.7 ✅ AsyncStorage → MMKV no AppEntrega (2026-08-03) — dados não sensíveis

**Antes:** o AppEntrega não tinha um storage geral centralizado — a persistência
não sensível (se existisse em componentes futuros) cairia em AsyncStorage, com
API assíncrona/lenta.

**Depois:** criado **`config/storage.ts`** — mesma fonte única do AppComida:
persistência local **síncrona** via **MMKV** (`react-native-mmkv ~4.3.2`),
com fallback para `localStorage` no web (mesma API síncrona).

```
config/storage.ts
  ├─ nativo: new MMKV({ id: "fuudelivery-entrega" })   (síncrono, rápido)
  └─ web:    localStorage (fallback — MMKV nunca é importado no bundle web)
```

- **O token JWT NÃO usa MMKV** — continua exclusivamente no SecureStore via
  `config/tokenStorage.ts` (MMKV não é cifrado; token nunca vai para storage
  não cifrado). `AuthContext.tsx` e `services/api.tsx` não foram tocados.
- **Migração one-time:** `config/legacyMigration.ts` transfere o JWT antigo do
  AsyncStorage para o SecureStore no primeiro launch após o update — ver seção **3.8**.
- `react-native-mmkv` adicionado ao `package.json` (mesma versão do AppComida).
- ⚠️ **Dev build:** MMKV é módulo nativo — requer *development build* (EAS
  build/dev client). Não funciona em Expo Go.

### 3.8 ✅ Migração one-time AsyncStorage → SecureStore (2026-08-03)

**Problema:** na 3.5 (bug do storage divergente), o token era escrito no
AsyncStorage pelo `AuthContext`. Usuários com sessão antiga nesse storage seriam
**deslogados silenciosamente** após o update — o novo código só lê SecureStore.

**Solução:** **`config/legacyMigration.ts`** — migração única, chamada no
`app/_layout.tsx` **antes** de montar o `AuthProvider`:

```
app/_layout.tsx (RootLayout)
  └─ migrateLegacyData().finally(() => setMigrationDone(true))
       └─ (só renderiza AuthProvider DEPOIS da migração)

config/legacyMigration.ts
  ├─ Regra (AppEntrega):
  │    JWT-TOKENIZED (sessão antiga) → tokenStorage.ts (SecureStore) [valida JWT]
  ├─ Flag legacy_migration_v1 (no MMKV) → idempotente: roda UMA vez
  ├─ Copia primeiro, remove a chave legada depois (só em nativo)
  └─ Nunca lança exceção — erro é logado e a flag NÃO é marcada (retenta)
```

- A dependência `@react-native-async-storage/async-storage` (1.23.1) foi
  re-adicionada **apenas para esta migração** — os dados nativos do AsyncStorage
  sobrevivem no dispositivo mesmo com o módulo fora do bundle.
- **Web:** mesmo keyspace (localStorage) → a chave legada não é removida na web.
- **Limpeza futura:** após a janela de migração, `async-storage` +
  `legacyMigration.ts` podem ser removidos.

### 3.6 ✅ Mismatch de endpoints CORRIGIDO (2026-08-02) — testado em produção
**Teste real na URL de produção** (`https://fuudelivery-api-8y6l.onrender.com`):

| Endpoint | HTTP | Resultado |
|----------|------|-----------|
| `POST /api/auth/delivery-man/login` (o que o app chamava) | **404** | ❌ Não existe — provável herança do serviço antigo (`delivery_api`) |
| `POST /delivery-man/login` (monolito) | **403** "Incorrect credentials" | ✅ Rota existe e funciona (403 é esperado p/ senha inválida) |

**Correção aplicada** (6 endpoints no AppEntrega, removendo os prefixos
`/api/auth` e `/api/delivery` que não existem no monolito):

| Arquivo | Antes | Depois |
|---------|-------|--------|
| `contexts/AuthContext.tsx` | `/api/auth/delivery-man/login` | `/delivery-man/login` |
| `contexts/AuthContext.tsx` | `/api/auth/delivery-man/register` | `/delivery-man/register` |
| `contexts/AuthContext.tsx` | `/api/delivery/deliveryman/has-active/:id` | `/deliveryman/has-active/:id` |
| `app/delivery_mode.tsx` | `/api/delivery/deliveryman/status` | `/deliveryman/status` |
| `app/confirm.tsx` | `/api/delivery/solicitation-orders/hand-shake` | `/solicitation-orders/hand-shake` |
| `app/(tabs)/two.tsx` | `/api/delivery/deliveryman/extrato/:id` | `/deliveryman/extrato/:id` |
| `app/pages/home.tsx` | `/api/delivery/solicitation-orders` | `/solicitation-orders` |
| `contexts/AuthContext.tsx` (comentado) | `/api/delivery/ws/:id` | `/ws/delivery/:id` |

> O WS comentado foi alinhado à rota real do monolito (`GET /ws/delivery/:orderId`).
> Verificado: **0 referências `/api/` restantes** nos sources do AppEntrega.

---

## 4. Comparativo — Fluxo de re-login após 401

| Etapa esperada | AppComida | AppEntrega |
|----------------|-----------|------------|
| Detectar 401 na resposta | ✅ | ✅ |
| Apagar token do storage | ✅ (tokenStorage) | ✅ (tokenStorage) |
| Redirecionar para a tela de login | ✅ (**corrigido** — logout via `setOnUnauthorized`) | ✅ (**corrigido** — mesmo padrão) |
| Enviar Authorization nas requests | ✅ | ✅ |
| WebSocket de tracking usa o token certo | ✅ (`useApi().token`) | — (não usa WS com token) |

**Conclusão (estado atual):** o ciclo completo de re-login após 401 agora é
**consistente e funcional** nos dois apps — o 401 limpa o token (SecureStore via
`config/tokenStorage.ts`) e dispara o logout do contexto, e o `nav.tsx` redireciona
ao login imediatamente.

---

## 5. Recomendações (priorizadas)

1. **[✅ FEITO] Padronizar o storage do AppEntrega:** criado `config/tokenStorage.ts`
   (SecureStore) e alinhados `AuthContext.tsx` + `services/api.tsx` (2026-08-02).
2. **[✅ FEITO] Endpoints do AppEntrega corrigidos (2026-08-02):** testado em
   produção (404 no caminho antigo) e corrigidos os 6 endpoints + WS comentado
   para bater com o monolito.
3. **[✅ FEITO] 401 sincroniza o estado (2026-08-02):** interceptors de ambos os
   apps registram o logout do contexto via `setOnUnauthorized` → `nav.tsx`
   redireciona ao login imediatamente.
4. **[✅ FEITO] AppComida — LiveTrackingReadonly:** agora usa `useApi().token`
   (contexto) em vez de AsyncStorage (2026-08-02).
5. **[✅ FEITO] Aplicar o mesmo `tokenStorage` no AppComida (2026-08-02):** criado
   `Frontend/AppComida/config/tokenStorage.ts` e migrados `ApiContext.tsx` e
   `services/api.tsx` — agora os dois apps têm o mesmo padrão centralizado.
6. **[✅ FEITO] AsyncStorage → MMKV no AppComida (2026-08-03):** criado
   `config/storage.ts` (MMKV síncrono + fallback web) e migrado o único uso
   restante (`ApiCartContext.tsx`, localizações). Token segue só no SecureStore.
7. **[✅ FEITO] Mesmo `config/storage.ts` no AppEntrega (2026-08-03):** criado
   `Frontend/AppEntrega/config/storage.ts` (MMKV síncrono + fallback web) com
   `react-native-mmkv ~4.3.2`. Token segue só no SecureStore via
   `config/tokenStorage.ts` (intocado).
8. **[✅ FEITO] Migração one-time AsyncStorage → novo storage (2026-08-03):**
   `config/legacyMigration.ts` nos dois apps — AppComida migra `token_location`
   (→ MMKV) e `TOKEN_JWT` (→ SecureStore); AppEntrega migra `JWT-TOKENIZED`
   (→ SecureStore). Idempotente (flag), valida dados, não remove na web (mesmo
   keyspace), retenta em erro. `async-storage` 1.23.1 re-adicionado apenas para
   a janela de migração (removível depois).

---

## 6. Arquivos de referência

| Arquivo | Papel |
|---------|-------|
| `Frontend/AppComida/contexts/ApiContext.tsx` | Login/restore/logout (SecureStore) |
| `Frontend/AppComida/config/tokenStorage.ts` | Fonte única do token (SecureStore) — **novo** |
| `Frontend/AppComida/config/storage.ts` | Persistência síncrona geral (MMKV + fallback web) — **novo** |
| `Frontend/AppComida/config/legacyMigration.ts` | Migração one-time AsyncStorage → MMKV/SecureStore — **novo** |
| `Frontend/AppComida/services/api.tsx` | Interceptors (tokenStorage + callback 401) |
| `Frontend/AppComida/components/LiveTrackingReadonly.tsx` | WebSocket tracking (usa `useApi().token`) ✅ |
| `Frontend/AppComida/app/nav.tsx` | Gate de navegação por `isLogged` |
| `Frontend/AppEntrega/config/tokenStorage.ts` | Fonte única do token (SecureStore) — **novo** |
| `Frontend/AppEntrega/config/storage.ts` | Persistência síncrona geral (MMKV + fallback web) — **novo** |
| `Frontend/AppEntrega/config/legacyMigration.ts` | Migração one-time AsyncStorage → SecureStore — **novo** |
| `Frontend/AppEntrega/contexts/AuthContext.tsx` | Login/register/restore/logout (via tokenStorage) |
| `Frontend/AppEntrega/services/api.tsx` | Interceptors (via tokenStorage) |
| `Frontend/AppEntrega/app/nav.tsx` | Gate de navegação por `isLogged` |
| `cmd/fuudelivery/main.go` | Rotas de auth do monolito (`/users/login`, `/delivery-man/login`, ...) |
