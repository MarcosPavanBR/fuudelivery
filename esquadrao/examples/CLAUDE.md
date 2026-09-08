# Exemplo de CLAUDE.md de projeto usando este plugin

Copie e adapte para a raiz do seu repositório.

## Stack
- Backend: Go (Postgres único, sem dual-write)
- Mobile: Expo/React Native (AppComida, AppEntrega, AppRestaurante)
- Web: Next.js (WebAdmin, WebRestaurant) ou portal de conteúdo
- Pagamento: gateway multi-provedor com router central

## Convenções deste projeto
- Ver `rules/common/` para o que é sempre aplicado.
- Autorização por recurso é obrigatória em todo handler novo, REST ou WebSocket — sem exceção "só para MVP".
- Nenhuma credencial real em arquivo de documentação — sempre placeholder.

## Como pedir trabalho
- Feature nova ou mudança em >1 arquivo: comece com `/plan`.
- Antes de merge: `/quality-gate`.
- Antes de deploy: `/deploy-check`.
