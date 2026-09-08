---
name: security-checklist
description: Checklist de segurança a aplicar antes de qualquer merge que toque autenticação, autorização, pagamentos, WebSocket, ou dados pessoais. Use PROACTIVELY sempre que o pedido mencionar login, sessão, pagamento, webhook, GPS/rastreamento, ou "produção" — não espere o usuário perguntar sobre segurança explicitamente.
---

# Checklist de Segurança

Baseado em OWASP Top 10 + no histórico real de achados deste tipo de projeto (vazamento de credencial, IDOR em WebSocket, webhook sem HMAC — todos já ocorreram e foram corrigidos aqui, o que significa que o padrão de erro é conhecido e deve ser checado primeiro).

## 1. Autorização por recurso (não apenas autenticação)
Todo endpoint/handler que recebe um ID de recurso (pedido, entrega, usuário, restaurante) precisa confirmar que o chamador autenticado tem direito àquele recurso específico. Isso vale igualmente para REST e para WebSocket — um handler de chat e um handler de rastreamento de GPS são dois lugares distintos que precisam da mesma checagem, corrigir um não corrige o outro.

## 2. Segredos nunca versionados
- `.env`, `CREDENTIALS.md`, `DOCUMENTATION.md` com valores reais, e backups de banco (`*.json`, `*.sql`) — todos precisam estar no `.gitignore` antes do primeiro commit que os cria, não depois.
- Um backup de banco com hash de senha de usuário é um vazamento de dado pessoal, não apenas "detalhe técnico" — trate com a mesma severidade de uma chave de API exposta.
- Se uma credencial já vazou (mesmo que removida depois), ela precisa ser rotacionada — remover do histórico do git não invalida a credencial.

## 3. Webhooks de pagamento
Verificação de assinatura (HMAC) é obrigatória e não pode ter caminho de bypass — nem para debug, nem para "ambiente de teste que ficou em produção por engano".

## 4. Sessão
Cookie HttpOnly + Secure para sessão web. `localStorage`/`AsyncStorage` para token de sessão é uma regressão, mesmo que "funcione" — é acessível via XSS.

## 5. Rate limiting
Endpoints de autenticação (login, recuperação de senha) e de criação de pedido precisam de rate limiting por IP/usuário — sem isso, força bruta e abuso de cupom são triviais.

## 6. Dados pessoais (ver skill lgpd-compliance)
Qualquer coleta de dado pessoal novo (CPF, localização, foto) precisa de base legal e de plano de retenção/expurgo, não só "guardar para sempre por via das dúvidas".
