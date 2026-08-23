package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// ============================================================================
// Carteiras — corte 4: todas as movimentações passam por
// models.AdjustWalletBalance (transação + SELECT FOR UPDATE + ledger),
// garantindo atomicidade que o Mongo legado não tinha. Saldo legado do
// Mongo é semeado na primeira movimentação (ensureWalletSeeded).
// ============================================================================

// GetBalance retorna o saldo da carteira do próprio usuário autenticado.
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

	wallet, wErr := ensureWalletSeeded(models.DB, reqUserID, "customer")
	if wErr != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Wallet not found", "balance": 0})
	}

	return c.Status(200).JSON(fiber.Map{
		"user_id":      wallet.UserID,
		"balance":      wallet.Balance,
		"last_updated": wallet.LastUpdated,
	})
}

// TopUp credita na carteira do cliente o valor de um pagamento CONFIRMADO.
// Idempotente: wallet_credited_at no pagamento + checagem prévia no ledger.
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

	payment, pErr := findPaymentByAbacatePayID(req.PaymentID)
	if pErr != nil {
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

	if payment.WalletCreditedAt != nil || models.HasLedgerEntry(models.DB, req.PaymentID, "credit") {
		log.Printf("[WALLET] TopUp rejected: payment %s already used for wallet credit", req.PaymentID)
		return c.Status(409).JSON(fiber.Map{"error": "Payment already used for wallet top-up"})
	}

	wallet, tErr := ensureWalletSeeded(models.DB, req.UserID, "customer")
	if tErr != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to top up wallet"})
	}

	newWallet, aErr := models.AdjustWalletBalance(
		models.DB, req.UserID, "customer",
		"credit", "", payment.Amount,
		req.PaymentID,
		"Wallet top-up via confirmed payment",
		"",
	)
	if aErr != nil {
		log.Printf("[WALLET] TopUp failed: user=%d payment=%s: %v", req.UserID, req.PaymentID, aErr)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to top up wallet"})
	}
	wallet = newWallet

	// Marca o pagamento como já usado para crédito (idempotência).
	now := time.Now()
	if uErr := models.DB.Model(payment).Update("wallet_credited_at", now).Error; uErr != nil {
		log.Printf("[WALLET] WARNING: falha ao marcar wallet_credited_at do pagamento %s: %v", req.PaymentID, uErr)
	} else {
		dualWritePaymentUpsert(payment) // DUAL-WRITE LEGADO
	}

	dualWriteLedgerEntry(req.UserID, "credit", "", payment.Amount, wallet.Balance, req.PaymentID,
		"Wallet top-up via confirmed payment", "") // DUAL-WRITE LEGADO
	dualWriteWallet(wallet) // DUAL-WRITE LEGADO

	log.Printf("[WALLET] TopUp OK: user=%d amount=%.2f payment=%s new_balance=%.2f", req.UserID, payment.Amount, req.PaymentID, wallet.Balance)

	return c.Status(200).JSON(fiber.Map{
		"user_id":      req.UserID,
		"balance":      wallet.Balance,
		"amount_added": payment.Amount,
		"message":      "Wallet topped up successfully",
	})
}

// DeductFromWallet debita um valor da carteira do usuário para pagar pedido.
// Atômico com guarda de saldo (nunca fica negativo).
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

	walletType := walletTypeForUser(req.UserID)
	newWallet, dErr := models.AdjustWalletBalance(
		models.DB, req.UserID, walletType,
		"debit", "", req.Amount,
		req.OrderID,
		"Wallet deduction",
		"",
	)
	if dErr == models.ErrInsufficientBalance {
		return c.Status(400).JSON(fiber.Map{"error": "Insufficient balance or wallet not found"})
	}
	if dErr != nil {
		log.Printf("[WALLET] Deduct failed: user=%d: %v", req.UserID, dErr)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to deduct from wallet"})
	}

	dualWriteLedgerEntry(req.UserID, "debit", "", req.Amount, newWallet.Balance, req.OrderID,
		"Wallet deduction", "") // DUAL-WRITE LEGADO
	dualWriteWallet(newWallet) // DUAL-WRITE LEGADO

	log.Printf("[WALLET] Deduct OK: user=%d amount=%.2f new_balance=%.2f", req.UserID, req.Amount, newWallet.Balance)

	return c.Status(200).JSON(fiber.Map{
		"user_id":         req.UserID,
		"balance":         newWallet.Balance,
		"amount_deducted": req.Amount,
		"message":         "Amount deducted successfully",
	})
}

