package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
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
	findErr := models.MongoDabase.Collection("wallets").FindOne(mongoCtx(), bson.M{"user_id": userIDStr}).Decode(&wallet)
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
		mongoCtx(),
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
	_, err = models.MongoDabase.Collection("wallets").UpdateOne(mongoCtx(), filter, update, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to top up wallet"})
	}

	now := time.Now()
	models.MongoDabase.Collection("payments").UpdateOne(
		mongoCtx(),
		bson.M{"abacatepay_id": req.PaymentID},
		bson.M{"$set": bson.M{"wallet_credited_at": now}},
	)

	var wallet models.Wallet
	models.MongoDabase.Collection("wallets").FindOne(mongoCtx(), filter).Decode(&wallet)

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
	if _, ledgerErr := models.MongoDabase.Collection("wallet_ledger").InsertOne(mongoCtx(), ledgerEntry); ledgerErr != nil {
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
		mongoCtx(),
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
	models.MongoDabase.Collection("wallets").FindOne(mongoCtx(), bson.M{"user_id": req.UserID}).Decode(&wallet)

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
	if _, ledgerErr := models.MongoDabase.Collection("wallet_ledger").InsertOne(mongoCtx(), ledgerEntry); ledgerErr != nil {
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

// establishmentLedgerTotals soma os créditos (total ganho) e os débitos de
// saque (kind == "withdrawal") do wallet_ledger de um estabelecimento.
// A carteira do restaurante é identificada pelo establishment_id (user_id).
func establishmentLedgerTotals(estID int64) (earned, withdrawn float64) {
	ctx := mongoCtx()
	ledger := models.MongoDabase.Collection("wallet_ledger")

	// Total ganho = soma dos créditos
	cursor, err := ledger.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"user_id": estID}},
		{"$group": bson.M{
			"_id":   "$type",
			"total": bson.M{"$sum": "$amount"},
		}},
	})
	if err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var row struct {
				ID    string  `bson:"_id"`
				Total float64 `bson:"total"`
			}
			if cursor.Decode(&row) == nil && row.ID == "credit" {
				earned += row.Total
			}
		}
	}

	// Total sacado = soma dos débitos marcados como saque (kind=withdrawal)
	cursor2, err := ledger.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"user_id": estID, "kind": "withdrawal"}},
		{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": "$amount"},
		}},
	})
	if err == nil {
		defer cursor2.Close(ctx)
		for cursor2.Next(ctx) {
			var row struct {
				Total float64 `bson:"total"`
			}
			if cursor2.Decode(&row) == nil {
				withdrawn += row.Total
			}
		}
	}
	return
}

// GetEstablishmentWallet retorna o saldo da carteira do estabelecimento
// autenticado (papel restaurante no WebRestaurant).
// GET /wallet/establishment/balance
//
// A carteira do restaurante é creditada pelo split quando um pagamento é
// confirmado (user_id = establishment_id). pending/blocked são 0: o modelo
// atual só tem saldo disponível; total_earned/total_withdrawn vêm do ledger.
func GetEstablishmentWallet(c *fiber.Ctx) error {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Establishment ID not found in token"})
	}

	var wallet models.Wallet
	findErr := models.MongoDabase.Collection("wallets").FindOne(mongoCtx(), bson.M{"user_id": estID}).Decode(&wallet)

	available := 0.0
	if findErr == nil {
		available = wallet.Balance
	}

	earned, withdrawn := establishmentLedgerTotals(estID)

	return c.JSON(fiber.Map{
		"user_id":         estID,
		"available":       available,
		"pending":         0.0,
		"blocked":         0.0,
		"total_earned":    earned,
		"total_withdrawn": withdrawn,
		"last_updated":    wallet.LastUpdated,
	})
}

