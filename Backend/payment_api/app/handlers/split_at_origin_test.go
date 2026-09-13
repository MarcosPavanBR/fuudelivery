package handlers

import (
	"testing"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
)

// TestApplicationFeeCents trava a aritmética do modelo 4→2 que o Marcos
// escolheu: a loja recebe a fatia dela; a application_fee é TODO o resto
// (total − fatia da loja). Errar aqui move dinheiro errado — a plataforma
// reteria de menos (perda) ou de mais (cobra o entregador/cashback da loja).
func TestApplicationFeeCents(t *testing.T) {
	casos := []struct {
		nome             string
		total, lojaShare int64
		querFee          int64
	}{
		{"85% pra loja de R$100", 10000, 8500, 1500},
		{"loja recebe tudo", 10000, 10000, 0},
		{"loja recebe zero (fee = total)", 10000, 0, 10000},
		{"share > total nunca vira fee negativa", 10000, 12000, 0},
		{"centavos exatos", 9999, 8499, 1500},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := applicationFeeCents(c.total, c.lojaShare); got != c.querFee {
				t.Errorf("applicationFeeCents(%d, %d) = %d, queria %d", c.total, c.lojaShare, got, c.querFee)
			}
		})
	}
}

// TestApplicationFeeCents_SomaFecha: fatia da loja + application_fee tem que
// somar exatamente o total. É o invariante que garante que o dinheiro todo é
// distribuído — nada some, nada é criado.
func TestApplicationFeeCents_SomaFecha(t *testing.T) {
	for _, tc := range []struct{ total, share int64 }{
		{10000, 8500}, {5000, 4250}, {12345, 6789}, {1, 0}, {99999, 99999},
	} {
		fee := applicationFeeCents(tc.total, tc.share)
		if tc.share <= tc.total && tc.share+fee != tc.total {
			t.Errorf("total=%d share=%d fee=%d: soma %d != total", tc.total, tc.share, fee, tc.share+fee)
		}
	}
}

// TestSplitAtOriginEnabled_NasceDesligado: sem a env, o recurso está OFF — o
// deploy não muda o caminho do dinheiro até alguém ligar explicitamente.
func TestSplitAtOriginEnabled_NasceDesligado(t *testing.T) {
	t.Setenv("SPLIT_AT_ORIGIN_ENABLED", "")
	if splitAtOriginEnabled() {
		t.Error("sem a env, o split na origem tem que estar DESLIGADO")
	}
	for _, off := range []string{"false", "0", "no", "off", "qualquer"} {
		t.Setenv("SPLIT_AT_ORIGIN_ENABLED", off)
		if splitAtOriginEnabled() {
			t.Errorf("%q não pode ligar o recurso", off)
		}
	}
	for _, on := range []string{"true", "TRUE", "True", " true "} {
		t.Setenv("SPLIT_AT_ORIGIN_ENABLED", on)
		if !splitAtOriginEnabled() {
			t.Errorf("%q deveria ligar o recurso", on)
		}
	}
}

// TestResolveSplitAtOrigin_FlagDesligado: com o flag off, resolve nunca tenta
// nada (nem toca no banco) — devolve ok=false direto.
func TestResolveSplitAtOrigin_FlagDesligado(t *testing.T) {
	t.Setenv("SPLIT_AT_ORIGIN_ENABLED", "false")
	_, _, ok := resolveSplitAtOrigin(&models.Payment{})
	if ok {
		t.Error("com o flag desligado, resolveSplitAtOrigin tem que devolver ok=false")
	}
}
