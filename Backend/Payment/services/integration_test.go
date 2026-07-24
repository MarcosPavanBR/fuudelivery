//go:build integration

// Teste de integração do fluxo: pagamento aprovado -> crédito na carteira
// do restaurante.
//
// No sistema real esse fluxo cruza dois serviços via RabbitMQ:
//
//	payment_api (webhook.go, publishPaymentApproved) --publica--> fila "payments"
//	Payment/consumers/payment_consumer.go             --consome--> WalletService.ProcessPaymentApproval
//
// Esse teste NÃO sobe RabbitMQ nem o processo do payment_api — isso é
// integração *entre processos* e fica melhor coberto por um teste E2E
// (ver Fase 3/4 do plano de testes). Aqui testamos com MongoDB real a parte
// que já dá pra pegar bug de verdade sem precisar de infra pesada: dado um
// Payment com status "approved" já salvo no banco (como o consumer decodifica
// da fila), será que ProcessPaymentApproval credita o valor líquido certo,
// registra a transação e marca wallet_credited_at?
//
// Rodar com:
//
//	go test -tags=integration ./services/... -run TestPaymentApproval -v
//
// Pré-requisito: adicionar ao go.mod (rodar com rede liberada p/ proxy.golang.org):
//
//	go get github.com/testcontainers/testcontainers-go
//	go get github.com/testcontainers/testcontainers-go/modules/mongodb
package services

import (
	"context"
	"testing"
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// setupPaymentIntegrationEnv sobe um MongoDB real em container e aponta as
// coleções globais de repository/mongo.go (Payments, Wallets, WalletTransactions)
// pra esse banco de teste. Devolve uma func de cleanup.
func setupPaymentIntegrationEnv(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()

	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err, "subir container do MongoDB")

	uri, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.NoError(t, err)
	require.NoError(t, client.Ping(ctx, nil))

	db := client.Database("payment_test")
	repository.Client = client
	repository.Database = db
	repository.Payments = db.Collection("payments")
	repository.Wallets = db.Collection("wallets")
	repository.WalletTransactions = db.Collection("wallet_transactions")
	repository.Chargebacks = db.Collection("chargebacks")
	repository.Evidences = db.Collection("evidences")
	repository.Users = db.Collection("users")

	return func() {
		_ = client.Disconnect(ctx)
		_ = mongoContainer.Terminate(ctx)
	}
}

// TestPaymentApproval_CreditsNetAmountToWallet cobre o caminho feliz:
// pagamento de R$100 com R$10 de taxa de entrega -> restaurante recebe R$90
// e a transação fica registrada.
//
// TODO (casos que faltam além do caminho feliz):
//   - Chamar ProcessPaymentApproval duas vezes com o mesmo Payment não deve
//     creditar em dobro (hoje não há checagem de idempotência — vale confirmar
//     se esse é o comportamento esperado ou se é bug a corrigir).
//   - Payment com status diferente de "approved" (ex: "pending", "rejected")
//     deve retornar nil sem tocar na carteira (já há teste unitário disso em
//     wallet_service_test.go — aqui a diferença é usar Mongo de verdade).
//   - amount - delivery_amount negativo ou zero: deveria falhar antes de chegar
//     aqui? Confirmar contrato com quem gera a mensagem da fila.
//   - Duas aprovações concorrentes (goroutines) para o mesmo establishment_id
//     não podem perder incremento — é o cenário que o $inc atômico existe pra
//     resolver, vale um teste de carga leve (ex: 20 goroutines, 1 real).
func TestPaymentApproval_CreditsNetAmountToWallet(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	establishmentID := "establishment-123"

	payment := &models.Payment{
		OrderID:         "order-1",
		EstablishmentID: establishmentID,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Status:          models.PaymentApproved,
		Method:          models.PaymentMethodPix,
		CreatedAt:       time.Now(),
	}
	require.NoError(t, repository.CreatePayment(payment))

	ws := NewWalletService()
	err := ws.ProcessPaymentApproval(payment)
	require.NoError(t, err)

	wallet, err := repository.GetWallet(establishmentID)
	require.NoError(t, err, "carteira deveria ter sido criada automaticamente no primeiro crédito")
	require.Equal(t, 90.00, wallet.Balance, "valor líquido = amount - delivery_amount")

	txs, err := ws.GetTransactions(establishmentID, 10)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, "order-1", txs[0].ReferenceID)

	updated, err := repository.GetPaymentByID(payment.ID)
	require.NoError(t, err)
	require.NotNil(t, updated, "payment deveria existir após o update")
}
