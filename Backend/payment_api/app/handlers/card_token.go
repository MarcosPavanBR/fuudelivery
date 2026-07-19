package handlers

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func randomDigits(length int) string {
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		result[i] = byte('0') + byte(n.Int64())
	}
	return string(result)
}

func randomString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[n.Int64()]
	}
	return string(result)
}

type CardTokenizeRequest struct {
	CardNumber     string `json:"card_number"`
	CardHolderName string `json:"card_holder_name"`
	ExpMonth       int    `json:"exp_month"`
	ExpYear        int    `json:"exp_year"`
	CardCVV        string `json:"card_cvv"`
}

type CardTokenizeResponse struct {
	Token      string `json:"token"`
	LastDigits string `json:"last_digits"`
	Brand      string `json:"brand"`
}

func detectCardBrand(cardNumber string) string {
	if len(cardNumber) == 0 {
		return "unknown"
	}
	switch cardNumber[0] {
	case '4':
		return "visa"
	case '5':
		return "mastercard"
	case '3':
		return "amex"
	case '6':
		return "discover"
	default:
		return "unknown"
	}
}

func TokenizeCard(c *fiber.Ctx) error {
	var req CardTokenizeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if len(req.CardNumber) < 13 || len(req.CardNumber) > 19 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid card number"})
	}

	lastDigits := req.CardNumber[len(req.CardNumber)-4:]
	token := fmt.Sprintf("ct_%s_%s", randomDigits(16), randomString(8))

	return c.Status(200).JSON(fiber.Map{
		"card_token":  token,
		"last_digits": lastDigits,
		"brand":       detectCardBrand(req.CardNumber),
		"exp_month":   req.ExpMonth,
		"exp_year":    req.ExpYear,
		"message":     "Card tokenized successfully",
	})
}

func ValidateCard(c *fiber.Ctx) error {
	var req struct {
		CardToken string `json:"card_token"`
		Amount    float64 `json:"amount"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if !strings.HasPrefix(req.CardToken, "ct_") {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid card token"})
	}

	return c.Status(200).JSON(fiber.Map{
		"valid":   true,
		"message": "Card token is valid",
	})
}
