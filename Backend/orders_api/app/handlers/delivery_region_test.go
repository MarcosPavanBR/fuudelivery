// Package handlers - delivery_region_test.go
//
// O frete sai da REGIÃO do endereço, não de um número que o cliente manda.
//
// O buraco que isto fecha era direto: `computeOrderTotal` recebia
// `request.Distance` do corpo e o frete era `(distance × perKm) + fixa`.
// Mandar `"distance": 0` pagava só a taxa fixa, em qualquer pedido, sempre.
// O valor era recalculado no servidor — mas a partir de uma entrada escolhida
// por quem pagava.
//
// E havia um erro de negócio junto: a distância vinha do GPS do celular no
// momento do pedido, não do endereço de entrega. Quem pedia do trabalho para
// entregar em casa era cobrado pela distância até o trabalho.
package handlers

import (
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
)

// ── Normalização do CEP ──

func TestNormalizeCep(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado int
	}{
		{"01310100", 1310100},
		{"01310-100", 1310100},  // o admin digita com traço
		{" 01310100 ", 1310100}, // espaço de copiar e colar
		{"1310100", 0},          // 7 dígitos não é CEP
		{"013101000", 0},        // 9 também não
		{"", 0},
		{"abcdefgh", 0},
	}
	for _, c := range casos {
		if got := models.NormalizeCep(c.entrada); got != c.esperado {
			t.Errorf("NormalizeCep(%q) = %d, esperava %d", c.entrada, got, c.esperado)
		}
	}
}

// CEP com zero à esquerda é o caso que quebra comparação textual: como string,
// "01310100" < "9" é verdadeiro e a faixa casaria errado. Guardando inteiro,
// não existe o problema.
func TestNormalizeCep_ZeroAEsquerdaVirouNumero(t *testing.T) {
	if n := models.NormalizeCep("01310100"); n != 1310100 {
		t.Fatalf("esperava 1310100, veio %d", n)
	}
}

// ── Resolução da região ──

func TestResolveRegionFee_CasaAFaixa(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedRegiao(t, "Centro", "01000000", "01999999", 6.00)
	seedRegiao(t, "Zona Sul", "04000000", "04999999", 9.00)

	regra, ok := models.ResolveRegionFee("01310100", "São Paulo", "SP")
	if !ok || regra.Fee != 6.00 {
		t.Fatalf("CEP do Centro deveria dar 6.00, veio %+v (ok=%v)", regra, ok)
	}

	regra, ok = models.ResolveRegionFee("04500000", "São Paulo", "SP")
	if !ok || regra.Fee != 9.00 {
		t.Fatalf("CEP da Zona Sul deveria dar 9.00, veio %+v (ok=%v)", regra, ok)
	}
}

// As duas pontas entram na faixa. Fronteira aberta por engano cria um buraco de
// um CEP que cai no fallback sem ninguém notar.
func TestResolveRegionFee_FronteirasInclusivas(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedRegiao(t, "Centro", "01000000", "01999999", 6.00)

	for _, cep := range []string{"01000000", "01999999"} {
		if _, ok := models.ResolveRegionFee(cep, "São Paulo", "SP"); !ok {
			t.Errorf("CEP %s está na ponta da faixa e deveria casar", cep)
		}
	}
	for _, cep := range []string{"00999999", "02000000"} {
		if _, ok := models.ResolveRegionFee(cep, "São Paulo", "SP"); ok {
			t.Errorf("CEP %s está FORA da faixa e não deveria casar", cep)
		}
	}
}

// Faixa estreita vence faixa larga sem depender de prioridade. É o que permite
// cadastrar "São Paulo capital" inteira e depois recortar um bairro caro sem
// ter que lembrar de reordenar nada.
func TestResolveRegionFee_MaisEspecificaVence(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedRegiao(t, "SP capital", "01000000", "05999999", 12.00)
	seedRegiao(t, "Centro histórico", "01300000", "01399999", 5.00)

	regra, ok := models.ResolveRegionFee("01310100", "São Paulo", "SP")
	if !ok {
		t.Fatal("deveria casar alguma regra")
	}
	if regra.Name != "Centro histórico" {
		t.Fatalf("a faixa mais estreita deveria ganhar, veio %q (%.2f)", regra.Name, regra.Fee)
	}
}

// Prioridade menor ganha mesmo com faixa mais larga — é a saída manual para
// quando a largura não resolve.
func TestResolveRegionFee_PrioridadeVenceLargura(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedRegiao(t, "Estreita", "01300000", "01399999", 5.00)
	if err := models.DB.Create(&models.DeliveryRegionFee{
		Name: "Promoção da semana", CepStart: models.NormalizeCep("01000000"),
		CepEnd: models.NormalizeCep("05999999"), Fee: 2.00, Priority: 1, Active: true,
	}).Error; err != nil {
		t.Fatalf("semear: %v", err)
	}

	regra, _ := models.ResolveRegionFee("01310100", "São Paulo", "SP")
	if regra.Name != "Promoção da semana" {
		t.Fatalf("prioridade 1 deveria ganhar, veio %q", regra.Name)
	}
}

