package gateway

import "testing"

// Regressão: os adapters Asaas e Mercado Pago faziam int64(valor*100) sem
// arredondar, truncando 1 centavo em valores comuns (ex: R$19,99 -> 1998
// em vez de 1999, por causa de imprecisão de float64). O pix.go do
// payment_api já tinha o mesmo fix isolado (toCents); esta é a versão
// compartilhada usada pelos adapters em pkg/gateway.
func TestToCents(t *testing.T) {
	tests := []struct {
		name   string
		reais  float64
		centav int64
	}{
		{"valor do bug real", 19.99, 1999},
		{"valor inteiro", 100.0, 10000},
		{"com centavos exatos", 25.90, 2590},
		{"truncamento float classico", 99.99, 9999},
		{"casa decimal binaria", 10.10, 1010},
		{"um centavo", 0.01, 1},
		{"zero", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToCents(tt.reais); got != tt.centav {
				t.Errorf("ToCents(%v) = %d; want %d", tt.reais, got, tt.centav)
			}
		})
	}
}
