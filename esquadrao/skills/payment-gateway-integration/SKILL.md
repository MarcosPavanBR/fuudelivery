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
- **Índice de idempotência no banco não vale nada se o handler passa referência vazia.** Índice parcial do tipo `WHERE reference_id <> ''` protege exatamente quem manda referência; quem manda `""` cai fora e fica sem proteção nenhuma, em silêncio — a constraint existe, o teste de schema passa, e o duplo débito acontece. Ao revisar, siga o valor do `reference_id` do handler até o `INSERT`: se existir caminho onde ele chega vazio, esse caminho não é idempotente. (Caso real: o saque passava `""` — duplo submit sacava duas vezes, limitado só pelo saldo.)
- Referência de idempotência é **escopada pelo dono** (`wd:<estabelecimento>:<chave>`, `ord:<usuário>:<pedido>`) e o índice único inclui o `wallet_id`. Referência num espaço global deixa reusar a chave alheia para provocar violação: o handler lê como "replay" e responde 200 sem mover dinheiro — falso sucesso, e bloqueio da operação da outra pessoa.
- Antes de responder 200 num replay, **confirme que o lançamento é de quem está chamando**. E leia a carteira sem criar: um `GetOrCreate` no caminho de replay inventa carteira zerada e responde "saldo 0" como se a operação tivesse ocorrido.

## Valores que o cliente manda
- Todo valor do corpo da requisição que alimenta split ou ledger é reconferido contra o servidor — não só o total. Caso real: o `amount` era validado contra o pedido, mas o `delivery_amount` não; como a regra de split trata `entrega >= total` como "a entrega consome tudo", mandar `delivery_amount = amount` zerava plataforma e estabelecimento e desviava 100% do valor.
- O split precisa de teste de invariante (a soma das partes é exatamente o total) com casos de borda, não só do caminho feliz: com `total=20, entrega=19, 10%/85%` a soma dava 21,00 porque o clamp existia só numa das parcelas.

## Pix e métodos locais
- Pix não tem "cancelamento" no sentido de cartão — trate estorno como uma transação nova, nunca como reversão da original no mesmo registro.

## Conciliação
- Job periódico que compara transações do gateway com o que o sistema registrou como pago, sinalizando discrepância — sem isso, erro só aparece quando o usuário reclama.
