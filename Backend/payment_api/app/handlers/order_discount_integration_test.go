//go:build integration

package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// O desconto do cupom chega à cobrança vindo do PEDIDO.
//
// Por que integração e não unitário: a leitura é uma query com casts do
// Postgres (`payload->>'discount_amount'::float8`) — o sqlite recusa a
// sintaxe, e um erro no nome da chave JSON cairia silenciosamente no ramo
// "sem cupom", deixando o split dividir como sempre dividiu. Verde pelo motivo
// errado é exatamente o que estes testes existem para impedir: o sintoma de um
// desconto que não chega é o restaurante pagando uma promoção da plataforma, e
// isso não aparece em lugar nenhum porque a conta continua fechando.
//
// Como rodar:
//
//	go test -tags=integration -run 'Desconto' ./app/handlers/
// ============================================================================

// seedOrderComCupom grava um pedido como o orders_api grava: o total JÁ
// descontado, o frete, e os dois campos de cupom.
func seedOrderComCupom(t *testing.T, legacyID string, orderTotal, deliveryValue, desconto float64, fundedBy string) {
	t.Helper()
	require.NoError(t, models.DB.Exec(
		`INSERT INTO order_documents (legacy_id, establishment_id, user_phone, payload)
		 VALUES (?, 7, '+5511999900001',
		         jsonb_build_object(
		             'order_total', ?::float8,
		             'deliveryValue', ?::float8,
		             'discount_amount', ?::float8,
		             'discount_funded_by', ?::text,
		             'establishmentId', 7))`,
		legacyID, orderTotal, deliveryValue, desconto, fundedBy).Error)
}

func TestLookupOrderRecipient_TrazDescontoEQuemBanca(t *testing.T) {
	teardown := setupCheckoutE2EEnv(t)
	defer teardown()
	createOrderDocumentsTable(t)

	t.Run("cupom da loja", func(t *testing.T) {
		seedOrderComCupom(t, "ord-desc-loja", 90.00, 10.00, 10.00, "establishment")
		facts, ok := lookupOrderRecipient("ord-desc-loja")
		require.True(t, ok)
		require.InDelta(t, 10.00, facts.DiscountAmount, 0.001)
		require.Equal(t, "establishment", facts.DiscountFundedBy)
	})

	t.Run("cupom da plataforma", func(t *testing.T) {
		seedOrderComCupom(t, "ord-desc-plat", 90.00, 10.00, 10.00, "platform")
		facts, ok := lookupOrderRecipient("ord-desc-plat")
		require.True(t, ok)
		require.InDelta(t, 10.00, facts.DiscountAmount, 0.001)
		require.Equal(t, "platform", facts.DiscountFundedBy)
	})

	// Pedido sem cupom: o split precisa dividir exatamente como sempre dividiu.
	t.Run("pedido sem cupom não inventa desconto", func(t *testing.T) {
		// Sem os dois campos de cupom no payload — como todo pedido criado
		// antes desta mudança. (seedOrderDocument não serve aqui: ele não
		// grava establishment_id, e lookupOrderRecipient exige destinatário.)
		require.NoError(t, models.DB.Exec(
			`INSERT INTO order_documents (legacy_id, establishment_id, user_phone, payload)
			 VALUES ('ord-sem-cupom', 7, '+5511999900001',
			         jsonb_build_object('order_total', 100.0, 'deliveryValue', 10.0))`).Error)
		facts, ok := lookupOrderRecipient("ord-sem-cupom")
		require.True(t, ok)
		require.Zero(t, facts.DiscountAmount)
		require.Empty(t, facts.DiscountFundedBy)
	})

	// funded_by desconhecido não pode virar cobrança no restaurante: quem não
	// escolheu bancar não banca.
	t.Run("funded_by inválido cai na plataforma", func(t *testing.T) {
		seedOrderComCupom(t, "ord-desc-invalido", 90.00, 10.00, 10.00, "entregador")
		facts, ok := lookupOrderRecipient("ord-desc-invalido")
		require.True(t, ok)
		require.Equal(t, "platform", facts.DiscountFundedBy,
			"funded_by desconhecido nunca pode cobrar do estabelecimento")
	})

	// Desconto com dono ausente (payload gravado antes da migração 22).
	t.Run("desconto sem funded_by cai na plataforma", func(t *testing.T) {
		require.NoError(t, models.DB.Exec(
			`INSERT INTO order_documents (legacy_id, establishment_id, payload)
			 VALUES ('ord-desc-sem-dono', 7,
			         jsonb_build_object('order_total', 90.0, 'discount_amount', 10.0))`).Error)
		facts, ok := lookupOrderRecipient("ord-desc-sem-dono")
		require.True(t, ok)
		require.InDelta(t, 10.00, facts.DiscountAmount, 0.001)
		require.Equal(t, "platform", facts.DiscountFundedBy)
	})
}

