---
name: go-patterns
description: Idiomas e padrões de Go para o backend — tratamento de erro, contexto de cancelamento, e organização de pacotes por domínio. Use ao escrever ou revisar qualquer código Go do backend.
---

# Padrões Go

## Erros
- Sempre `if err != nil { return fmt.Errorf("descrição curta: %w", err) }` — nunca engula o erro em silêncio.
- Erros de domínio (ex: "saldo insuficiente") são tipos próprios (`var ErrInsufficientBalance = errors.New(...)`), não strings mágicas comparadas por conteúdo.

## Contexto
- Toda função que faz I/O (chamada de rede, query de banco) recebe `context.Context` como primeiro parâmetro e o propaga — permite cancelamento e timeout em cascata.
- Jobs de background (ex: `split_decay_job.go`) escutam `ctx.Done()` para encerrar de forma limpa em vez de rodar indefinidamente após shutdown.

## Concorrência
- Nunca acesse um map ou slice compartilhado entre goroutines sem mutex ou canal — corrida de dados em Go não sempre é detectada em teste, use `go test -race` na suíte.
- Prefira canais para comunicação entre goroutines, mutex para proteger estado compartilhado simples — não misture os dois padrões pro mesmo dado.

## Organização de pacote
- Pacote por domínio (`wallet`, `order`, `delivery`), cada um com `service.go` (regra de negócio), `repository.go` (acesso a dado), `handler.go` (HTTP/WS) — não por camada técnica global.
- Interfaces pequenas, definidas onde são consumidas: `type BalanceReader interface { GetBalance(ctx, userID) (int64, error) }` no pacote que consome, não no pacote `wallet`.

## Idempotência
- Toda operação financeira aceita uma chave de idempotência explícita (não derivada de timestamp) e a suíte de testes inclui o caso de chamada duplicada.

## Ponteiro nil virando interface (typed nil)
- Nunca descarte o erro de um construtor cujo resultado vira interface: `gw, _ := pagarme.NewGateway()` seguido de `NewRouter(gw)` registra um ponteiro nil que a interface passa a considerar **não-nil** (ela guarda tipo + valor, e só é nil quando os dois são). O método com receiver nil até funciona; o primeiro que desreferencia um campo dá panic — e, dentro de uma cadeia de fallback, derruba a cadeia inteira.
- A checagem tem que ser no **erro ou no ponteiro concreto, antes da conversão**. Um `if x != nil` depois que o valor virou interface não pega nada.
- Caso real (FuuDelivery, 2026-09): quatro gateways de pagamento registrados com o erro descartado; dois retornam `nil, err` sem credencial. Corrigido montando o slice condicionalmente, com teste que falha se alguém reintroduzir o padrão.

## Data que vira string perde a validação
- Nunca leia timestamp como texto para dar `time.Parse` com layout fixo. `coluna::text` sobre um `timestamptz` produz `2026-09-08 13:41:07.123456+00`, que não casa com `"2006-01-02 15:04:05"` nem com `"2006-01-02T15:04:05Z"` — fração de segundo e offset quebram os dois. Leia como `time.Time` e compare direto.
- O que torna isso perigoso não é o parse falhar, é o **fallback**: `if err != nil { assume válido }` transforma falha de formato em permissão concedida. Caso real (FuuDelivery, 2026-09): a vigência da assinatura era lida assim, o parse falhava em 100% dos casos, e qualquer assinatura com `status = 'active'` dava frete grátis **para sempre, mesmo vencida há meses**. Nenhum erro aparecia em log.
- Ao revisar um fallback de parse, pergunte para que lado ele erra. Errar para "concede" precisa de justificativa explícita; o padrão é errar para "nega".

## Consulta que devolve uma linha só
- `Scan`/`First` numa struct única sem `ORDER BY` pega o que o banco devolver — não "a primeira" em nenhum sentido estável. Se mais de uma linha puder casar (assinatura duplicada por retry de webhook, upgrade que não encerrou a anterior), o resultado é sorteio.
- Ou garanta unicidade no banco (índice único), ou ordene explicitamente e limite: qual linha vence é regra de negócio, e regra de negócio não pode ficar implícita no plano de execução.
