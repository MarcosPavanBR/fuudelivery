package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/vercardapio/auth_api/app/middlewares"
	"github.com/carloshomar/vercardapio/payment_api/app/dto"
	"github.com/carloshomar/vercardapio/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetBalance(c *fiber.Ctx) error {
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	userIDStr := c.Params("user_id")

	var reqUserID int64
	if _, scanErr := fmt.Sscanf(userIDStr, "%d", &reqUserID); scanErr != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid user_id"})
	}

	if tokenUserID != reqUserID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot view another user's balance"})
	}

	var wallet models.Wallet
	findErr := models.MongoDabase.Collection("wallets").FindOne(context.Background(), bson.M{"user_id": userIDStr}).Decode(&wallet)
	if findErr != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Wallet not found", "balance": 0})
	}

	return c.Status(200).JSON(fiber.Map{
		"user_id":      wallet.UserID,
		"balance":      wallet.Balance,
		"last_updated": wallet.LastUpdated,
	})
}

func TopUp(c *fiber.Ctx) error {
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var req dto.WalletTopUpRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Amount must be greater than zero"})
	}

	if req.PaymentID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "payment_id is required for wallet top-up"})
	}

	if tokenUserID != req.UserID {
		log.Printf("[WALLET] TopUp rejected: token user %d != body user %d", tokenUserID, req.UserID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot top up another user's wallet"})
	}

	var payment models.Payment
	err = models.MongoDabase.Collection("payments").FindOne(
		context.Background(),
		bson.M{"abacatepay_id": req.PaymentID},
	).Decode(&payment)
	if err != nil {
		log.Printf("[WALLET] TopUp rejected: payment %s not found (user=%d)", req.PaymentID, req.UserID)
		return c.Status(404).JSON(fiber.Map{"error": "Payment not found"})
	}

	if payment.Status != "CONFIRMED" {
		log.Printf("[WALLET] TopUp rejected: payment %s status=%s, expected CONFIRMED", req.PaymentID, payment.Status)
		return c.Status(402).JSON(fiber.Map{"error": "Payment not confirmed", "status": payment.Status})
	}

	if payment.CustomerID != req.UserID {
		log.Printf("[WALLET] TopUp rejected: payment %s belongs to user %d, requested by user %d", req.PaymentID, payment.CustomerID, req.UserID)
		return c.Status(403).JSON(fiber.Map{"error": "Payment does not belong to this user"})
	}

	if payment.WalletCreditedAt != nil {
		log.Printf("[WALLET] TopUp rejected: payment %s already used for wallet credit at %v", req.PaymentID, payment.WalletCreditedAt)
		return c.Status(409).JSON(fiber.Map{"error": "Payment already used for wallet top-up"})
	}

	amountToCredit := payment.Amount

	filter := bson.M{"user_id": req.UserID}
	update := bson.M{
		"$inc": bson.M{"balance": amountToCredit},
		"$set": bson.M{"last_updated": time.Now()},
		"$setOnInsert": bson.M{
			"_id":       primitive.NewObjectID(),
			"user_id":   req.UserID,
			"user_type": "customer",
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err = models.MongoDabase.Collection("wallets").UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to top up wallet"})
	}

	now := time.Now()
	models.MongoDabase.Collection("payments").UpdateOne(
		context.Background(),
		bson.M{"abacatepay_id": req.PaymentID},
		bson.M{"$set": bson.M{"wallet_credited_at": now}},
	)

	var wallet models.Wallet
	models.MongoDabase.Collection("wallets").FindOne(context.Background(), filter).Decode(&wallet)

	ledgerEntry := bson.M{
		"_id":           primitive.NewObjectID(),
		"user_id":       req.UserID,
		"type":          "credit",
		"amount":        amountToCredit,
		"payment_id":    req.PaymentID,
		"balance_after": wallet.Balance,
		"description":   "Wallet top-up via confirmed payment",
		"created_at":    time.Now(),
	}
	if _, ledgerErr := models.MongoDabase.Collection("wallet_ledger").InsertOne(context.Background(), ledgerEntry); ledgerErr != nil {
		log.Printf("[WALLET] WARNING: Failed to write ledger for user=%d: %v", req.UserID, ledgerErr)
	}

	log.Printf("[WALLET] TopUp OK: user=%d amount=%.2f payment=%s new_balance=%.2f", req.UserID, amountToCredit, req.PaymentID, wallet.Balance)

	return c.Status(200).JSON(fiber.Map{
		"user_id":      req.UserID,
		"balance":      wallet.Balance,
		"amount_added": amountToCredit,
		"message":      "Wallet topped up successfully",
	})
}

func DeductFromWallet(c *fiber.Ctx) error {
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	var req struct {
		UserID  int64   `json:"user_id"`
		Amount  float64 `json:"amount"`
		OrderID string  `json:"order_id,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Amount must be greater than zero"})
	}

	if tokenUserID != req.UserID {
		log.Printf("[WALLET] Deduct rejected: token user %d != body user %d", tokenUserID, req.UserID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot deduct from another user's wallet"})
	}

	result, err := models.MongoDabase.Collection("wallets").UpdateOne(
		context.Background(),
		bson.M{
			"user_id": req.UserID,
			"balance": bson.M{"$gte": req.Amount},
		},
		bson.M{
			"$inc": bson.M{"balance": -req.Amount},
			"$set": bson.M{"last_updated": time.Now()},
		},
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to deduct from wallet"})
	}

	if result.ModifiedCount == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Insufficient balance or wallet not found"})
	}

	var wallet models.Wallet
	models.MongoDabase.Collection("wallets").FindOne(context.Background(), bson.M{"user_id": req.UserID}).Decode(&wallet)

	ledgerEntry := bson.M{
		"_id":           primitive.NewObjectID(),
		"user_id":       req.UserID,
		"type":          "debit",
		"amount":        req.Amount,
		"order_id":      req.OrderID,
		"balance_after": wallet.Balance,
		"description":   "Wallet deduction",
		"created_at":    time.Now(),
	}
	if _, ledgerErr := models.MongoDabase.Collection("wallet_ledger").InsertOne(context.Background(), ledgerEntry); ledgerErr != nil {
		log.Printf("[WALLET] WARNING: Failed to write ledger for user=%d: %v", req.UserID, ledgerErr)
	}

	log.Printf("[WALLET] Deduct OK: user=%d amount=%.2f new_balance=%.2f", req.UserID, req.Amount, wallet.Balance)

	return c.Status(200).JSON(fiber.Map{
		"user_id":         req.UserID,
		"balance":         wallet.Balance,
		"amount_deducted": req.Amount,
		"message":         "Amount deducted successfully",
	})
}
