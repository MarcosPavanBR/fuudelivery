---
name: websocket-security
description: Padrões de segurança para handlers WebSocket — autorização por recurso, autenticação da conexão, e prevenção de vazamento entre usuários. Use PROACTIVELY para qualquer handler WS novo ou alterado (chat, rastreamento de entrega, status de pedido em tempo real).
---

# Segurança em WebSocket

## Autorização por recurso (o erro mais comum e mais caro aqui)
Autenticar a conexão (saber quem é o usuário) não é o mesmo que autorizar o dado que ele está pedindo. Todo handler que aceita um ID de recurso via mensagem ou parâmetro de conexão (`orderId`, `deliveryId`) precisa confirmar que o usuário conectado é dono ou participante legítimo daquele recurso específico — a cada mensagem relevante, não só na conexão inicial.

Isso precisa ser verificado **por handler**, não uma vez só: um handler de chat correto não significa que o handler de rastreamento de GPS também está correto — são pontos de checagem independentes, e já houve caso aqui onde a documentação afirmava a checagem existir em um lugar que na prática não tinha.

## Autenticação da conexão
- Token de autenticação validado no handshake, não apenas confiado porque veio de um cliente "esperado".
- Conexão sem token válido ou expirado é rejeitada no handshake, nunca aceita e filtrada depois.

## Vazamento entre usuários
- Nunca faça broadcast de uma mensagem para todos os conectados quando ela deveria ir só para os participantes de um recurso específico — isso vaza dado (ex: localização de uma entrega) para qualquer usuário autenticado no sistema.

## Reconexão
- Ao reconectar, o cliente deve re-autenticar e re-autorizar — não reaproveitar um estado de autorização assumido de uma conexão anterior.
