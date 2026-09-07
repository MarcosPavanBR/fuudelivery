---
name: nodejs-api-patterns
description: Padrões de API Node.js/Express — validação de entrada, tratamento de erro centralizado, e versionamento. Use ao construir ou revisar qualquer endpoint Node.js.
---

# Padrões de API Node.js

## Validação
- Toda entrada de body/query/params validada em uma camada única (ex: `zod`) antes de chegar na lógica de negócio — nunca confie em validação implícita do TypeScript em runtime (tipos somem na compilação).

## Tratamento de erro
- Middleware de erro centralizado que mapeia erros de domínio para status HTTP consistentes — nunca `try/catch` espalhado retornando formatos de erro diferentes em cada rota.
- Erro para o cliente nunca inclui stack trace ou detalhe interno em produção.

## Autorização por recurso
- Middleware de autenticação confirma *quem* é o chamador; a checagem de *autorização* (esse usuário pode acessar esse pedido específico?) é responsabilidade explícita de cada handler que recebe um ID — não assuma que autenticação implica autorização.

## Versionamento
- Mudança que quebra contrato existente vai em rota nova versionada (`/v2/...`), nunca sobrescreve o contrato de `/v1/...` que apps mobile em produção ainda chamam — apps mobile não atualizam instantaneamente como um site.

## Rate limiting
- Endpoints de autenticação e criação de recurso (pedido, cupom) com rate limit por IP e por usuário autenticado.
