package handlers

import "testing"

// resolveDeliveryAmount fecha um desvio de dinheiro real.
//
// O Amount da cobrança já era conferido contra o servidor
// (validateChargeAmount), mas o DeliveryAmount vinha CRU do corpo da
// requisição e seguia direto para services.CalculateSplitRules — onde
// `deliveryAmount >= total` zera plataforma E estabelecimento e manda o
// valor inteiro para a regra "deliveryman". Um cliente mandando
// delivery_amount igual ao amount pagava o pedido normalmente e desviava
// 100% do dinheiro do estabelecimento.
//
// Estes testes rodam com models.DB == nil, então lookupOrderDelivery devolve
// false e o caminho exercitado é o de PEDIDO LEGADO (sem deliveryValue
// gravado) — justamente onde só a guarda de sanidade protege. O caminho com
// o frete do pedido gravado é coberto no teste de integração.

func TestResolveDeliveryAmount_RejeitaFreteQueEngoleOTotal(t *testing.T) {
	casos := []struct {
		nome     string
		delivery float64
		total    float64
	}{
		{"frete igual ao total (zera estabelecimento)", 100.00, 100.00},
		{"frete acima do total", 150.00, 100.00},
		{"frete um centavo acima", 100.01, 100.00},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			if _, ok := resolveDeliveryAmount("pedido-legado", tc.delivery, tc.total); ok {
				t.Fatalf("frete %.2f num pedido de %.2f deveria ser rejeitado", tc.delivery, tc.total)
			}
		})
	}
}

func TestResolveDeliveryAmount_RejeitaNegativo(t *testing.T) {
	if _, ok := resolveDeliveryAmount("pedido-legado", -1.00, 100.00); ok {
		t.Fatal("frete negativo deveria ser rejeitado")
	}
}

// Contraprova: sem isto, a guarda acima poderia estar simplesmente
// rejeitando tudo e o teste continuaria verde.
func TestResolveDeliveryAmount_AceitaFreteLegitimo(t *testing.T) {
	casos := []struct {
		nome     string
		delivery float64
	}{
		{"frete comum", 7.00},
		{"retirada no balcão (frete zero)", 0.00},
		{"frete um centavo abaixo do total", 99.99},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			got, ok := resolveDeliveryAmount("pedido-legado", tc.delivery, 100.00)
			if !ok {
				t.Fatalf("frete %.2f num pedido de 100.00 deveria ser aceito", tc.delivery)
			}
			if got != tc.delivery {
				t.Fatalf("esperava frete %.2f preservado, veio %.2f", tc.delivery, got)
			}
		})
	}
}
