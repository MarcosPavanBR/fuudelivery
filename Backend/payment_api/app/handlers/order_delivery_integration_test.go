//go:build integration

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Frete do pedido lido do Postgres (caminho com deliveryValue gravado).
//
// Por que integração e não unitário: os testes unitários de
// resolveDeliveryAmount rodam com models.DB == nil, então lookupOrderDelivery
// devolve false e eles só exercitam o ramo de PEDIDO LEGADO. Um erro no nome
// da chave JSON (`deliveryValue`) ou na query cairia silenciosamente nesse
// mesmo ramo e os unitários continuariam verdes — a validação contra o valor
// registrado no pedido estaria desligada sem ninguém perceber.
//
// Como rodar:
//
//	go test -tags=integration -run 'TestResolveDeliveryAmount_ComPedido' ./app/handlers/
// ============================================================================

// createOrderDocumentsTable cria só o que lookupOrderDelivery consulta.
// setupCheckoutE2EEnv faz AutoMigrate apenas das tabelas do domínio de
// pagamentos; order_documents pertence ao orders_api.
func createOrderDocumentsTable(t *testing.T) {
	t.Helper()
	require.NoError(t, models.DB.Exec(`DROP TABLE IF EXISTS order_documents CASCADE`).Error)
	require.NoError(t, models.DB.Exec(`
		CREATE TABLE order_documents (
			id               BIGSERIAL PRIMARY KEY,
			legacy_id        VARCHAR(32) UNIQUE,
			establishment_id BIGINT,
			user_phone       VARCHAR(32),
			payload          JSONB
		)`).Error)
}

// seedOrderDocument grava um pedido com total e frete, como o orders_api faz
// (payload = dto.RequestPayload serializado inteiro).
func seedOrderDocument(t *testing.T, legacyID string, orderTotal, deliveryValue float64) {
	t.Helper()
	require.NoError(t, models.DB.Exec(
		`INSERT INTO order_documents (legacy_id, payload)
		 VALUES (?, jsonb_build_object('order_total', ?::float8, 'deliveryValue', ?::float8))`,
		legacyID, orderTotal, deliveryValue).Error)
}

func TestResolveDeliveryAmount_ComPedidoGravado(t *testing.T) {
	teardown := setupCheckoutE2EEnv(t)
	defer teardown()
	createOrderDocumentsTable(t)

	const orderID = "ord-frete-gravado"
	seedOrderDocument(t, orderID, 100.00, 7.00)

	t.Run("frete do cliente confere com o do pedido", func(t *testing.T) {
		got, ok := resolveDeliveryAmount(orderID, 7.00, 100.00)
		require.True(t, ok, "frete idêntico ao do pedido deve ser aceito")
		require.InDelta(t, 7.00, got, 0.001)
	})

	// O ponto do teste: mesmo ABAIXO do total (portanto passando na guarda de
	// sanidade do ramo legado), um frete que não é o do pedido é recusado.
	// É isto que prova que a query realmente leu o pedido.
	t.Run("frete inflado mas abaixo do total é recusado", func(t *testing.T) {
		if _, ok := resolveDeliveryAmount(orderID, 40.00, 100.00); ok {
			t.Fatal("frete de 40,00 num pedido que registrou 7,00 deveria ser recusado")
		}
	})

	t.Run("o valor gravado vence o do cliente", func(t *testing.T) {
		got, ok := resolveDeliveryAmount(orderID, 7.004, 100.00)
		require.True(t, ok, "diferença sub-centavo entra na tolerância")
		require.InDelta(t, 7.00, got, 0.001, "deve gravar o frete do pedido, não o do cliente")
	})

	t.Run("pedido sem deliveryValue cai na guarda de sanidade", func(t *testing.T) {
		const legacyOrder = "ord-sem-frete"
		require.NoError(t, models.DB.Exec(
			`INSERT INTO order_documents (legacy_id, payload)
			 VALUES (?, jsonb_build_object('order_total', 100.00::float8))`, legacyOrder).Error)

		got, ok := resolveDeliveryAmount(legacyOrder, 7.00, 100.00)
		require.True(t, ok, "pedido legado aceita o frete do cliente")
		require.InDelta(t, 7.00, got, 0.001)

		if _, ok := resolveDeliveryAmount(legacyOrder, 100.00, 100.00); ok {
			t.Fatal("mesmo em pedido legado, frete que engole o total deve ser recusado")
		}
	})
}

