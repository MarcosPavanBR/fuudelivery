package handlers

import "testing"

// TestRepasseEnabled_NasceDesligado: sem a env, o repasse está OFF — nenhuma
// dívida é criada e a trava não bloqueia nada.
func TestRepasseEnabled_NasceDesligado(t *testing.T) {
	t.Setenv("REPASSE_ENABLED", "")
	if repasseEnabled() {
		t.Error("sem a env, o repasse tem que estar DESLIGADO")
	}
	for _, on := range []string{"true", "TRUE", " true "} {
		t.Setenv("REPASSE_ENABLED", on)
		if !repasseEnabled() {
			t.Errorf("%q deveria ligar o repasse", on)
		}
	}
	for _, off := range []string{"false", "0", "sim", "1"} {
		t.Setenv("REPASSE_ENABLED", off)
		if repasseEnabled() {
			t.Errorf("%q não pode ligar o repasse", off)
		}
	}
}

// TestRepasseCreditLimit: valor ausente ou inválido cai no default conservador,
// NUNCA em "sem limite" — a trava não pode ser desarmada por erro de config.
func TestRepasseCreditLimit(t *testing.T) {
	t.Setenv("REPASSE_CREDIT_LIMIT_CENTS", "")
	if got := repasseCreditLimitCents(); got != repasseCreditLimitDefaultCents {
		t.Errorf("ausente: obtive %d, queria default %d", got, repasseCreditLimitDefaultCents)
	}
	for _, ruim := range []string{"abc", "-100", "1.5", ""} {
		t.Setenv("REPASSE_CREDIT_LIMIT_CENTS", ruim)
		if got := repasseCreditLimitCents(); got != repasseCreditLimitDefaultCents {
			t.Errorf("%q devia cair no default, obtive %d", ruim, got)
		}
	}
	t.Setenv("REPASSE_CREDIT_LIMIT_CENTS", "100000")
	if got := repasseCreditLimitCents(); got != 100000 {
		t.Errorf("valor válido: obtive %d, queria 100000", got)
	}
	// Zero é válido e é o mais rígido: qualquer dívida em aberto bloqueia.
	t.Setenv("REPASSE_CREDIT_LIMIT_CENTS", "0")
	if got := repasseCreditLimitCents(); got != 0 {
		t.Errorf("zero é válido, obtive %d", got)
	}
}

// TestDebtBlocksNewOrder trava a regra: bloqueia quando a dívida ATINGE o limite.
func TestDebtBlocksNewOrder(t *testing.T) {
	casos := []struct {
		open, limit  int64
		querBloqueio bool
	}{
		{0, 50000, false},     // sem dívida, livre
		{49999, 50000, false}, // abaixo do teto, livre
		{50000, 50000, true},  // atingiu o teto, bloqueia
		{60000, 50000, true},  // acima, bloqueia
		{1, 0, true},          // limite zero: qualquer dívida bloqueia
		{0, 0, true},          // limite zero e dívida zero: 0>=0 bloqueia (config mais rígida possível)
	}
	for _, c := range casos {
		if got := debtBlocksNewOrder(c.open, c.limit); got != c.querBloqueio {
			t.Errorf("debtBlocksNewOrder(%d, %d) = %v, queria %v", c.open, c.limit, got, c.querBloqueio)
		}
	}
}

// TestEstablishmentBlockedByDebt_FlagDesligado: com o repasse off, nunca
// bloqueia (nem toca no banco).
func TestEstablishmentBlockedByDebt_FlagDesligado(t *testing.T) {
	t.Setenv("REPASSE_ENABLED", "false")
	blocked, _, _ := EstablishmentBlockedByDebt(42)
	if blocked {
		t.Error("com o repasse desligado, EstablishmentBlockedByDebt tem que devolver false")
	}
}
