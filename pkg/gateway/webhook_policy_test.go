package gateway

import "testing"

// TestAllowUnsignedWebhook trava a regra que os quatro adapters violavam:
// sem secret configurada eles retornavam true incondicionalmente, ou seja,
// aceitavam webhook não assinado inclusive em produção.
func TestAllowUnsignedWebhook(t *testing.T) {
	t.Run("producao rejeita (fail-closed)", func(t *testing.T) {
		t.Setenv("GO_ENV", "production")
		if AllowUnsignedWebhook("TESTE", "TESTE_WEBHOOK_SECRET") {
			t.Error("em produção, secret ausente tem que REJEITAR o webhook — " +
				"aceitar transforma erro de configuração em furo de segurança silencioso")
		}
	})

	t.Run("fora de producao permite (dev local)", func(t *testing.T) {
		t.Setenv("GO_ENV", "development")
		if !AllowUnsignedWebhook("TESTE", "TESTE_WEBHOOK_SECRET") {
			t.Error("fora de produção deve permitir, senão trava desenvolvimento local")
		}
	})

	t.Run("GO_ENV vazio nao e producao", func(t *testing.T) {
		t.Setenv("GO_ENV", "")
		if !AllowUnsignedWebhook("TESTE", "TESTE_WEBHOOK_SECRET") {
			t.Error("sem GO_ENV definido, trata como dev")
		}
	})
}
