// Package repository - payment_repo.go
// Funcoes de acesso a dados para a colecao de pagamentos.
// Cada funcao implementa uma operacao MongoDB (CRUD + consultas).
package repository

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreatePayment insere um novo pagamento no MongoDB.
// Gera automaticamente o ObjectID e os timestamps CreatedAt/UpdatedAt.
func CreatePayment(payment *models.Payment) error {
	ctx := MongoCtx()
	payment.ID = primitive.NewObjectID()
	payment.CreatedAt = time.Now()
	payment.UpdatedAt = time.Now()
	_, err := Payments.InsertOne(ctx, payment)
	return err
}

// GetPaymentByID busca um pagamento pelo seu ObjectID.
// Retorna erro se nao encontrar o documento.
func GetPaymentByID(id primitive.ObjectID) (*models.Payment, error) {
	ctx := MongoCtx()
	var payment models.Payment
	err := Payments.FindOne(ctx, bson.M{"_id": id}).Decode(&payment)
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// GetPaymentByOrderID busca um pagamento pelo ID do pedido.
// Usado para evitar pagamentos duplicados para o mesmo pedido.
func GetPaymentByOrderID(orderID string) (*models.Payment, error) {
	ctx := MongoCtx()
	var payment models.Payment
	err := Payments.FindOne(ctx, bson.M{"order_id": orderID}).Decode(&payment)
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// UpdatePaymentStatus atualiza o status de um pagamento e campos adicionais.
// O parameter updates permite enviar campos dinamicos (approved_by, rejection_reason, etc).
func UpdatePaymentStatus(id primitive.ObjectID, status models.PaymentStatus, updates bson.M) error {
	ctx := MongoCtx()
	updates["status"] = status
	updates["updated_at"] = time.Now()
	_, err := Payments.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	return err
}

// ListPayments lista pagamentos com filtros, paginacao e ordenacao.
// Retorna a lista de pagamentos, total de resultados e erro (se houver).
// A paginacao e controlada pelos campos Page e Limit do PaymentFilter.
func ListPayments(filter models.PaymentFilter) ([]models.Payment, int64, error) {
	ctx := MongoCtx()
	query := bson.M{}

	// Adiciona filtros apenas se os valores forem fornecidos
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.RiskLevel != "" {
		query["risk_level"] = filter.RiskLevel
	}
	if filter.EstablishmentID != "" {
		query["establishment_id"] = filter.EstablishmentID
	}
	if filter.CustomerID != "" {
		query["customer_id"] = filter.CustomerID
	}
	if filter.Method != "" {
		query["method"] = filter.Method
	}

	// Conta total de documentos para paginacao
	total, err := Payments.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	// Configura paginacao (default: pagina 1, 20 itens)
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Calcula offset e configura opcoes de busca
	skip := int64((page - 1) * limit)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}). // Mais recentes primeiro
		SetSkip(skip).
		SetLimit(int64(limit))

	// Executa a busca e decodifica os resultados
	cursor, err := Payments.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var payments []models.Payment
	if err := cursor.All(ctx, &payments); err != nil {
		return nil, 0, err
	}

	return payments, total, nil
}

// GetPaymentStats retorna estatisticas gerais dos pagamentos.
// Conta pagamentos por status e nivel de risco para exibicao no dashboard.
func GetPaymentStats() (map[string]interface{}, error) {
	ctx := MongoCtx()
	stats := map[string]interface{}{}

	// Conta por status
	total, _ := Payments.CountDocuments(ctx, bson.M{})
	stats["total"] = total

	pending, _ := Payments.CountDocuments(ctx, bson.M{"status": "pending"})
	stats["pending"] = pending

	approved, _ := Payments.CountDocuments(ctx, bson.M{"status": "approved"})
	stats["approved"] = approved

	rejected, _ := Payments.CountDocuments(ctx, bson.M{"status": "rejected"})
	stats["rejected"] = rejected

	disputed, _ := Payments.CountDocuments(ctx, bson.M{"status": "disputed"})
	stats["disputed"] = disputed

	// Conta pagamentos de alto risco (high + critical)
	highRisk, _ := Payments.CountDocuments(ctx, bson.M{"risk_level": bson.M{"$in": []string{"high", "critical"}}})
	stats["high_risk"] = highRisk

	return stats, nil
}