// GetEstablishmentTransactions lista o extrato (wallet_ledger) do
// estabelecimento autenticado, paginado por cursor (ObjectID hex).
// GET /wallet/establishment/transactions?limit=20&cursor=...
//
// Retorna { data, next_cursor, has_more }. Cada item expõe type em
// MAIÚSCULAS (CREDIT/DEBIT/WITHDRAWAL) para o frontend colorir o extrato.
func GetEstablishmentTransactions(c *fiber.Ctx) error {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Establishment ID not found in token"})
	}

	limit := 20
	if l := c.QueryInt("limit"); l > 0 && l <= 100 {
		limit = l
	}

	query := bson.M{"user_id": estID}
	if cursorHex := c.Query("cursor"); cursorHex != "" {
		oid, oidErr := primitive.ObjectIDFromHex(cursorHex)
		if oidErr != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid cursor"})
		}
		query["_id"] = bson.M{"$lt": oid}
	}

	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(limit + 1))
	cursor, err := models.MongoDabase.Collection("wallet_ledger").Find(mongoCtx(), query, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list transactions"})
	}
	defer cursor.Close(mongoCtx())

	var entries []bson.M
	if err := cursor.All(mongoCtx(), &entries); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode transactions"})
	}

	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}

	data := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		item := map[string]interface{}{}
		if oid, ok := e["_id"].(primitive.ObjectID); ok {
			item["id"] = oid.Hex()
		}

		typ, _ := e["type"].(string)
		if kind, ok := e["kind"].(string); ok && kind == "withdrawal" {
			item["type"] = "WITHDRAWAL"
		} else if typ == "credit" {
			item["type"] = "CREDIT"
		} else {
			item["type"] = "DEBIT"
		}

		if desc, ok := e["description"].(string); ok {
			item["description"] = desc
		}
		if amt, ok := e["amount"].(float64); ok {
			item["amount"] = amt
		}
		if bal, ok := e["balance_after"].(float64); ok {
			item["balance"] = bal
		}
		if ts, ok := e["created_at"].(time.Time); ok {
			item["created_at"] = ts.Format(time.RFC3339)
		}

		ref := ""
		if pid, ok := e["payment_id"].(string); ok {
			ref = pid
		} else if oid, ok := e["order_id"].(string); ok {
			ref = oid
		}
		item["payment_ref"] = ref

		data = append(data, item)
	}

	nextCursor := ""
	if hasMore {
		if last, ok := entries[len(entries)-1]["_id"].(primitive.ObjectID); ok {
			nextCursor = last.Hex()
		}
	}

	return c.JSON(fiber.Map{
		"data":        data,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}

// EstablishmentWithdraw processa um saque da carteira do estabelecimento
// autenticado (papel restaurante). Débito atômico com guarda de saldo e
// lançamento no wallet_ledger (type=debit, kind=withdrawal).
// POST /wallet/establishment/withdraw  body: {amount, destination, method}
func EstablishmentWithdraw(c *fiber.Ctx) error {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Establishment ID not found in token"})
	}

	var req struct {
		Amount      float64 `json:"amount"`
		Destination string  `json:"destination"`
		Method      string  `json:"method"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount < 10.0 {
		return c.Status(400).JSON(fiber.Map{"error": "Valor mínimo para saque: R$ 10,00"})
	}
	if req.Destination == "" || len(req.Destination) < 10 {
		return c.Status(400).JSON(fiber.Map{"error": "Informe uma chave PIX ou dados bancários válidos"})
	}
	if req.Method == "" {
		req.Method = "PIX"
	}

	wallets := models.MongoDabase.Collection("wallets")
	res, err := wallets.UpdateOne(
		mongoCtx(),
		bson.M{"user_id": estID, "balance": bson.M{"$gte": req.Amount}},
		bson.M{
			"$inc": bson.M{"balance": -req.Amount},
			"$set": bson.M{"last_updated": time.Now()},
		},
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Falha ao processar saque"})
	}
	if res.ModifiedCount == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Saldo insuficiente para este saque"})
	}

	var wallet models.Wallet
	wallets.FindOne(mongoCtx(), bson.M{"user_id": estID}).Decode(&wallet)

	now := time.Now()
	ledgerEntry := bson.M{
		"_id":           primitive.NewObjectID(),
		"user_id":       estID,
		"type":          "debit",
		"kind":          "withdrawal",
		"amount":        req.Amount,
		"balance_after": wallet.Balance,
		"description":   fmt.Sprintf("Saque via %s para %s", req.Method, req.Destination),
		"destination":   req.Destination,
		"created_at":    now,
	}
	if _, ledgerErr := models.MongoDabase.Collection("wallet_ledger").InsertOne(mongoCtx(), ledgerEntry); ledgerErr != nil {
		log.Printf("[WALLET] WARNING: falha ao gravar ledger do saque establishment=%d: %v", estID, ledgerErr)
	}

	log.Printf("[WALLET] Saque OK: establishment=%d amount=%.2f method=%s novo_saldo=%.2f", estID, req.Amount, req.Method, wallet.Balance)

	return c.JSON(fiber.Map{
		"message": "Saque solicitado com sucesso",
		"balance": wallet.Balance,
	})
}
