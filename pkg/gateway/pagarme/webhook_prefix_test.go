package pagarme

import "testing"

// TestPrefixForLog cobre a regressão de panic: o log de mismatch fatiava
// signature[:8] direto, e a assinatura vem de quem chama o webhook — uma
// assinatura curta derrubava o handler.
func TestPrefixForLog(t *testing.T) {
	casos := []struct {
		nome string
		in   string
		want string
	}{
		{"vazia", "", "..."},
		{"menor que o corte", "abc", "abc..."},
		{"exatamente no corte", "12345678", "12345678..."},
		{"maior que o corte", "1234567890abcdef", "12345678..."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := prefixForLog(c.in); got != c.want {
				t.Errorf("prefixForLog(%q) = %q, esperava %q", c.in, got, c.want)
			}
		})
	}
}