func TestResolveRegionFee_DesativadaNaoCasa(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	if err := models.DB.Create(&models.DeliveryRegionFee{
		Name: "Desativada", CepStart: models.NormalizeCep("01000000"),
		CepEnd: models.NormalizeCep("01999999"), Fee: 6.00, Priority: 100, Active: false,
	}).Error; err != nil {
		t.Fatalf("semear: %v", err)
	}

	if _, ok := models.ResolveRegionFee("01310100", "São Paulo", "SP"); ok {
		t.Fatal("regra desativada não pode precificar pedido")
	}
}

// Cidade na REGRA restringe; regra sem cidade vale para qualquer uma. Sem isso,
// CEPs iguais em cidades diferentes (acontece em faixas amplas) pegariam o
// preço errado.
func TestResolveRegionFee_CidadeNaRegraRestringe(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	if err := models.DB.Create(&models.DeliveryRegionFee{
		Name: "Só Santos", CepStart: models.NormalizeCep("01000000"),
		CepEnd: models.NormalizeCep("01999999"), City: "Santos", UF: "SP",
		Fee: 6.00, Priority: 100, Active: true,
	}).Error; err != nil {
		t.Fatalf("semear: %v", err)
	}

	if _, ok := models.ResolveRegionFee("01310100", "São Paulo", "SP"); ok {
		t.Fatal("regra de Santos não pode valer para São Paulo")
	}
	if _, ok := models.ResolveRegionFee("01310100", "SANTOS", "sp"); !ok {
		t.Fatal("a comparação de cidade/UF tem de ignorar caixa")
	}
}

// ── O vetor original ──

// Antes: `"distance": 0` no corpo → frete = (0 × perKm) + fixa. O cliente
// pagava só a taxa fixa em qualquer pedido. Agora o corpo não tem por onde
// influenciar: computeOrderTotal nem recebe distância.
func TestFrete_DistanciaZeroNoCorpoNaoBarateiaOPedido(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedDelivery(t, 1, 5.00, 2.00) // a taxa fixa que o atacante queria pagar
	seedRegiao(t, "Zona Leste", "08000000", "08999999", 22.00)
	seedProduct(t, 300, 1, 40.00)

	cart := cartDe(300, 1)
	_, frete, err := computeOrderTotal(cart, endereco("08500000"), 1, nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if frete != 22.00 {
		t.Fatalf("o frete é o da região do endereço (22.00), veio %.2f", frete)
	}
	if frete == 5.00 {
		t.Fatal("caiu na taxa fixa — é exatamente o furo que esta mudança fecha")
	}
}

// Sem região cadastrada, cai no por-km do estabelecimento — e NUNCA em zero.
// Zero por falta de configuração seria entregar de graça em silêncio.
func TestFrete_SemRegiaoCaiNoPorKmENuncaEmZero(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedDelivery(t, 1, 5.00, 2.00)

	fee, err := computeDeliveryFee(endereco("99999999"), 4, 1, nil, 0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if fee.Value != 13.00 { // 4 × 2 + 5
		t.Fatalf("fallback por km deveria dar 13.00, veio %.2f", fee.Value)
	}
	if fee.RegionName != "" {
		t.Fatalf("sem região o nome fica vazio, veio %q", fee.RegionName)
	}
}

// A região que casou volta na cotação para o cliente saber por que paga aquilo.
func TestFrete_NomeDaRegiaoVoltaNoResultado(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedRegiao(t, "Zona Sul", "04000000", "04999999", 9.00)

	fee, err := computeDeliveryFee(endereco("04500000"), 0, 1, nil, 0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if fee.RegionName != "Zona Sul" {
		t.Fatalf("esperava o nome da região no resultado, veio %q", fee.RegionName)
	}
	if fee.Value != 9.00 {
		t.Fatalf("esperava 9.00, veio %.2f", fee.Value)
	}
}

// CEP vazio ou malformado (pedido antigo, endereço sem CEP) não pode virar
// frete grátis: cai no por-km como qualquer endereço sem região.
func TestFrete_CepInvalidoNaoDaFreteGratis(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedDelivery(t, 1, 5.00, 2.00)
	seedRegiao(t, "Centro", "01000000", "01999999", 6.00)

	for _, cep := range []string{"", "123", "abc"} {
		fee, err := computeDeliveryFee(endereco(cep), 2, 1, nil, 0)
		if err != nil {
			t.Fatalf("CEP %q: erro inesperado: %v", cep, err)
		}
		if fee.Value == 0 {
			t.Fatalf("CEP %q não pode resultar em frete zero", cep)
		}
		if fee.Value != 9.00 { // 2 × 2 + 5
			t.Fatalf("CEP %q deveria cair no por-km (9.00), veio %.2f", cep, fee.Value)
		}
	}
}