// O corpo da cobrança não decide de quem sai o desconto.
//
// Se decidisse, o cliente escolheria "establishment" no próprio pedido e o
// restaurante pagaria uma promoção da plataforma; e mandando um
// discount_amount alto ele inflaria o bruto sobre o qual as porcentagens
// incidem, aumentando a fatia do estabelecimento acima do que o pedido pagou.
func TestBindRecipient_DescontoVemDoPedidoNaoDoCorpo(t *testing.T) {
	teardown := setupCheckoutE2EEnv(t)
	defer teardown()
	createOrderDocumentsTable(t)

	const orderID = "ord-desc-bind"
	seedOrderComCupom(t, orderID, 90.00, 10.00, 10.00, "platform")

	// O cliente tenta mandar outro desconto e outro financiador. Os campos são
	// `json:"-"` no DTO, então nem chegariam pelo BodyParser — aqui são
	// preenchidos à mão para provar que bindRecipientToOrder sobrescreve
	// mesmo assim, e não só "por não ter sido lido".
	req := dto.PaymentRequest{
		OrderID:          orderID,
		Amount:           90.00,
		DiscountAmount:   80.00,
		DiscountFundedBy: "establishment",
	}

	// bindRecipientToOrder precisa de um *fiber.Ctx (lê o customer_id do
	// token). Sem token ele só não sobrescreve o customer_id; o resto do
	// binding — que é o que este teste mede — acontece igual.
	var ok bool
	app := fiber.New()
	app.Post("/bind", func(c *fiber.Ctx) error {
		ok = bindRecipientToOrder(c, &req)
		return c.SendStatus(200)
	})
	_, err := app.Test(httptest.NewRequest("POST", "/bind", nil), -1)
	require.NoError(t, err)
	require.True(t, ok)

	require.InDelta(t, 10.00, req.DiscountAmount, 0.001,
		"o desconto tem de vir do pedido (10,00), não do corpo (80,00)")
	require.Equal(t, "platform", req.DiscountFundedBy,
		"quem banca vem do pedido, não da escolha do cliente")
}

// Cupom que zera o valor dos produtos deixa total == frete. É uma venda
// legítima — o cliente paga só a entrega, o entregador recebe tudo, e quem
// bancou o cupom comeu a comida.
//
// Comparando o frete contra o total JÁ descontado, essa cobrança seria
// recusada e a promoção que alguém quis fazer não poderia ser paga. A guarda
// compara contra o bruto.
func TestResolveDeliveryAmount_CupomQueZeraOsProdutosNaoTravaACobranca(t *testing.T) {
	teardown := setupCheckoutE2EEnv(t)
	defer teardown()
	createOrderDocumentsTable(t)

	// Produtos 60, frete 11, cupom de 60: bruto 71, cliente paga 11.
	const orderID = "ord-cupom-total"
	seedOrderComCupom(t, orderID, 11.00, 11.00, 60.00, "establishment")

	got, ok := resolveDeliveryAmount(orderID, 11.00, 11.00)
	require.True(t, ok,
		"frete igual ao total descontado é legítimo quando o cupom comeu os produtos")
	require.InDelta(t, 11.00, got, 0.001)
}

// A guarda continua fechada onde sempre esteve: pedido SEM cupom com frete
// igual ao total é o vetor original (o split zera plataforma e loja e manda
// tudo para o entregador).
func TestResolveDeliveryAmount_SemCupomAGuardaContinuaFechada(t *testing.T) {
	teardown := setupCheckoutE2EEnv(t)
	defer teardown()
	createOrderDocumentsTable(t)

	const orderID = "ord-sem-cupom-frete-total"
	seedOrderDocument(t, orderID, 100.00, 100.00)

	if _, ok := resolveDeliveryAmount(orderID, 100.00, 100.00); ok {
		t.Fatal("sem cupom, frete igual ao total continua sendo recusado")
	}
}