// ============================================================================
// Destinatário da cobrança vem do PEDIDO, não do corpo da requisição.
//
// Achado do security-reviewer, severidade alta: `amount` e `delivery_amount`
// já eram reconferidos contra o pedido, mas `establishment_id` não — e era
// exatamente ele que decidia qual carteira o webhook creditava
// (adjustEstablishmentWallet). Como POST /establishments/register é aberto,
// dava para registrar um estabelecimento próprio, fazer um pedido de verdade
// num restaurante alheio e mandar `establishment_id` apontando para si: o
// restaurante entregava a comida e o split caía na carteira do atacante,
// sacável por EstablishmentWithdraw.
// ============================================================================

func TestBindRecipientToOrder_IgnoraEstabelecimentoDoCorpo(t *testing.T) {
	teardown := setupCheckoutE2EEnv(t)
	defer teardown()
	createOrderDocumentsTable(t)

	const (
		orderID          = "ord-destinatario"
		vitimaEstID      = int64(4001) // restaurante que realmente vendeu
		atacanteEstID    = int64(9999) // estabelecimento registrado pelo atacante
		telefoneDoPedido = "+5511999900001"
	)
	require.NoError(t, models.DB.Exec(
		`INSERT INTO order_documents (legacy_id, establishment_id, user_phone, payload)
		 VALUES (?, ?, ?, jsonb_build_object('order_total', 100.00::float8, 'deliveryValue', 7.00::float8))`,
		orderID, vitimaEstID, telefoneDoPedido).Error)

	app := fiber.New()
	app.Post("/bind", func(c *fiber.Ctx) error {
		var req dto.PaymentRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "corpo inválido"})
		}
		if !bindRecipientToOrder(c, &req) {
			return c.Status(400).JSON(fiber.Map{"error": "pedido inválido"})
		}
		return c.JSON(fiber.Map{
			"establishment_id": req.EstablishmentID,
			"customer_phone":   req.CustomerPhone,
		})
	})

	call := func(body string) map[string]interface{} {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/bind", bytesReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		var out map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		return out
	}

	t.Run("establishment_id do atacante é descartado", func(t *testing.T) {
		out := call(fmt.Sprintf(
			`{"order_id":"%s","amount":100.00,"delivery_amount":7.00,"establishment_id":%d}`,
			orderID, atacanteEstID))
		require.EqualValues(t, vitimaEstID, out["establishment_id"],
			"o split tem que ir para o estabelecimento do pedido, nunca para o do corpo")
	})

	t.Run("customer_phone também vem do pedido", func(t *testing.T) {
		out := call(fmt.Sprintf(
			`{"order_id":"%s","amount":100.00,"customer_phone":"+5511000000000"}`, orderID))
		require.Equal(t, telefoneDoPedido, out["customer_phone"],
			"o ACL de leitura (canViewOrderPayment) não pode ser escolhido pelo chamador")
	})

	t.Run("pedido sem estabelecimento não vira cobrança", func(t *testing.T) {
		require.NoError(t, models.DB.Exec(
			`INSERT INTO order_documents (legacy_id, payload)
			 VALUES ('ord-sem-est', jsonb_build_object('order_total', 50.00::float8))`).Error)

		req := httptest.NewRequest(http.MethodPost, "/bind",
			bytesReader([]byte(`{"order_id":"ord-sem-est","amount":50.00,"establishment_id":9999}`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 400, resp.StatusCode)
	})
}
