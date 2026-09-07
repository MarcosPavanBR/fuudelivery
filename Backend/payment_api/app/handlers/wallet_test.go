package handlers

import (
	"testing"
	"time"
)

// ============================================================================
// Testes unitários da carteira.
//
// Os testes anteriores deste arquivo (TestTopUpValidation,
// TestDeductFromWalletValidation, TestWalletAntiReplay) foram removidos por
// serem tautológicos: reimplementavam a condição inline (`amount <= 0`) ou
// validavam um `map` local, sem nunca chamar handler nenhum — passariam
// mesmo se os handlers fossem apagados.
//
// A idempotência de saque/débito é imposta por índice PARCIAL do Postgres
// (uq_wallet_txns_debit_ref, sql/18) e por isso só pode ser testada de
// verdade contra um banco real — está em wallet_idempotency_test.go, sob a
// tag de build `integration`.
// ============================================================================

// TestDerivedWithdrawKey cobre o fallback de idempotência usado quando o
// cliente não manda Idempotency-Key. A propriedade que importa: mesma
// requisição dentro da mesma janela produz a MESMA chave (barra o duplo
// clique); qualquer coisa diferente produz chave diferente (não bloqueia
// saque legítimo).
func TestDerivedWithdrawKey(t *testing.T) {
	base := time.Unix(1_700_000_000, 0) // instante fixo: teste não pode depender do relógio

	key := func(est int64, amount float64, dest string, ts time.Time) string {
		return derivedWithdrawKey(est, amount, dest, ts)
	}

	ref := key(42, 50.00, "pix@example.com", base)

	t.Run("mesma requisicao na mesma janela gera a mesma chave", func(t *testing.T) {
		// +30s continua no mesmo bucket de 60s
		if got := key(42, 50.00, "pix@example.com", base.Add(30*time.Second)); got != ref {
			t.Errorf("esperava mesma chave dentro da janela; ref=%s got=%s", ref, got)
		}
	})

	t.Run("janela seguinte gera chave diferente", func(t *testing.T) {
		// Sem isso, um segundo saque legítimo idêntico ficaria bloqueado pra sempre.
		if got := key(42, 50.00, "pix@example.com", base.Add(withdrawDedupWindow)); got == ref {
			t.Error("esperava chave diferente na janela seguinte, veio igual")
		}
	})

	t.Run("estabelecimento diferente gera chave diferente", func(t *testing.T) {
		if got := key(43, 50.00, "pix@example.com", base); got == ref {
			t.Error("chave não pode colidir entre estabelecimentos")
		}
	})

	t.Run("valor diferente gera chave diferente", func(t *testing.T) {
		if got := key(42, 50.01, "pix@example.com", base); got == ref {
			t.Error("chave não pode colidir entre valores diferentes")
		}
	})

	t.Run("destino diferente gera chave diferente", func(t *testing.T) {
		if got := key(42, 50.00, "outra@example.com", base); got == ref {
			t.Error("chave não pode colidir entre destinos diferentes")
		}
	})

	t.Run("chave é estável e não vazia", func(t *testing.T) {
		if ref == "" {
			t.Fatal("chave derivada não pode ser vazia — vazia cai fora do índice parcial")
		}
		if got := key(42, 50.00, "pix@example.com", base); got != ref {
			t.Error("função deve ser determinística para as mesmas entradas")
		}
	})
}
