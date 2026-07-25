// Package repository - idempotency_repo.go
// Trava atomica de idempotencia para o processamento de pagamentos.
//
// Por que isso existe: TransactionExistsByReference (em wallet_repo.go) faz
// "contar quantas transacoes existem, depois decidir" — sao duas operacoes
// separadas. Se duas chamadas para o MESMO order_id chegarem ao mesmo tempo
// (duas instancias do Payment Service consumindo a fila, ou uma reentrega de
// mensagem do RabbitMQ, que usa auto-ack e nao tem dedupe nativo), as duas
// podem ver "nao existe ainda" ANTES de qualquer uma escrever, e as duas
// creditam a carteira. ClaimOrderProcessing resolve isso fazendo a checagem
// e a reserva na MESMA operacao atomica do banco.
package repository

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ErrOrderAlreadyProcessed nao e uma falha — e o sinal de que este
// order_id ja passou por ClaimOrderProcessing antes. Quem chama deve
// tratar isso como "pular o credito", nao como erro de verdade.
var ErrOrderAlreadyProcessed = errors.New("order already processed")

// ClaimOrderProcessing reserva atomicamente o direito de processar um
// order_id. Insere um documento com _id = orderID: o _id do MongoDB e
// unico por natureza (indice implicito), entao a PRIMEIRA chamada pra um
// order_id insere com sucesso, e qualquer chamada seguinte para o MESMO
// order_id esbarra na duplicidade e recebe ErrOrderAlreadyProcessed —
// mesmo que as duas tenham chegado no mesmo milissegundo, o Mongo garante
// que so uma das duas insercoes vence.
func ClaimOrderProcessing(orderID string) error {
	ctx := MongoCtx()
	_, err := ProcessedOrders.InsertOne(ctx, bson.M{
		"_id":          orderID,
		"processed_at": time.Now(),
	})
	if mongo.IsDuplicateKeyError(err) {
		return ErrOrderAlreadyProcessed
	}
	return err
}
