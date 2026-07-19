package routes

import (
	"github.com/carloshomar/vercardapio/payment_api/app/handlers"
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	app.Post("/payments/pix/generate", handlers.GeneratePIX)
	app.Post("/payments/card/tokenize", handlers.TokenizeCard)
	app.Post("/payments/card/charge", handlers.ChargeCard)
	app.Post("/payments/process", handlers.ProcessPayment)
	app.Post("/payments/split", handlers.ProcessSplit)
	app.Post("/payments/webhook", handlers.HandlePaymentWebhook)
	app.Post("/payments/mercadopago/webhook", handlers.MercadoPagoWebhook)
	app.Get("/wallet/balance/:user_id", handlers.GetBalance)
	app.Post("/wallet/topup", handlers.TopUp)
	app.Post("/wallet/deduct", handlers.DeductFromWallet)
}
