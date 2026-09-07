---
name: payment-gateway-integration
description: Padrões de integração com gateways de pagamento brasileiros e internacionais (Pix, cartão, AbacatePay, multi-gateway routing) — webhooks, idempotência, e fallback. Use PROACTIVELY sempre que o pedido envolver checkout, pagamento, ou split financeiro.
---

# Integração de Gateway de Pagamento

## Roteamento multi-gateway
- Toda cobrança passa pelo router configurado (que decide qual gateway usar), nunca um caminho direto a um provedor específico como atalho — um fallback direto reintroduz o ponto único de falha que o roteamento existe para evitar.
- O router precisa de um caso de teste que força a falha do gateway primário e confirma o fallback para o secundário.

## Webhooks
- Toda rota de webhook verifica a assinatura (HMAC ou equivalente do provedor) antes de processar qualquer evento — sem exceção para ambiente de teste, que costuma vazar para produção por engano.
- Evento com assinatura inválida ou ausente é logado e descartado, nunca processado "para não perder o evento".

## Idempotência
- Chave de idempotência derivada do ID do evento do gateway (não de timestamp de recebimento) — o mesmo evento reentregue pelo gateway (comportamento normal, não bug) não pode duplicar crédito, débito ou notificação ao usuário.

## Pix e métodos locais
- Pix não tem "cancelamento" no sentido de cartão — trate estorno como uma transação nova, nunca como reversão da original no mesmo registro.

## Conciliação
- Job periódico que compara transações do gateway com o que o sistema registrou como pago, sinalizando discrepância — sem isso, erro só aparece quando o usuário reclama.