// establishmentLedgerTotals soma os créditos (total ganho) e os débitos de
// saque (kind == "withdrawal") do ledger de um estabelecimento via SQL.
func establishmentLedgerTotals(estID int64) (earned, withdrawn float64) {
	var res struct {
		Earned    float64
		Withdrawn float64
	}
	err := models.DB.Model(&models.WalletTxn{}).
		Joins("JOIN wallets ON wallets.id = wallet_transactions.wallet_id").
		Where("wallets.user_id = ? AND wallets.user_type = ?", estID, "establishment").
		Select(`COALESCE(SUM(CASE WHEN wallet_transactions.type = 'credit' THEN wallet_transactions.amount ELSE 0 END), 0) AS earned,
		       COALESCE(SUM(CASE WHEN wallet_transactions.kind = 'withdrawal' THEN wallet_transactions.amount ELSE 0 END), 0) AS withdrawn`).
		Scan(&res).Error
	if err != nil {
		log.Printf("[WALLET] Falha ao somar ledger do estabelecimento %d: %v", estID, err)
		return 0, 0
	}
	return res.Earned, res.Withdrawn
}

// GetEstablishmentWallet retorna o saldo da carteira do estabelecimento
// autenticado (papel restaurante no WebRestaurant).
// GET /wallet/establishment/balance
//
// pending/blocked são 0: o modelo atual só tem saldo disponível;
// total_earned/total_withdrawn vêm do ledger.
func GetEstablishmentWallet(c *fiber.Ctx) error {
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Establishment ID not found in token"})
	}

	wallet, wErr := ensureWalletSeeded(models.DB, estID, "establishment")
	available := 0.0
	var lastUpdated time.Time
	if wErr == nil {
		available = wallet.Balance
		lastUpdated = wallet.LastUpdated
	}

	earned, withdrawn := establishmentLedgerTotals(estID)

	return c.JSON(fiber.Map{
		"user_id":         estID,
		"available":       available,
		"pending":         0.0,
		"blocked":         0.0,
		"total_earned":    earned,
		"total_withdrawn": withdrawn,
		"last_updated":    lastUpdated,
	})
}

// GetEstablishmentTransactions lista o extrato do estabelecimento autenticado,
// paginado por cursor (ID numérico em string — antes era ObjectID hex; o
// frontend trata cursor como opaco, então a troca é transparente).
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

	q := models.DB.Model(&models.WalletTxn{}).
		Joins("JOIN wallets ON wallets.id = wallet_transactions.wallet_id").
		Where("wallets.user_id = ? AND wallets.user_type = ?", estID, "establishment")

	if cursorStr := c.Query("cursor"); cursorStr != "" {
		cursorID, cErr := strconv.ParseInt(cursorStr, 10, 64)
		if cErr != nil || cursorID <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid cursor"})
		}
		q = q.Where("wallet_transactions.id < ?", cursorID)
	}

	var rows []models.WalletTxn
	if err := q.Order("wallet_transactions.id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list transactions"})
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	data := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		item := map[string]interface{}{
			"id":          strconv.FormatInt(r.ID, 10),
			"description": r.Description,
			"amount":      r.Amount,
			"balance":     r.BalanceAfter,
			"created_at":  r.CreatedAt.Format(time.RFC3339),
			"payment_ref": r.ReferenceID,
		}
		switch {
		case r.Kind == "withdrawal":
			item["type"] = "WITHDRAWAL"
		case r.Type == "credit":
			item["type"] = "CREDIT"
		default:
			item["type"] = "DEBIT"
		}
		data = append(data, item)
	}

	nextCursor := ""
	if hasMore && len(rows) > 0 {
		nextCursor = strconv.FormatInt(rows[len(rows)-1].ID, 10)
	}

	return c.JSON(fiber.Map{
		"data":        data,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}

// EstablishmentWithdraw processa um saque da carteira do estabelecimento
// autenticado (papel restaurante). Débito atômico com guarda de saldo e
// lançamento no ledger (type=debit, kind=withdrawal, destination=chave PIX).
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

	description := fmt.Sprintf("Saque via %s para %s", req.Method, req.Destination)
	newWallet, dErr := models.AdjustWalletBalance(
		models.DB, estID, "establishment",
		"debit", "withdrawal", req.Amount,
		"", // saques não têm referência de pagamento
		description,
		req.Destination,
	)
	if dErr == models.ErrInsufficientBalance {
		return c.Status(400).JSON(fiber.Map{"error": "Saldo insuficiente para este saque"})
	}
	if dErr != nil {
		log.Printf("[WALLET] Saque falhou: establishment=%d: %v", estID, dErr)
		return c.Status(500).JSON(fiber.Map{"error": "Falha ao processar saque"})
	}

	dualWriteLedgerEntry(estID, "debit", "withdrawal", req.Amount, newWallet.Balance, "",
		description, req.Destination) // DUAL-WRITE LEGADO
	dualWriteWallet(newWallet) // DUAL-WRITE LEGADO

	log.Printf("[WALLET] Saque OK: establishment=%d amount=%.2f method=%s novo_saldo=%.2f", estID, req.Amount, req.Method, newWallet.Balance)

	return c.JSON(fiber.Map{
		"message": "Saque solicitado com sucesso",
		"balance": newWallet.Balance,
	})
}
