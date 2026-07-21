package routes

import (
	"github.com/carloshomar/vercardapio/auth_api/app/middlewares"
	"github.com/carloshomar/vercardapio/payment_api/app/handlers"
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	// Webhooks - NO auth (called by payment gateways)
	app.Post("/payments/webhook", handlers.HandlePaymentWebhook)

	// Protected routes - require JWT
	auth := func(c *fiber.Ctx) error {
		_, err := middlewares.ValidateJWT(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		return c.Next()
	}

	app.Post("/payments/pix/generate", auth, handlers.GeneratePIX)
	app.Post("/payments/card/tokenize", auth, handlers.TokenizeCard)
	app.Post("/payments/card/charge", auth, handlers.ChargeCard)
	app.Post("/payments/process", auth, handlers.ProcessPayment)
	app.Post("/payments/split", auth, handlers.ProcessSplit)
	app.Get("/wallet/balance/:user_id", auth, handlers.GetBalance)
	app.Post("/wallet/topup", auth, handlers.TopUp)
	app.Post("/wallet/deduct", auth, handlers.DeductFromWallet)
}
