package repository

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreatePayment(payment *models.Payment) error {
	ctx := MongoCtx()
	payment.ID = primitive.NewObjectID()
	payment.CreatedAt = time.Now()
	payment.UpdatedAt = time.Now()
	_, err := Payments.InsertOne(ctx, payment)
	return err
}

func GetPaymentByID(id primitive.ObjectID) (*models.Payment, error) {
	ctx := MongoCtx()
	var payment models.Payment
	err := Payments.FindOne(ctx, bson.M{"_id": id}).Decode(&payment)
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func GetPaymentByOrderID(orderID string) (*models.Payment, error) {
	ctx := MongoCtx()
	var payment models.Payment
	err := Payments.FindOne(ctx, bson.M{"order_id": orderID}).Decode(&payment)
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func UpdatePaymentStatus(id primitive.ObjectID, status models.PaymentStatus, updates bson.M) error {
	ctx := MongoCtx()
	updates["status"] = status
	updates["updated_at"] = time.Now()
	_, err := Payments.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	return err
}

func ListPayments(filter models.PaymentFilter) ([]models.Payment, int64, error) {
	ctx := MongoCtx()
	query := bson.M{}

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

	total, err := Payments.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}

	skip := int64((page - 1) * limit)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

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

func GetPaymentStats() (map[string]interface{}, error) {
	ctx := MongoCtx()
	stats := map[string]interface{}{}

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

	highRisk, _ := Payments.CountDocuments(ctx, bson.M{"risk_level": bson.M{"$in": []string{"high", "critical"}}})
	stats["high_risk"] = highRisk

	return stats, nil
}
