package handlers

import (
	"context"
	"time"

	"github.com/carloshomar/vercardapio/payment_api/app/dto"
	"github.com/carloshomar/vercardapio/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetBalance(c *fiber.Ctx) error {
	userIDStr := c.Params("user_id")

	var wallet models.Wallet
	err := models.MongoDabase.Collection("wallets").FindOne(context.Background(), bson.M{"user_id": userIDStr}).Decode(&wallet)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Wallet not found", "balance": 0})
	}

	return c.Status(200).JSON(fiber.Map{
		"user_id":      wallet.UserID,
		"balance":      wallet.Balance,
		"last_updated": wallet.LastUpdated,
	})
}

func TopUp(c *fiber.Ctx) error {
	var req dto.WalletTopUpRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Amount must be greater than zero"})
	}

	filter := bson.M{"user_id": req.UserID}
	update := bson.M{
		"$inc":  bson.M{"balance": req.Amount},
		"$set":  bson.M{"last_updated": time.Now()},
		"$setOnInsert": bson.M{
			"_id":       primitive.NewObjectID(),
			"user_id":   req.UserID,
			"user_type": "customer",
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := models.MongoDabase.Collection("wallets").UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to top up wallet"})
	}

	var wallet models.Wallet
	models.MongoDabase.Collection("wallets").FindOne(context.Background(), filter).Decode(&wallet)

	return c.Status(200).JSON(fiber.Map{
		"user_id":      req.UserID,
		"balance":      wallet.Balance,
		"amount_added": req.Amount,
		"message":      "Wallet topped up successfully",
	})
}

func DeductFromWallet(c *fiber.Ctx) error {
	var req struct {
		UserID int64   `json:"user_id"`
		Amount float64 `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Amount must be greater than zero"})
	}

	var wallet models.Wallet
	err := models.MongoDabase.Collection("wallets").FindOne(context.Background(), bson.M{"user_id": req.UserID}).Decode(&wallet)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Wallet not found"})
	}

	if wallet.Balance < req.Amount {
		return c.Status(400).JSON(fiber.Map{"error": "Insufficient balance"})
	}

	_, err = models.MongoDabase.Collection("wallets").UpdateOne(
		context.Background(),
		bson.M{"user_id": req.UserID},
		bson.M{
			"$inc":  bson.M{"balance": -req.Amount},
			"$set":  bson.M{"last_updated": time.Now()},
		},
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to deduct from wallet"})
	}

	models.MongoDabase.Collection("wallets").FindOne(context.Background(), bson.M{"user_id": req.UserID}).Decode(&wallet)

	return c.Status(200).JSON(fiber.Map{
		"user_id":         req.UserID,
		"balance":         wallet.Balance,
		"amount_deducted": req.Amount,
		"message":         "Amount deducted successfully",
	})
}
