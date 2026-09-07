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
