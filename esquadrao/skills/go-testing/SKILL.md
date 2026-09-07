---
name: go-testing
description: Padrões de teste em Go — testes de tabela, mocks de interface, e uso de -race. Use ao escrever testes Go, especialmente para lógica financeira ou de máquina de estado.
---

# Testes em Go

## Testes de tabela
```go
func TestZoneFeeDecay(t *testing.T) {
    cases := []struct {
        name string
        deliveries int
        wantFeePct float64
    }{
        {"abaixo do limiar", 0, 0.03},
        {"exatamente no limiar de transição", 50, 0.12}, // limite: sempre testar a borda exata
        {"acima do limiar", 200, 0.12},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            got := calculateZoneFee(c.deliveries)
            if got != c.wantFeePct {
                t.Errorf("got %v, want %v", got, c.wantFeePct)
            }
        })
    }
}
```

## Mocks
- Defina a interface no pacote que consome, gere ou escreva um mock manual simples — evite frameworks pesados de mock para interfaces de 1-2 métodos.

## Race detector
- Rode `go test -race ./...` no CI, não só localmente — corrida de dados em jobs de background costuma só aparecer sob carga real.

## Testes de idempotência (financeiro)
```go
func TestWalletCredit_Idempotent(t *testing.T) {
    idempotencyKey := "evt_123"
    creditWallet(userID, 1000, idempotencyKey)
    creditWallet(userID, 1000, idempotencyKey) // mesma chave, chamado 2x
    if balance := getBalance(userID); balance != 1000 {
        t.Errorf("esperava crédito único de 1000, saldo final foi %d", balance)
    }
}
```
