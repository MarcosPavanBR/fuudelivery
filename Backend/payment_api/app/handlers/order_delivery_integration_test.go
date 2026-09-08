//go:build integration

package handlers

import (
	"testing"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
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
			id        BIGSERIAL PRIMARY KEY,
			legacy_id VARCHAR(32) UNIQUE,
			payload   JSONB
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
