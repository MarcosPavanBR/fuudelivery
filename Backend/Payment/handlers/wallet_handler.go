package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/services"
)

type WalletHandler struct {
	Service *services.WalletService
}

func NewWalletHandler() *WalletHandler {
	return &WalletHandler{
		Service: services.NewWalletService(),
	}
}

func (wh *WalletHandler) GetBalance(c *fiber.Ctx) error {
	userID := c.Params("user_id")
	if userID == "" {
		userID = c.Locals("user_id").(string)
	}

	balance, err := wh.Service.GetBalance(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get balance"})
	}

	return c.JSON(fiber.Map{"balance": balance})
}

func (wh *WalletHandler) GetTransactions(c *fiber.Ctx) error {
	userID := c.Params("user_id")
	if userID == "" {
		userID = c.Locals("user_id").(string)
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	transactions, err := wh.Service.GetTransactions(userID, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get transactions"})
	}

	return c.JSON(transactions)
}

func (wh *WalletHandler) Credit(c *fiber.Ctx) error {
	userID := c.Params("user_id")

	var body struct {
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
		ReferenceID string  `json:"reference_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := wh.Service.CreditWallet(userID, body.Amount, body.Description, body.ReferenceID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to credit wallet"})
	}

	return c.JSON(fiber.Map{"message": "Wallet credited"})
}

func (wh *WalletHandler) Debit(c *fiber.Ctx) error {
	userID := c.Params("user_id")

	var body struct {
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
		ReferenceID string  `json:"reference_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := wh.Service.DebitWallet(userID, body.Amount, body.Description, body.ReferenceID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to debit wallet"})
	}

	return c.JSON(fiber.Map{"message": "Wallet debited"})
}

func (wh *WalletHandler) GetOrCreate(c *fiber.Ctx) error {
	userID := c.Params("user_id")
	userType := c.Query("type", "establishment")

	wallet, err := wh.Service.GetOrCreateWallet(userID, userType)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get or create wallet"})
	}

	return c.JSON(wallet)
}
